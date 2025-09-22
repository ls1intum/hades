package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/nats-io/nats.go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type PodLogReader struct {
	k8sClient *kubernetes.Clientset
	namespace string
	jobID     string
	nc        *nats.Conn
}

func (pl PodLogReader) waitForAllContainers(ctx context.Context) error {
	cli := pl.k8sClient.CoreV1().Pods(pl.namespace)

	// Get pod spec to know what containers to expect
	p, err := cli.Get(ctx, pl.jobID, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Wait for init containers
	for _, initContainer := range p.Spec.InitContainers {
		if err := pl.waitForContainer(ctx, initContainer.Name, true); err != nil {
			return err
		}
	}

	// Wait for regular container (dummy)
	for _, container := range p.Spec.Containers {
		if err := pl.waitForContainer(ctx, container.Name, false); err != nil {
			return err
		}
	}

	return nil
}

func (pl PodLogReader) waitForContainer(ctx context.Context, containerName string, isInitContainer bool) error {
	var exitCode int32

	cli := pl.k8sClient.CoreV1().Pods(pl.namespace)

	err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		p, err := cli.Get(ctx, pl.jobID, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		var containerStatuses []corev1.ContainerStatus
		if isInitContainer {
			containerStatuses = p.Status.InitContainerStatuses
		} else {
			containerStatuses = p.Status.ContainerStatuses
		}

		// Find our specific container
		for _, status := range containerStatuses {
			if status.Name == containerName {
				if status.State.Terminated != nil {
					exitCode = status.State.Terminated.ExitCode
					return true, nil
				}
				break
			}
		}

		return false, nil
	})

	if err != nil {
		return fmt.Errorf("timeout waiting for container %s: %v", containerName, err)
	}

	slog.Debug("Container completed", slog.String("job_id", pl.jobID), slog.String("container_name", containerName),
		slog.Int("exit_code", int(exitCode)))

	// Process logs upon container completion
	if err := pl.processContainerLogs(ctx, containerName); err != nil {
		slog.Error("Warning: failed to process logs for %s: %v\n", containerName, err)
	}

	if exitCode != 0 {
		return fmt.Errorf("container %s failed with exit code %d", containerName, exitCode)
	}

	return nil
}

func (pl PodLogReader) processContainerLogs(ctx context.Context, containerName string) error {
	stdout, stderr, err := pl.getContainerLogs(ctx, containerName)
	if err != nil {
		return fmt.Errorf("getting container logs: %w", err)
	}

	buildJobLog, err := log.ParseContainerLogs(stdout, stderr, containerName)
	if err != nil {
		return fmt.Errorf("parsing container logs: %w", err)
	}
	buildJobLog.JobID = pl.jobID
	publisher := log.NewNATSPublisher(pl.nc)
	return publisher.PublishLogs(buildJobLog)
}

func (pl PodLogReader) getContainerLogs(ctx context.Context, containerName string) (*bytes.Buffer, *bytes.Buffer, error) {
	// get logs of <container name>
	podLogOpts := corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		Timestamps: true,
	}

	req := pl.k8sClient.CoreV1().Pods(pl.namespace).GetLogs(pl.jobID, &podLogOpts)
	logReader, err := req.Stream(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("getting container logs: %w", err)
	}
	defer logReader.Close()

	// K8s logs are already combined (stdout/stderr mixed)
	// So we need to parse them differently
	allLogs := new(bytes.Buffer)
	if _, err := io.Copy(allLogs, logReader); err != nil {
		return nil, nil, fmt.Errorf("reading logs: %w", err)
	}

	// For K8s, you might need to separate stdout/stderr differently
	// or just treat all logs as stdout
	return allLogs, new(bytes.Buffer), nil
}

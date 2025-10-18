package k8s

import (
	"bytes"
	"context"
	"errors"
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
	if pl.k8sClient == nil {
		return fmt.Errorf("nil k8sClient in PodLogReader: operator mode must also initialize typed clientset")
	}

	cli := pl.k8sClient.CoreV1().Pods(pl.namespace)

	// Resolve actual pod name using jobID
	podName, err := pl.resolvePodName(ctx)
	if err != nil {
		return err
	}

	// Get pod spec to know what containers to expect
	p, err := cli.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Wait for init containers
	for _, initContainer := range p.Spec.InitContainers {
		if err := pl.waitForContainer(ctx, podName, initContainer.Name, true); err != nil {
			return err
		}
	}

	// Wait for regular container (dummy)
	for _, container := range p.Spec.Containers {
		if err := pl.waitForContainer(ctx, podName, container.Name, false); err != nil {
			return err
		}
	}

	return nil
}

func (pl PodLogReader) waitForContainer(ctx context.Context, podName string, containerName string, isInitContainer bool) error {
	var exitCode int32

	cli := pl.k8sClient.CoreV1().Pods(pl.namespace)

	err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
		p, err := cli.Get(ctx, podName, metav1.GetOptions{})
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
		if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
			slog.Debug("wait canceled (expected on job completion)",
				slog.String("job_id", pl.jobID),
				slog.String("pod_name", podName),
				slog.String("container_name", containerName),
			)
			return nil
		}
		return fmt.Errorf("timeout waiting for container %s: %v", containerName, err)
	}

	slog.Debug("Container completed",
		slog.String("job_id", pl.jobID),
		slog.String("pod_name", podName),
		slog.String("container_name", containerName),
		slog.Int("exit_code", int(exitCode)),
	)

	// Process logs upon container completion
	if err := pl.processContainerLogs(ctx, podName, containerName); err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
			slog.Debug("processContainerLogs canceled (expected)", "pod_name", podName, "container_name", containerName)
		} else {
			slog.Error("Warning: failed to process logs", "pod_name", podName, "container_name", containerName, "error", err)
		}
	}

	if exitCode != 0 {
		return fmt.Errorf("container %s failed with exit code %d", containerName, exitCode)
	}

	return nil
}

func (pl PodLogReader) processContainerLogs(ctx context.Context, podName string, containerName string) error {
	stdout, stderr, err := pl.getContainerLogs(ctx, podName, containerName)
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

func (pl PodLogReader) getContainerLogs(ctx context.Context, podName string, containerName string) (*bytes.Buffer, *bytes.Buffer, error) {
	// get logs of <container name>
	podLogOpts := corev1.PodLogOptions{
		Container:  containerName,
		Follow:     false,
		Timestamps: true,
	}

	req := pl.k8sClient.CoreV1().Pods(pl.namespace).GetLogs(podName, &podLogOpts)
	logReader, err := req.Stream(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
			slog.Debug("Log stream canceled (expected on job completion)", "pod", podName, "container", containerName)
			return new(bytes.Buffer), new(bytes.Buffer), nil
		}
		return nil, nil, fmt.Errorf("getting container logs: %w", err)
	}
	defer logReader.Close()

	// K8s logs are already combined (stdout/stderr mixed)
	// So we need to parse them differently
	allLogs := new(bytes.Buffer)
	if _, err := io.Copy(allLogs, logReader); err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
			slog.Debug("Log copy canceled (expected on job completion)", "pod", podName, "container", containerName)
			return allLogs, new(bytes.Buffer), nil
		}
		return nil, nil, fmt.Errorf("reading logs: %w", err)
	}

	// For K8s, you might need to separate stdout/stderr differently
	// or just treat all logs as stdout
	return allLogs, new(bytes.Buffer), nil
}

func (pl PodLogReader) resolvePodName(ctx context.Context) (string, error) {
	cli := pl.k8sClient.CoreV1().Pods(pl.namespace)

	// if we have a pod name, we're done'
	if p, err := cli.Get(ctx, pl.jobID, metav1.GetOptions{}); err == nil {
		return p.Name, nil
	}

	// otherwise, we need to find the pod name by looking at the job name
	jobName := fmt.Sprintf("buildjob-%s", pl.jobID)
	if lst, err := cli.List(ctx, metav1.ListOptions{
		LabelSelector: "job-name=" + jobName,
	}); err == nil {
		if len(lst.Items) == 1 {
			return lst.Items[0].Name, nil
		}
		if len(lst.Items) > 1 {
			return "", fmt.Errorf("found multiple pods with label job-name=%s; expected exactly 1", jobName)
		}
	}

	return "", fmt.Errorf("pod for jobID %s not found yet", pl.jobID)
}

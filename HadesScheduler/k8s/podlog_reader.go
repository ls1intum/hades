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
	"github.com/ls1intum/hades/shared/buildlogs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	JobNameLabel = "job-name=%s"
	pollInterval = 3 * time.Second
	pollTimeout  = 10 * time.Minute
)

type PodLogReader struct {
	K8sClient *kubernetes.Clientset
	Namespace string
	JobID     string
	Publisher log.Publisher
}

// Waits for all containers in the pod to complete and processes their logs
// Currently used in Scheduler mode only
func (pl PodLogReader) waitForAllContainers(ctx context.Context) error {
	if pl.K8sClient == nil {
		return fmt.Errorf("nil k8sClient in PodLogReader: operator mode must also initialize typed clientset")
	}

	cli := pl.K8sClient.CoreV1().Pods(pl.Namespace)

	// Resolve pod name using jobID
	podName, err := pl.ResolvePodName(ctx)
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
			if errors.Is(err, context.Canceled) {
				slog.Debug("Init container processing cancelled", "job_id", pl.JobID)
				return ctx.Err()
			}
			return err
		}
	}

	// Wait for dummy container
	for _, container := range p.Spec.Containers {
		if err := pl.waitForContainer(ctx, podName, container.Name, false); err != nil {
			if errors.Is(err, context.Canceled) {
				slog.Debug("Container processing cancelled", "job_id", pl.JobID)
				return ctx.Err()
			}
			return err
		}
	}

	return nil
}

// Helper function for waitForAllContainers
// Waits for a specific container to complete by polling and then processes its logs
// Currently used in Scheduler mode only
func (pl PodLogReader) waitForContainer(ctx context.Context, podName string, containerName string, isInitContainer bool) error {
	var exitCode int32

	cli := pl.K8sClient.CoreV1().Pods(pl.Namespace)

	err := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
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
				slog.String("job_id", pl.JobID),
				slog.String("pod_name", podName),
				slog.String("container_name", containerName),
			)
			return context.Canceled
		}
		return fmt.Errorf("timeout waiting for container %s: %v", containerName, err)
	}

	slog.Debug("Container completed",
		slog.String("job_id", pl.JobID),
		slog.String("pod_name", podName),
		slog.String("container_name", containerName),
		slog.Int("exit_code", int(exitCode)),
	)

	// Process logs upon container completion
	if err := pl.ProcessContainerLogs(ctx, podName, containerName); err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
			slog.Debug("processContainerLogs canceled (expected)", "pod_name", podName, "container_name", containerName)
			return context.Canceled
		} else {
			slog.Error("Warning: failed to process logs", "pod_name", podName, "container_name", containerName, "error", err)
		}
	}

	if exitCode != 0 {
		return fmt.Errorf("container %s failed with exit code %d", containerName, exitCode)
	}

	return nil
}

// Processes logs for a specific container in the pod
// Fetches logs, parses them, and publishes to NATS
// Used in both Scheduler and Operator modes
func (pl PodLogReader) ProcessContainerLogs(ctx context.Context, podName string, containerName string) error {
	slog.Info("Getting container logs", "pod", podName, "container", containerName)
	stdout, stderr, err := pl.getContainerLogs(ctx, podName, containerName)
	if err != nil {
		return fmt.Errorf("getting container logs: %w", err)
	}

	slog.Info("Parsing container logs", "pod", podName, "container", containerName)
	parser := log.NewStdLogParser(stdout, stderr)
	buildJobLog, err := parser.ParseContainerLogs(containerName, pl.JobID)
	if err != nil {
		return fmt.Errorf("parsing container logs: %w", err)
	}

	buildJobLog.JobID = pl.JobID
	slog.Info("Publishing logs", "pod", podName, "container", containerName)
	return pl.Publisher.PublishLog(ctx, buildJobLog)
}

// Helper function for ProcessContainerLogs
// Fetches logs for a specific container in the pod using container name
// Used in both Scheduler and Operator modes
func (pl PodLogReader) getContainerLogs(ctx context.Context, podName string, containerName string) (*bytes.Buffer, *bytes.Buffer, error) {
	// get logs of <container name>
	podLogOpts := corev1.PodLogOptions{
		Container:  containerName,
		Follow:     false,
		Timestamps: true,
	}

	req := pl.K8sClient.CoreV1().Pods(pl.Namespace).GetLogs(podName, &podLogOpts)
	logReader, err := req.Stream(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() == context.Canceled {
			slog.Debug("Log stream canceled (expected on job completion)", "pod", podName, "container", containerName)
			return new(bytes.Buffer), new(bytes.Buffer), context.Canceled
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
	// Currently all logs are treated as stdout for simplicity
	return allLogs, new(bytes.Buffer), nil
}

// Resolves the pod name for the given JobID
// Used in both Scheduler and Operator modes
func (pl PodLogReader) ResolvePodName(ctx context.Context) (string, error) {
	cli := pl.K8sClient.CoreV1().Pods(pl.Namespace)

	// if we have a pod name, return
	if p, err := cli.Get(ctx, pl.JobID, metav1.GetOptions{}); err == nil {
		return p.Name, nil
	}

	// else: find the pod name by looking at the job name
	jobName := fmt.Sprintf(buildlogs.JobNamePrefix, pl.JobID)
	if lst, err := cli.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf(JobNameLabel, jobName),
	}); err == nil {
		if len(lst.Items) == 1 {
			return lst.Items[0].Name, nil
		}
		if len(lst.Items) > 1 {
			return "", fmt.Errorf("found multiple pods with label job-name=%s; expected exactly 1", jobName)
		}
	}

	return "", fmt.Errorf("pod for jobID %s not found yet", pl.JobID)
}

package k8s

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/buildlogs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const JobNameLabel = "job-name=%s"

type PodLogReader struct {
	K8sClient *kubernetes.Clientset
	Namespace string
	JobID     string
	Publisher log.NATSPublisher
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
	return pl.Publisher.PublishJobLog(ctx, buildJobLog)
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

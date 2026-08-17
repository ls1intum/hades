package controller

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	buildv1 "github.com/ls1intum/hades/HadesScheduler/HadesOperator/api/v1"
	"github.com/ls1intum/hades/hadesScheduler/k8s"
	"github.com/ls1intum/hades/shared/buildlogs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const FinalizerContainerName = "buildjob-finalizer"

// helper: build a configured PodLogReader for the given namespace/job.
func (r *BuildJobReconciler) podLogReader(namespace, jobID string) k8s.PodLogReader {
	return k8s.PodLogReader{
		K8sClient: r.K8sClient,
		Namespace: namespace,
		JobID:     jobID,
		Publisher: *r.Publisher,
	}
}

// initializeContainerStatuses creates Pending status entries for all expected containers of the BuildJob
func (r *BuildJobReconciler) initializeContainerStatuses(ctx context.Context, bj *buildv1.BuildJob) error {
	slog.Info("Initializing container statuses for BuildJob", "buildJob", bj.Name)
	statuses := make([]buildv1.ContainerStatus, 0, len(bj.Spec.Steps)+1)

	// Initialize status for each step (init containers)
	for _, step := range bj.Spec.Steps {
		statuses = append(statuses, buildv1.ContainerStatus{
			Name:          fmt.Sprintf(BuildStepPrefix, step.ID),
			StepID:        step.ID,
			State:         buildv1.ContainerStatePending,
			LogsPublished: false,
		})
	}

	// Initialize status for finalizer container
	statuses = append(statuses, buildv1.ContainerStatus{
		Name:          FinalizerContainerName,
		StepID:        0, // 0 indicates it's not a step
		State:         buildv1.ContainerStatePending,
		LogsPublished: false,
	})

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var fresh buildv1.BuildJob
		if err := r.Get(ctx, client.ObjectKeyFromObject(bj), &fresh); err != nil {
			return err
		}
		fresh.Status.ContainerStatuses = statuses
		currentStep := int32(1)
		fresh.Status.CurrentStep = &currentStep
		return r.Status().Update(ctx, &fresh)
	})
}

// updateContainerStatuses resolves PodName using the BuildJob and updates each
// BuildJob's container statuses accordingly. It returns draining=true when at
// least one container has terminated but its live log stream has not finished
// publishing, signalling the caller to requeue so the job is not finalized (and
// its pod deleted) before the tail logs are captured.
func (r *BuildJobReconciler) updateContainerStatuses(ctx context.Context, bj *buildv1.BuildJob) (draining bool, err error) {
	slog.Info("Updating container statuses for BuildJob", "buildJob", bj.Name)

	pl := r.podLogReader(bj.Namespace, bj.Name)

	podName, err := pl.ResolvePodName(ctx)
	if err != nil {
		slog.Error("Failed to resolve pod name", "error", err)
		return false, err
	}

	p, err := r.K8sClient.CoreV1().Pods(bj.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var fresh buildv1.BuildJob
		if err := r.Get(ctx, client.ObjectKeyFromObject(bj), &fresh); err != nil {
			return err
		}

		// Build map of current statuses from container status slice for easy lookup
		statusMap := make(map[string]buildv1.ContainerStatus)
		for _, cs := range fresh.Status.ContainerStatuses {
			statusMap[cs.Name] = cs
		}

		// Update init container statuses (build steps)
		for _, initCS := range p.Status.InitContainerStatuses {
			statusMap[initCS.Name] = r.updateContainerStateMap(ctx, bj, p, statusMap, initCS)
		}

		// Update regular container statuses (finalizer)
		for _, containerCS := range p.Status.ContainerStatuses {
			statusMap[containerCS.Name] = r.updateContainerStateMap(ctx, bj, p, statusMap, containerCS)
		}

		// Determine current step
		currentStep := r.determineCurrentStep(p, len(bj.Spec.Steps))

		// Convert map back to slice
		newStatuses := make([]buildv1.ContainerStatus, 0, len(statusMap))
		for _, cs := range statusMap {
			newStatuses = append(newStatuses, cs)
		}

		// A terminated container whose logs are not yet published still has a log
		// stream draining; keep requeueing until it finishes.
		draining = false
		for _, cs := range newStatuses {
			if (cs.State == buildv1.ContainerStateSucceeded || cs.State == buildv1.ContainerStateFailed) && !cs.LogsPublished {
				draining = true
				break
			}
		}

		// Update BuildJob status

		fresh.Status.ContainerStatuses = newStatuses
		fresh.Status.CurrentStep = &currentStep
		fresh.Status.PodName = p.Name
		return r.Status().Update(ctx, &fresh)
	})
	return draining, err
}

// updateContainerStateMap updates the ContainerStatus for a specific container
// in the status map and drives its live log stream.
//
// A single follow-stream goroutine is started once the container is running (or,
// after an operator restart or for a very short container, once it is
// terminated) and follows the container to EOF, publishing logs incrementally.
// A terminated container's logs are marked published only after its stream has
// fully drained, so the caller can hold off finalizing the job until then.
func (r *BuildJobReconciler) updateContainerStateMap(ctx context.Context, bj *buildv1.BuildJob, p *corev1.Pod, statusMap map[string]buildv1.ContainerStatus, containerState corev1.ContainerStatus) buildv1.ContainerStatus {
	cs := statusMap[containerState.Name]

	newCS := r.buildContainerStatus(containerState.Name, cs.StepID, containerState.State)
	cs.State = newCS.State

	terminated := cs.State == buildv1.ContainerStateSucceeded || cs.State == buildv1.ContainerStateFailed
	key := logStreamKey(bj.Namespace, bj.Name, cs.Name)

	// Start (idempotently) the live log stream once the container is running or
	// terminated and its logs have not yet been fully published. The stream
	// captures LogsStreamedUntil at start time so a re-established stream (after
	// an operator restart) skips already-published lines.
	if (cs.State == buildv1.ContainerStateRunning || terminated) && !cs.LogsPublished {
		podName := p.Name
		sinceTime := cs.LogsStreamedUntil
		containerName := cs.Name
		started := r.logStreams().ensureStarted(key, bj.Name, func(streamCtx context.Context, progress func(time.Time)) error {
			pl := r.podLogReader(bj.Namespace, bj.Name)
			return pl.StreamContainerLogs(streamCtx, podName, containerName, sinceTime, progress)
		})
		// Register the container's slot synchronously, in container order (this
		// function is called per container in pod order), so a step producing no
		// output keeps its position even though streams run concurrently. Done
		// once per container, guarded by started.
		if started {
			if err := r.Publisher.PublishJobLog(ctx, buildlogs.Log{JobID: bj.Name, ContainerID: containerName}); err != nil {
				slog.Warn("Failed to register container log slot", "container", containerName, "error", err)
			}
		}
	}

	// Persist streaming progress, throttled, so a restart re-follows from near the
	// last published line rather than from the container's start.
	if stream := r.logStreams().get(key); stream != nil {
		cs.LogsStreamedUntil = throttledStreamOffset(cs.LogsStreamedUntil, stream.progress())
	}

	// Once terminated, mark logs published only after the stream has drained.
	if terminated && !cs.LogsPublished {
		if stream := r.logStreams().get(key); stream != nil {
			if finished, streamErr := stream.result(); finished {
				if streamErr == nil {
					cs.LogsPublished = true
				} else {
					slog.Error("Container log stream failed; will retry", "container", cs.Name, "error", streamErr)
				}
				// Drop the entry either way: on success it is done; on failure the
				// next reconcile re-establishes it (from the persisted offset).
				r.logStreams().remove(key)
			}
		}
	}

	return cs
}

// throttledStreamOffset returns the LogsStreamedUntil value to persist. It keeps
// the previously persisted value unless streaming progress has advanced by at
// least logStreamPersistThrottle, so the field changes rarely and does not cause
// a reconcile/etcd write storm.
func throttledStreamOffset(persisted *metav1.Time, progress time.Time) *metav1.Time {
	if progress.IsZero() {
		return persisted
	}
	if persisted == nil || progress.Sub(persisted.Time) >= logStreamPersistThrottle {
		t := metav1.NewTime(progress)
		return &t
	}
	return persisted
}

// buildContainerStatus maps K8s container state to BuildJob ContainerStatus
func (r *BuildJobReconciler) buildContainerStatus(name string, stepID int32, state corev1.ContainerState) buildv1.ContainerStatus {
	cs := buildv1.ContainerStatus{
		Name:   name,
		StepID: stepID,
	}

	if state.Waiting != nil {
		cs.State = buildv1.ContainerStatePending

	} else if state.Running != nil {
		cs.State = buildv1.ContainerStateRunning

	} else if state.Terminated != nil {
		if state.Terminated.ExitCode == 0 {
			cs.State = buildv1.ContainerStateSucceeded
		} else {
			cs.State = buildv1.ContainerStateFailed
		}
	} else {
		cs.State = buildv1.ContainerStateUnknown
	}

	return cs
}

// determineCurrentStep figures out which step is currently executing
func (r *BuildJobReconciler) determineCurrentStep(pod *corev1.Pod, totalSteps int) int32 {
	// Check init containers (steps)
	for i, initCS := range pod.Status.InitContainerStatuses {
		if initCS.State.Running != nil {
			return int32(i + 1) // Step IDs are 1-based
		}
		if initCS.State.Waiting != nil {
			return int32(i + 1)
		}
	}

	// If all init containers finished, check regular container (finalizer)
	for _, containerCS := range pod.Status.ContainerStatuses {
		if containerCS.State.Running != nil || containerCS.State.Waiting != nil {
			return int32(totalSteps + 1) // Finalizer is the last "step"
		}
	}

	// All done or not started yet
	if len(pod.Status.InitContainerStatuses) == 0 {
		return 1 // Not started
	}
	return int32(totalSteps + 1) // All done
}

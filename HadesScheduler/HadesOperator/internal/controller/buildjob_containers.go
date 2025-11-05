package controller

import (
	"context"
	"log/slog"

	buildv1 "github.com/ls1intum/hades/HadesScheduler/HadesOperator/api/v1"
	"github.com/ls1intum/hades/hadesScheduler/k8s"
	corev1 "k8s.io/api/core/v1"
)

func (r *BuildJobReconciler) updateContainerStateMap(ctx context.Context, bj *buildv1.BuildJob, p *corev1.Pod, statusMap map[string]buildv1.ContainerStatus, containerState corev1.ContainerStatus) buildv1.ContainerStatus {
	cs := statusMap[containerState.Name]

	newCS := r.buildContainerStatus(containerState.Name, cs.StepID, containerState.State)
	cs.State = newCS.State

	if cs.State == buildv1.ContainerStateSucceeded || cs.State == buildv1.ContainerStateFailed {
		if !cs.LogsPublished {
			slog.Info("Container terminated, reading logs", "container", cs.Name)

			pl := k8s.PodLogReader{
				K8sClient: r.K8sClient,
				Namespace: p.Namespace,
				JobID:     bj.Name,
				Nc:        r.NatsConnection,
			}

			if err := pl.ProcessContainerLogs(ctx, p.Name, cs.Name); err != nil {
				slog.Error("Failed to read/publish logs", "container", cs.Name, "error", err)
			} else {
				cs.LogsPublished = true
			}
		}
	}

	return cs
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

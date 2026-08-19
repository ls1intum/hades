package controller

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func waitingStatus(name, image, reason string) corev1.ContainerStatus {
	return corev1.ContainerStatus{
		Name:  name,
		Image: image,
		State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{Reason: reason},
		},
	}
}

func runningStatus(name string) corev1.ContainerStatus {
	return corev1.ContainerStatus{
		Name:  name,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	}
}

func TestPodStuckReason(t *testing.T) {
	tests := []struct {
		name        string
		pod         corev1.Pod
		wantStuck   bool
		wantSubstrs []string
	}{
		{
			name: "init container ImagePullBackOff is stuck",
			pod: corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("step-1", "ghcr.io/x/y:1.0.0", "ImagePullBackOff"),
				},
			}},
			wantStuck:   true,
			wantSubstrs: []string{"ImagePullBackOff", "step 1", "ghcr.io/x/y:1.0.0"},
		},
		{
			name: "app container CrashLoopBackOff is stuck",
			pod: corev1.Pod{Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("finalizer", "busybox:latest", "CrashLoopBackOff"),
				},
			}},
			wantStuck:   true,
			wantSubstrs: []string{"CrashLoopBackOff", "busybox:latest"},
		},
		{
			name: "transient ErrImagePull is not stuck",
			pod: corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("step-1", "ghcr.io/x/y:1.0.0", "ErrImagePull"),
				},
			}},
			wantStuck: false,
		},
		{
			name: "transient ContainerCreating is not stuck",
			pod: corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("step-1", "ghcr.io/x/y:1.0.0", "ContainerCreating"),
				},
			}},
			wantStuck: false,
		},
		{
			name: "running container is not stuck",
			pod: corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{runningStatus("step-1")},
			}},
			wantStuck: false,
		},
		{
			name: "init container reason wins over app container PodInitializing",
			pod: corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("step-2", "ghcr.io/x/y:2.0.0", "ImagePullBackOff"),
				},
				ContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("finalizer", "hades-finalizer:latest", "PodInitializing"),
				},
			}},
			wantStuck:   true,
			wantSubstrs: []string{"step 1", "ghcr.io/x/y:2.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, stuck := podStuckReason(&tt.pod)
			if stuck != tt.wantStuck {
				t.Fatalf("stuck = %v, want %v (reason=%q)", stuck, tt.wantStuck, reason)
			}
			if !tt.wantStuck {
				if reason != "" {
					t.Fatalf("expected empty reason, got %q", reason)
				}
				return
			}
			for _, sub := range tt.wantSubstrs {
				if !strings.Contains(reason, sub) {
					t.Fatalf("reason %q does not contain %q", reason, sub)
				}
			}
		})
	}
}

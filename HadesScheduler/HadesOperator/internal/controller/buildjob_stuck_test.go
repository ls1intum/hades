package controller

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func waitingStatus(name, reason, message string) corev1.ContainerStatus {
	return corev1.ContainerStatus{
		Name: name,
		State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{Reason: reason, Message: message},
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
		name       string
		pod        corev1.Pod
		wantStuck  bool
		wantSubstr string
	}{
		{
			name: "init container ImagePullBackOff is stuck",
			pod: corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("step-1", "ImagePullBackOff", `Back-off pulling image "ghcr.io/x/y:1.0.0"`),
				},
			}},
			wantStuck:  true,
			wantSubstr: "ImagePullBackOff",
		},
		{
			name: "app container CrashLoopBackOff is stuck",
			pod: corev1.Pod{Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("finalizer", "CrashLoopBackOff", "back-off restarting failed container"),
				},
			}},
			wantStuck:  true,
			wantSubstr: "CrashLoopBackOff",
		},
		{
			name: "transient ErrImagePull is not stuck",
			pod: corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("step-1", "ErrImagePull", "pulling"),
				},
			}},
			wantStuck: false,
		},
		{
			name: "transient ContainerCreating is not stuck",
			pod: corev1.Pod{Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("step-1", "ContainerCreating", ""),
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
					waitingStatus("step-2", "ImagePullBackOff", "back-off"),
				},
				ContainerStatuses: []corev1.ContainerStatus{
					waitingStatus("finalizer", "PodInitializing", ""),
				},
			}},
			wantStuck:  true,
			wantSubstr: "step 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, stuck := podStuckReason(&tt.pod)
			if stuck != tt.wantStuck {
				t.Fatalf("stuck = %v, want %v (reason=%q)", stuck, tt.wantStuck, reason)
			}
			if tt.wantStuck && !strings.Contains(reason, tt.wantSubstr) {
				t.Fatalf("reason %q does not contain %q", reason, tt.wantSubstr)
			}
			if !tt.wantStuck && reason != "" {
				t.Fatalf("expected empty reason, got %q", reason)
			}
		})
	}
}

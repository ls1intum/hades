package buildlogs

import "testing"

func TestJobStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status JobStatus
		want   string
	}{
		{"Queued", StatusQueued, "Queued"},
		{"Running", StatusRunning, "Running"},
		{"Succeeded", StatusSucceeded, "Succeeded"},
		{"Failed", StatusFailed, "Failed"},
		{"Stopped", StatusStopped, "Stopped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("JobStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobStatus_Subject(t *testing.T) {
	tests := []struct {
		name   string
		status JobStatus
		want   string
	}{
		{"Queued", StatusQueued, "hades.jobstatus.Queued"},
		{"Running", StatusRunning, "hades.jobstatus.Running"},
		{"Succeeded", StatusSucceeded, "hades.jobstatus.Succeeded"},
		{"Failed", StatusFailed, "hades.jobstatus.Failed"},
		{"Stopped", StatusStopped, "hades.jobstatus.Stopped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.Subject(); got != tt.want {
				t.Errorf("JobStatus.Subject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status JobStatus
		want   bool
	}{
		{"queued is valid", StatusQueued, true},
		{"running is valid", StatusRunning, true},
		{"succeeded is valid", StatusSucceeded, true},
		{"failed is valid", StatusFailed, true},
		{"stopped is valid", StatusStopped, true},
		{"invalid status", JobStatus("invalid"), false},
		{"empty status", JobStatus(""), false},
		{"random string", JobStatus("random"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("JobStatus.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

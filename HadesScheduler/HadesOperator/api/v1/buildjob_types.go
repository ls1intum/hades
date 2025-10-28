/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuildJobSpec struct {
	// Job name, useful for debugging
	Name string `json:"name"`

	// Additional metadata
	Metadata map[string]string `json:"metadata,omitempty"`

	// Build steps to be executed sequentially
	// +kubebuilder:validation:MinItems=1
	Steps []BuildStep `json:"steps"`

	// (Optional) Timeout for the whole job; if exceeded, the job is marked as failed
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty"`

	// (Optional) Number of times to retry the job on failure
	// +kubebuilder:validation:Minimum=0
	MaxRetries *int32 `json:"maxRetries,omitempty"`
}

type BuildStep struct {
	// Step ID (must be unique and start from 1)
	// +kubebuilder:validation:Minimum=1
	ID int32 `json:"id"`

	// Step name
	Name string `json:"name,omitempty"`

	// Execution image (required)
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Script or command to run inside the container
	Script string `json:"script,omitempty"`

	// Additional metadata like environment variables, credentials, etc.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Resource limits (if empty, defaults will be used from LimitRange/Quota)
	CPULimit    *resource.Quantity `json:"cpuLimit,omitempty"`
	MemoryLimit *resource.Quantity `json:"memoryLimit,omitempty"`
}

type BuildJobStatus struct {
	// Pending | Running | Succeeded | Failed
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`

	// Name of the Pod/Job created by the Operator, helpful for troubleshooting
	PodName string `json:"podName,omitempty"`

	StartTime      *metav1.Time `json:"startTime,omitempty"`
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// ID of the currently executing step
	CurrentStep *int32 `json:"currentStep,omitempty"`

	// Number of retry attempts so far
	RetryCount int32 `json:"retryCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// BuildJob is the Schema for the buildjobs API.
type BuildJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildJobSpec   `json:"spec,omitempty"`
	Status BuildJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BuildJobList contains a list of BuildJob.
type BuildJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BuildJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BuildJob{}, &BuildJobList{})
}

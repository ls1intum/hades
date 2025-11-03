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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	buildv1 "github.com/ls1intum/hades/HadesScheduler/HadesOperator/api/v1"
	"github.com/ls1intum/hades/hadesScheduler/k8s"
	"github.com/nats-io/nats.go"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const conflictRequeueDelay = 200 * time.Millisecond

// BuildJobReconciler reconciles a BuildJob object
type BuildJobReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	K8sClient        *kubernetes.Clientset
	NatsConnection   *nats.Conn
	DeleteOnComplete bool
}

// +kubebuilder:rbac:groups=build.hades.tum.de,resources=buildjobs;buildjobs/status;buildjobs/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods;events;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures the cluster state matches the desired state of a BuildJob.
// It creates/owns a batch Job, updates BuildJob status, and cleans up on completion.
func (r *BuildJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// ----------------------------- 0. Retrieve the BuildJob instance -----------------------------
	var bj buildv1.BuildJob
	if err := r.Get(ctx, req.NamespacedName, &bj); err != nil {
		if apierrors.IsNotFound(err) {
			// Object has been deleted; ignore it
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// ----------------------------- 0b. If being deleted, skip (avoid recreating children) --------
	if !bj.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// ----------------------------- 1. Exit if already processed ----------------------------------
	// Only process objects that are not marked as "finalized" (i.e., not deleted)
	if bj.Status.Phase == "Succeeded" || bj.Status.Phase == "Failed" {
		return ctrl.Result{}, nil
	}

	// ----------------------------- 2. Check if the Job already exists ----------------------------
	jobName := fmt.Sprintf("buildjob-%s", bj.Name)
	var existingJob batchv1.Job
	err := r.Get(ctx, client.ObjectKey{Namespace: bj.Namespace, Name: jobName}, &existingJob)
	if err == nil {
		// Job already exists check the status of the containers
		if err := r.updateContainerStatuses(ctx, &bj); err != nil {
			slog.Error("Failed to update container statuses", "error", err)
		}

		// Job already exists, check the status of the job
		done, succeeded, msg := jobFinished(&existingJob)
		if done {
			// Job is done, publish "completed" event
			r.publishBuildJobEvent(ctx, bj.Name, bj.Namespace, "completed", map[string]any{
				"succeeded": succeeded,
				"message":   msg,
			})
			slog.Info("BuildJob completed event published", "subject", fmt.Sprintf("buildjob.events.%s", bj.Name))

			if err := r.setStatusCompleted(ctx, req.NamespacedName, succeeded, msg); err != nil {
				if apierrors.IsConflict(err) {
					return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
				}
				return ctrl.Result{}, err
			}

			// If DeleteOnComplete is false, don't delete the CR, used for debugging
			if !r.DeleteOnComplete {
				return ctrl.Result{}, nil
			}

			// Delete the CR once the job is done
			policy := metav1.DeletePropagationForeground
			return ctrl.Result{}, r.Delete(ctx, &bj, &client.DeleteOptions{PropagationPolicy: &policy})
		}

		// if bj.Status.Phase != "Running" {
		// Job starting to run - publish event
		// r.publishBuildJobEvent(ctx, bj.Name, bj.Namespace, "pod_running", map[string]any{})
		// slog.Info("BuildJob running event published", "subject", fmt.Sprintf("buildjob.events.%s", bj.Name))

		// Build is not done, set the status to be "running"
		if err := r.setStatusRunning(ctx, req.NamespacedName, jobName); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// ----------------------------- 3. First-time creation -----------------------------

	// 3.1 Create Kubernetes Job (initContainers = bj.Spec.Steps)
	k8sJob := buildK8sJob(&bj, jobName, r.DeleteOnComplete)

	// 3.2 Set OwnerReference
	if err := controllerutil.SetControllerReference(&bj, k8sJob, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// 3.3 Create Job in Kubernetes as Pod
	slog.Info("Creating Job for BuildJob", "job", k8sJob.Name)

	if err := r.Create(ctx, k8sJob); err != nil {
		if apierrors.IsAlreadyExists(err) {
			slog.Debug("Job already exists, ", "job", k8sJob.Name)
		} else {
			slog.Error("cannot create Job", "error", err)
			return ctrl.Result{}, err
		}
	}

	// Initialize container statuses
	if err := r.initializeContainerStatuses(ctx, &bj); err != nil {
		slog.Error("Failed to initialize container statuses", "error", err)
	}

	// 3.4 Update CR Status → Running
	if err := r.setStatusRunning(ctx, req.NamespacedName, jobName); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
		}
		return ctrl.Result{}, err
	}

	// r.publishBuildJobEvent(ctx, bj.Name, bj.Namespace, "pod_running", map[string]any{})
	// slog.Info("BuildJob running event published", "subject", fmt.Sprintf("buildjob.events.%s", bj.Name))

	// Do not requeue; later Job status changes will re-trigger reconciliation
	return ctrl.Result{}, nil
}

// initializeContainerStatuses creates Pending status entries for all expected containers
func (r *BuildJobReconciler) initializeContainerStatuses(ctx context.Context, bj *buildv1.BuildJob) error {
	slog.Info("Initializing container statuses for BuildJob", "buildJob", bj.Name)
	statuses := make([]buildv1.ContainerStatus, 0, len(bj.Spec.Steps)+1)

	// Initialize status for each step (init containers)
	for _, step := range bj.Spec.Steps {
		statuses = append(statuses, buildv1.ContainerStatus{
			Name:          fmt.Sprintf("step-%d", step.ID),
			StepID:        step.ID,
			State:         buildv1.ContainerStatePending,
			LogsPublished: false,
		})
	}

	// Initialize status for finalizer container
	statuses = append(statuses, buildv1.ContainerStatus{
		Name:          "buildjob-finalizer",
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

func (r *BuildJobReconciler) updateContainerStatuses(ctx context.Context, bj *buildv1.BuildJob) error {
	slog.Info("Updating container statuses for BuildJob", "buildJob", bj.Name)

	pl := k8s.PodLogReader{
		K8sClient: r.K8sClient,
		Namespace: bj.Namespace,
		JobID:     bj.Name,
		Nc:        r.NatsConnection,
	}

	podName, err := pl.ResolvePodName(ctx)
	if err != nil {
		slog.Error("Failed to resolve pod name", "error", err)
		return err
	}

	p, err := r.K8sClient.CoreV1().Pods(bj.Namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Build map of current statuses from container status slice for easy lookup
	statusMap := make(map[string]buildv1.ContainerStatus)
	for _, cs := range bj.Status.ContainerStatuses {
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

	// Update BuildJob status
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var fresh buildv1.BuildJob
		if err := r.Get(ctx, client.ObjectKeyFromObject(bj), &fresh); err != nil {
			return err
		}
		fresh.Status.ContainerStatuses = newStatuses
		fresh.Status.CurrentStep = &currentStep
		fresh.Status.PodName = p.Name
		return r.Status().Update(ctx, &fresh)
	})
}

// setStatusRunning sets BuildJob.Status to "Running", records StartTime and PodName.
// Uses optimistic concurrency (RetryOnConflict).
func (r *BuildJobReconciler) setStatusRunning(ctx context.Context, nn types.NamespacedName, jobName string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &buildv1.BuildJob{}
		if err := r.Get(ctx, nn, latest); err != nil {
			return client.IgnoreNotFound(err)
		}
		if latest.DeletionTimestamp != nil {
			return nil
		}
		if latest.Status.Phase == "Running" {
			return nil
		}

		base := latest.DeepCopy()
		now := metav1.Now()
		latest.Status.Phase = "Running"
		latest.Status.StartTime = &now
		latest.Status.PodName = jobName

		if err := r.Status().Patch(ctx, latest, client.MergeFrom(base)); err != nil {
			slog.Error("Failed to patch BuildJob status to Running", "error", err)
			return err
		}
		return nil
	})
}

// setStatusCompleted marks BuildJob.Status as Succeeded/Failed and sets CompletionTime/message.
// Uses optimistic concurrency (RetryOnConflict).
func (r *BuildJobReconciler) setStatusCompleted(ctx context.Context, nn types.NamespacedName, succeeded bool, msg string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &buildv1.BuildJob{}
		if err := r.Get(ctx, nn, latest); err != nil {
			return client.IgnoreNotFound(err)
		}
		if latest.DeletionTimestamp != nil {
			return nil
		}

		if latest.Status.Phase == "Succeeded" || latest.Status.Phase == "Failed" {
			return nil
		}

		base := latest.DeepCopy()
		now := metav1.Now()
		if succeeded {
			latest.Status.Phase = "Succeeded"
		} else {
			latest.Status.Phase = "Failed"
		}
		latest.Status.Message = msg
		latest.Status.CompletionTime = &now

		return r.Status().Patch(ctx, latest, client.MergeFrom(base))
	})
}

// buildK8sJob creates a one-time Job with InitContainers that execute each Step in order
func buildK8sJob(bj *buildv1.BuildJob, jobName string, deleteOnComplete bool) *batchv1.Job {
	sharedVolName := "shared"
	sharedMount := corev1.VolumeMount{Name: sharedVolName, MountPath: "/shared"}

	var initCtrs []corev1.Container
	for _, s := range bj.Spec.Steps {
		c := corev1.Container{
			Name:         fmt.Sprintf("step-%d", s.ID),
			Image:        s.Image,
			Env:          envFromMeta(s.Metadata), // Convert metadata to environment variables
			VolumeMounts: []corev1.VolumeMount{sharedMount},
		}

		if strings.TrimSpace(s.Script) != "" {
			c.Command = []string{"/bin/sh", "-c"}
			c.Args = []string{s.Script}
		}

		// Set resource limits if specified
		if s.CPULimit != nil || s.MemoryLimit != nil {
			c.Resources.Limits = corev1.ResourceList{}
			if s.CPULimit != nil {
				c.Resources.Limits[corev1.ResourceCPU] = *s.CPULimit
			}
			if s.MemoryLimit != nil {
				c.Resources.Limits[corev1.ResourceMemory] = *s.MemoryLimit
			}
		}

		initCtrs = append(initCtrs, c)
	}

	// Add a dummy container that runs after all init containers have finished
	dummy := corev1.Container{
		Name:         "buildjob-finalizer",
		Image:        "busybox",
		Command:      []string{"sh", "-c", "echo build finished"},
		VolumeMounts: []corev1.VolumeMount{sharedMount},
	}

	podSpec := corev1.PodSpec{
		InitContainers: initCtrs,
		Containers:     []corev1.Container{dummy},
		Volumes: []corev1.Volume{{
			Name: sharedVolName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}},
		RestartPolicy: corev1.RestartPolicyNever,
	}

	ttl := (*int32)(nil)
	backoff := int32(0)

	if !deleteOnComplete {
		t := int32(3600)
		ttl = &t
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: bj.Namespace,
			Labels:    map[string]string{"job-id": bj.Name},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: ttl,
			BackoffLimit:            &backoff,
			Template: corev1.PodTemplateSpec{
				Spec: podSpec,
			},
		},
	}
}

// envFromMeta converts a string map to []corev1.EnvVar for container env injection.
func envFromMeta(m map[string]string) []corev1.EnvVar {
	var envs []corev1.EnvVar
	for k, v := range m {
		envs = append(envs, corev1.EnvVar{Name: k, Value: v})
	}
	return envs
}

// SetupWithManager registers the controller, watching BuildJobs and owned Jobs.
func (r *BuildJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&buildv1.BuildJob{}).
		Owns(&batchv1.Job{}).
		Named("buildjob").
		Complete(r)
}

// jobFinished checks Job conditions and reports terminal state and reason.
func jobFinished(k8sJob *batchv1.Job) (done bool, succeeded bool, reason string) {
	for _, c := range k8sJob.Status.Conditions {
		switch c.Type {
		case batchv1.JobComplete:
			if c.Status == corev1.ConditionTrue {
				return true, true, c.Message
			}
		case batchv1.JobFailed:
			if c.Status == corev1.ConditionTrue {
				return true, false, c.Message
			}
		}
	}
	return false, false, ""
}

func (r *BuildJobReconciler) publishBuildJobEvent(ctx context.Context, buildJobName, namespace, status string, data map[string]any) {
	if r.NatsConnection == nil {
		slog.Error("Cannot publish BuildJob Event: nil NATS Connection", "buildJobName", buildJobName)
		return
	}

	event := map[string]any{
		"buildJob":  buildJobName,
		"namespace": namespace,
		"status":    status,
		"timestamp": time.Now(),
	}

	// Merge additional data
	maps.Copy(event, data)

	eventBytes, _ := json.Marshal(event)
	subject := fmt.Sprintf("buildjob.events.%s", buildJobName)

	if err := r.NatsConnection.Publish(subject, eventBytes); err != nil {
		// Log but don't fail the reconciliation
		slog.Error("Failed to publish BuildJob event", "subject", subject, "error", err)
	}
}

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
	"fmt"
	"log/slog"
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
	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/buildlogs"
	"github.com/nats-io/nats.go"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const conflictRequeueDelay = 200 * time.Millisecond
const BuildStepPrefix = "step-%d"

// BuildJobReconciler reconciles a BuildJob object
type BuildJobReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	K8sClient        *kubernetes.Clientset
	NatsConnection   *nats.Conn
	DeleteOnComplete bool
	Publisher        log.NATSPublisher
}

// +kubebuilder:rbac:groups=build.hades.tum.de,resources=buildjobs;buildjobs/status;buildjobs/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods;events;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures the cluster state matches the desired state of a BuildJob.
// It creates/owns a batch Job, updates BuildJob status and each ContainerStatus of the BuildJob, and cleans up on completion.
// Triggers NATS log publishing on ContainerStatus changes, and status publishing on BuildJob status updates.
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
	if bj.Status.Phase == string(buildlogs.StatusSucceeded) || bj.Status.Phase == string(buildlogs.StatusFailed) {
		return ctrl.Result{}, nil
	}

	// ----------------------------- 2. Check if the Job already exists ----------------------------
	jobName := fmt.Sprintf(buildlogs.JobNamePrefix, bj.Name)
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

			if err := r.setStatusCompleted(ctx, req.NamespacedName, bj.Name, succeeded, msg); err != nil {
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

		// Build is not done, set the status to be "running"
		if bj.Status.Phase != string(buildlogs.StatusRunning) {
			if err := r.setStatusRunning(ctx, req.NamespacedName, jobName, bj.Name); err != nil {
				if apierrors.IsConflict(err) {
					return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
				}
				return ctrl.Result{}, err
			}
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

	// 3.3.1 Initialize container statuses
	if err := r.initializeContainerStatuses(ctx, &bj); err != nil {
		slog.Error("Failed to initialize container statuses", "error", err)
	}

	// 3.4 Update CR Status → Running
	if err := r.setStatusRunning(ctx, req.NamespacedName, jobName, bj.Name); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
		}
		return ctrl.Result{}, err
	}

	// Do not requeue; later Job status changes will re-trigger reconciliation
	return ctrl.Result{}, nil
}

// setStatusRunning sets BuildJob.Status to "Running", records StartTime and PodName.
// publishes "running" jobstatus to NATS.
// Uses optimistic concurrency (RetryOnConflict).
func (r *BuildJobReconciler) setStatusRunning(ctx context.Context, nn types.NamespacedName, jobName string, jobID string) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &buildv1.BuildJob{}
		if err := r.Get(ctx, nn, latest); err != nil {
			return client.IgnoreNotFound(err)
		}
		if latest.DeletionTimestamp != nil {
			return nil
		}
		if latest.Status.Phase == string(buildlogs.StatusRunning) {
			return nil
		}

		base := latest.DeepCopy()
		now := metav1.Now()
		latest.Status.Phase = string(buildlogs.StatusRunning)
		latest.Status.StartTime = &now
		latest.Status.PodName = jobName

		if err := r.Status().Patch(ctx, latest, client.MergeFrom(base)); err != nil {
			slog.Error("Failed to patch BuildJob status to Running", "error", err)
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	if err := r.Publisher.PublishJobStatus(ctx, buildlogs.StatusRunning, jobID); err != nil {
		slog.Error("Failed to publish job running status", "job_id", jobID, "error", err)
		return err
	}
	slog.Info("Published job running status", "job_id", jobID)
	return nil
}

// setStatusCompleted marks BuildJob.Status as Succeeded/Failed and sets CompletionTime/message.
// publishes "success" or "failed" jobstatus to NATS.
// Uses optimistic concurrency (RetryOnConflict).
func (r *BuildJobReconciler) setStatusCompleted(ctx context.Context, nn types.NamespacedName, jobID string, succeeded bool, msg string) error {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &buildv1.BuildJob{}
		if err := r.Get(ctx, nn, latest); err != nil {
			return client.IgnoreNotFound(err)
		}
		if latest.DeletionTimestamp != nil {
			return nil
		}

		if latest.Status.Phase == string(buildlogs.StatusSucceeded) || latest.Status.Phase == string(buildlogs.StatusFailed) {
			return nil
		}

		base := latest.DeepCopy()
		now := metav1.Now()
		if succeeded {
			latest.Status.Phase = string(buildlogs.StatusSucceeded)
		} else {
			latest.Status.Phase = string(buildlogs.StatusFailed)
		}
		latest.Status.Message = msg
		latest.Status.CompletionTime = &now

		return r.Status().Patch(ctx, latest, client.MergeFrom(base))
	}); err != nil {
		return err
	}

	if succeeded {
		if err := r.Publisher.PublishJobStatus(ctx, buildlogs.StatusSucceeded, jobID); err != nil {
			slog.Error("Failed to publish job succeeded status", "job_id", jobID, "error", err)
			return err
		}
		slog.Info("Published job succeeded status", "job_id", jobID)
		return nil
	} else {
		if err := r.Publisher.PublishJobStatus(ctx, buildlogs.StatusFailed, jobID); err != nil {
			slog.Error("Failed to publish job failed status", "job_id", jobID, "error", err)
			return err
		}
		slog.Info("Published job failed status", "job_id", jobID)
		return nil
	}
}

// buildK8sJob creates a one-time Job with InitContainers that execute each Step in order
func buildK8sJob(bj *buildv1.BuildJob, jobName string, deleteOnComplete bool) *batchv1.Job {
	sharedVolName := "shared"
	sharedMount := corev1.VolumeMount{Name: sharedVolName, MountPath: "/shared"}

	var initCtrs []corev1.Container
	for _, s := range bj.Spec.Steps {
		c := corev1.Container{
			Name:         fmt.Sprintf(BuildStepPrefix, s.ID),
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
		Name:         FinalizerContainerName,
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

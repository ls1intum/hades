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
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	buildv1 "github.com/ls1intum/hades/HadesScheduler/HadesOperator/api/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const conflictRequeueDelay = 200 * time.Millisecond

const (
	LabelManagedBy = "hades.tum.de/managed-by"
	LabelBuildJob  = "hades.tum.de/buildjob"
)

// BuildJobReconciler reconciles a BuildJob object
type BuildJobReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	DeleteOnComplete bool
	MaxParallelism   int
}

// +kubebuilder:rbac:groups=build.hades.tum.de,resources=buildjobs;buildjobs/status;buildjobs/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods;events;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures the cluster state matches the desired state of a BuildJob.
// It creates/owns a batch Job, updates BuildJob status, and cleans up on completion.
func (r *BuildJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

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
		// Job already exists, check the status of the job
		done, succeeded, msg := jobFinished(&existingJob)
		if done {
			if err := r.setStatusCompleted(ctx, req.NamespacedName, succeeded, msg); err != nil {
				if apierrors.IsConflict(err) {
					return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
				}
				return ctrl.Result{}, err
			}

			// If DeleteOnComplete is false, don't delete the CR, used for debugging
			if !r.DeleteOnComplete {
				// If there is available concurrency, admit one suspended job (optional)
				_ = r.admitOneSuspendedJob(ctx, bj.Namespace)
				return ctrl.Result{}, nil
			}

			// Delete the CR once the job is done
			policy := metav1.DeletePropagationForeground
			if err := r.Delete(ctx, &bj, &client.DeleteOptions{PropagationPolicy: &policy}); err != nil {
				return ctrl.Result{}, err
			}

			// Once a old job is deleted, admit one suspended job
			_ = r.admitOneSuspendedJob(ctx, bj.Namespace)
			return ctrl.Result{}, nil
		}

		// Build is still running, set the status to be "running"
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
	//3.0 check concurrency limit and current active jobs
	active, err := r.countActiveJobs(ctx, bj.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	shouldSuspend := active >= r.MaxParallelism

	// 3.1 Create Kubernetes Job (initContainers = bj.Spec.Steps)
	k8sJob := buildK8sJob(&bj, jobName, r.DeleteOnComplete, shouldSuspend)

	// 3.2 Set OwnerReference
	if err := controllerutil.SetControllerReference(&bj, k8sJob, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// 3.3 Create Job in Kubernetes as Pod
	log.Info("Creating Job for BuildJob", "job", k8sJob.Name, "suspend", shouldSuspend)

	if err := r.Create(ctx, k8sJob); err != nil {
		if apierrors.IsAlreadyExists(err) {
			slog.Debug("Job already exists, ", "job", k8sJob.Name)
		} else {
			log.Error(err, "cannot create Job")
			return ctrl.Result{}, err
		}
	}

	// 3.4 Update CR Status → Running
	// 3.4.1 If over the concurrency limit, set to Pending and requeue
	if shouldSuspend {
		if err := r.setStatusPending(ctx, req.NamespacedName,
			fmt.Sprintf("Waiting for capacity: active=%d, limit=%d", active, r.MaxParallelism),
		); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	//3.4.2 Otherwise, set to Running
	if err := r.setStatusRunning(ctx, req.NamespacedName, jobName); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: conflictRequeueDelay}, nil
		}
		return ctrl.Result{}, err
	}

	// Do not requeue; later Job status changes will re-trigger reconciliation
	return ctrl.Result{}, nil
}

// setStatusRunning sets BuildJob.Status to "Running", records StartTime and PodName.
// Uses optimistic concurrency (RetryOnConflict).
func (r *BuildJobReconciler) setStatusRunning(ctx context.Context, nn types.NamespacedName, jobName string) error {
	logger := log.FromContext(ctx)

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
			logger.Error(err, "failed to patch BuildJob status to Running")
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

// setStatusPending marks BuildJob.Status as Pending with a message.
// Uses optimistic concurrency (RetryOnConflict).
func (r *BuildJobReconciler) setStatusPending(ctx context.Context, nn types.NamespacedName, msg string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &buildv1.BuildJob{}
		if err := r.Get(ctx, nn, latest); err != nil {
			return client.IgnoreNotFound(err)
		}
		if latest.DeletionTimestamp != nil {
			return nil
		}
		if latest.Status.Phase == "Pending" {
			return nil
		}
		base := latest.DeepCopy()
		now := metav1.Now()
		latest.Status.Phase = "Pending"
		latest.Status.Message = msg
		if latest.Status.StartTime == nil {
			latest.Status.StartTime = &now
		}
		return r.Status().Patch(ctx, latest, client.MergeFrom(base))
	})
}

// buildK8sJob creates a one-time Job with InitContainers that execute each Step in order
func buildK8sJob(bj *buildv1.BuildJob, jobName string, deleteOnComplete bool, suspend bool) *batchv1.Job {
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
			Labels: map[string]string{
				LabelManagedBy: "hades-operator",
				LabelBuildJob:  bj.Name},
		},
		Spec: batchv1.JobSpec{
			Suspend:                 &suspend,
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

func jobDone(k8sJob *batchv1.Job) (done bool, succeeded bool, reason string) {
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

// countActiveJobs counts the number of non-completed, non-suspended Jobs in the given namespace.
func (r *BuildJobReconciler) countActiveJobs(ctx context.Context, namespace string) (int, error) {
	var jl batchv1.JobList
	if err := r.List(ctx, &jl,
		client.InNamespace(namespace),
		client.MatchingLabels{"hades.tum.de/managed-by": "hades-operator"},
	); err != nil {
		return 0, err
	}
	active := 0
	for _, j := range jl.Items {
		done, _, _ := jobDone(&j)
		if done {
			continue
		}
		// Unless the job is completed or suspended, count it as active
		if j.Spec.Suspend == nil || !*j.Spec.Suspend {
			active++
		}
	}
	return active, nil
}

// admitOneSuspendedJob unsuspends the oldest suspended Job in the given namespace, if any.
func (r *BuildJobReconciler) admitOneSuspendedJob(ctx context.Context, namespace string) error {
	var jl batchv1.JobList
	if err := r.List(ctx, &jl,
		client.InNamespace(namespace),
		client.MatchingLabels{"hades.tum.de/managed-by": "hades-operator"},
	); err != nil {
		return err
	}

	var oldest *batchv1.Job
	for i := range jl.Items {
		j := &jl.Items[i]
		done, _, _ := jobDone(j)
		if done {
			continue
		}
		if j.Spec.Suspend != nil && *j.Spec.Suspend {
			if oldest == nil || j.CreationTimestamp.Before(&oldest.CreationTimestamp) {
				oldest = j
			}
		}
	}
	if oldest == nil {
		return nil
	}

	base := oldest.DeepCopy()
	falseVal := false
	oldest.Spec.Suspend = &falseVal
	return r.Patch(ctx, oldest, client.MergeFrom(base))
}

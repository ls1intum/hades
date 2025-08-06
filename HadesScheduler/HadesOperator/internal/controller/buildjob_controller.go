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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"

	buildv1 "github.com/ls1intum/hades/api/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildJobReconciler reconciles a BuildJob object
type BuildJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=build.hades.tum.de,resources=buildjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=build.hades.tum.de,resources=buildjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=build.hades.tum.de,resources=buildjobs/finalizers,verbs=update

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
			// Build finished
			bj.Status.Phase = map[bool]string{true: "Succeeded", false: "Failed"}[succeeded]
			bj.Status.Message = msg
			now := metav1.Now()
			bj.Status.CompletionTime = &now
			if err := r.Status().Update(ctx, &bj); err != nil {
				log.Error(err, "update final status")
				return ctrl.Result{}, err
			}
			// Delete the CRD after the job is either successful or failed
			return ctrl.Result{}, r.Delete(ctx, &bj)
		}

		// Build is still running, set the status to be "running"
		r.setStatusRunning(ctx, &bj, jobName)
		return ctrl.Result{}, nil
	}
	if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// ----------------------------- 3. First-time creation -----------------------------

	// 3.1 Create Kubernetes Job (initContainers = bj.Spec.Steps)
	k8sJob := buildK8sJob(&bj, jobName)

	// 3.2 Set OwnerReference
	if err := controllerutil.SetControllerReference(&bj, k8sJob, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// 3.3 Create Job in Kubernetes as Pod
	log.Info("Creating Job for BuildJob", "job", k8sJob.Name)
	if err := r.Create(ctx, k8sJob); err != nil {
		log.Error(err, "cannot create Job")
		return ctrl.Result{}, err
	}

	// 3.4 Update CR Status → Running
	r.setStatusRunning(ctx, &bj, jobName)

	// Do not requeue; later Job status changes will re-trigger reconciliation
	return ctrl.Result{}, nil
}

func (r *BuildJobReconciler) setStatusRunning(ctx context.Context, bj *buildv1.BuildJob, jobName string) {
	if bj.Status.Phase == "Running" {
		return
	}
	now := metav1.Now()
	bj.Status.Phase = "Running"
	bj.Status.StartTime = &now
	bj.Status.PodName = jobName

	if err := r.Status().Update(ctx, bj); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "failed to update BuildJob status to Running")
	}
}

// buildK8sJob creates a one-time Job with InitContainers that execute each Step in order
func buildK8sJob(bj *buildv1.BuildJob, jobName string) *batchv1.Job {
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

	ttl := int32(300)
	backoff := int32(0)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: bj.Namespace,
			Labels:    map[string]string{"job-id": bj.Name},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			BackoffLimit:            &backoff,
			Template: corev1.PodTemplateSpec{
				Spec: podSpec,
			},
		},
	}
}

func envFromMeta(m map[string]string) []corev1.EnvVar {
	var envs []corev1.EnvVar
	for k, v := range m {
		envs = append(envs, corev1.EnvVar{Name: k, Value: v})
	}
	return envs
}

// SetupWithManager sets up the controller with the Manager.
func (r *BuildJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&buildv1.BuildJob{}).
		Owns(&batchv1.Job{}).
		Named("buildjob").
		Complete(r)
}

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

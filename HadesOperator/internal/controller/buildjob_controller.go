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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	// Only process objects entering for the first time (Phase is empty and Job not yet created)
	if bj.Status.Phase != "" {
		return ctrl.Result{}, nil
	}

	// ----------------------------- 2. Check if the Job already exists ----------------------------
	jobName := fmt.Sprintf("buildjob-%s", bj.Name)
	var existingJob batchv1.Job
	err := r.Get(ctx, client.ObjectKey{Namespace: bj.Namespace, Name: jobName}, &existingJob)
	if err == nil {
		// Job already exists (maybe previously created, but CR status hasn't been updated yet); set Phase to Running
		r.setStatusRunning(ctx, &bj, jobName)
		return ctrl.Result{}, nil
	}
	if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// ----------------------------- 3. First-time creation -----------------------------
	// 3.1 Create ConfigMap (e.g., for build scripts)
	cm := buildScriptConfigMap(&bj, jobName)
	if err := r.Create(ctx, cm); err != nil && !apierrors.IsAlreadyExists(err) {
		log.Error(err, "cannot create ConfigMap")
		return ctrl.Result{}, err
	}

	// 3.2 Create Kubernetes Job (initContainers = bj.Spec.Steps)
	k8sJob := buildK8sJob(&bj, cm.Name, jobName)

	// 3.3 Set OwnerReference (automatically delete ConfigMap when Job is deleted)
	if err := controllerutil.SetControllerReference(&bj, k8sJob, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// 3.4 Create Job in Kubernetes as Pod
	log.Info("Creating Job for BuildJob", "job", k8sJob.Name)
	if err := r.Create(ctx, k8sJob); err != nil {
		log.Error(err, "cannot create Job")
		return ctrl.Result{}, err
	}

	// 3.5 Update CR Status → Running
	r.setStatusRunning(ctx, &bj, jobName)

	return ctrl.Result{}, nil // Do not requeue; later Job status changes will re-trigger reconciliation
}

func (r *BuildJobReconciler) setStatusRunning(ctx context.Context, bj *buildv1.BuildJob, jobName string) {
	bj.Status.Phase = "Running"
	now := metav1.NewTime(time.Now())
	bj.Status.StartTime = &now
	bj.Status.PodName = jobName

	if err := r.Status().Update(ctx, bj); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "failed to update BuildJob status to Running")
	}
}

// buildScriptConfigMap writes each step's script into the ConfigMap as key=<step-id>.sh
func buildScriptConfigMap(bj *buildv1.BuildJob, jobName string) *corev1.ConfigMap {
	data := make(map[string]string)
	for _, s := range bj.Spec.Steps {
		data[fmt.Sprintf("%d.sh", s.ID)] = s.Script
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName + "-scripts",
			Namespace: bj.Namespace,
		},
		Data: data,
	}
}

// buildK8sJob creates a one-time Job with InitContainers that execute each Step in order
func buildK8sJob(bj *buildv1.BuildJob, cmName, jobName string) *batchv1.Job {
	var initCtrs []corev1.Container
	for _, s := range bj.Spec.Steps {
		initCtrs = append(initCtrs, corev1.Container{
			Name:    fmt.Sprintf("step-%d", s.ID),
			Image:   s.Image,
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{fmt.Sprintf("chmod +x /scripts/%d.sh && /scripts/%d.sh", s.ID, s.ID)},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "scripts",
				MountPath: "/scripts",
			}},
		})
	}

	podSpec := corev1.PodSpec{
		InitContainers: initCtrs,
		Containers: []corev1.Container{{
			Name:    "BuildJobFinalizer",
			Image:   "busybox",
			Command: []string{"sh", "-c", "echo build finished"},
		}},
		Volumes: []corev1.Volume{{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
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

// SetupWithManager sets up the controller with the Manager.
func (r *BuildJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&buildv1.BuildJob{}).
		Named("buildjob").
		Complete(r)
}

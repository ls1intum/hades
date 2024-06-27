package k8s

import (
	"fmt"

	"github.com/ls1intum/hades/shared/payload"
	corev1 "k8s.io/api/core/v1"
)

type K8sStep struct {
	// The Hades step which is executed by this container.
	step payload.Step

	// The name of the volume which contains the shared data between all steps.
	sharedVolumeName string

	// Job Global Metadata
	jobMetadata map[string]string
}

// Returns the k8s container spec for the step. (To be used to build a Pod spec)
func (k8sStep *K8sStep) containerSpec() []corev1.Container {

	containerSpec := []corev1.Container{
		{
			Name:  k8sStep.step.IDstring(),
			Image: k8sStep.step.Image,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      k8sStep.sharedVolumeName,
					MountPath: "/shared",
				},
				{
					Name:      fmt.Sprintf("%s-build-script", k8sStep.step.IDstring()),
					MountPath: "/tmp/script",
				},
			},
			// TODO: ImagePolicy should be configurable in the future
			Env: k8sStep.containerEnvSpec()}}

	// Only use a script if it is provided - Otherwise we assume that the image has a script baked in as an entrypoint
	if k8sStep.step.Script != "" {
		containerSpec[0].Command = []string{"/bin/sh", "-c", "/tmp/script/buildscript.sh"}
	}

	return containerSpec
}

func (k8sSpec *K8sStep) containerEnvSpec() []corev1.EnvVar {
	var envVars []corev1.EnvVar

	// Add step Metadata to the container
	for key, value := range k8sSpec.step.Metadata {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	// Add job Metadata to the container
	for key, value := range k8sSpec.jobMetadata {
		envVars = append(envVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	return envVars

}

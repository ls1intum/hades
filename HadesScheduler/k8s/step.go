package k8s

import (
	"github.com/ls1intum/hades/shared/payload"
	corev1 "k8s.io/api/core/v1"
)

type K8sStep struct {
	// The Hades step which is executed by this container.
	step payload.Step

	// The name of the volume which contains the shared data between all steps.
	sharedVolumeName string

	// The name of the volume which contains the build script data.
	// All buildscripts are stored in a ConfigMap. Each script is then specified as a volume on the pod level.
	// We use the volumes here to mount the buildscript into the container.
	buidScriptVolumeName string
}

const (
	BuidScriptPath = "/tmp/buildscripts.sh"
)

// Returns the k8s container spec for the step. (To be used to build a Pod spec)
func (k8sStep *K8sStep) containerSpec() []corev1.Container {
	containerSpec := []corev1.Container{
		{
			Name:    k8sStep.step.IDstring(),
			Image:   k8sStep.step.Image,
			Command: []string{"/bin/sh", "-c", BuidScriptPath},
			Args:    []string{},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      k8sStep.sharedVolumeName,
					MountPath: "/shared",
				},
				{
					Name:      k8sStep.buidScriptVolumeName,
					MountPath: BuidScriptPath,
					ReadOnly:  true,
				},
			},
		},
	}
	return containerSpec
}

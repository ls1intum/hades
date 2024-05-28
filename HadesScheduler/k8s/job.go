package k8s

import (
	"context"

	"github.com/ls1intum/hades/shared/payload"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

type K8sJob struct {
	job              payload.QueuePayload
	k8sClient        *kubernetes.Clientset
	namespace        string
	sharedVolumeName string
}

// Schedules a Hades Job on the Kubernetes cluster
func (k8sJob K8sJob) execute(ctx context.Context) error {
	log.Infof("Scheduling job %s", k8sJob.job.ID)

	log.Debug("Create buildscript ConfigMap")
	configMap := k8sJob.configMapSpec()
	log.Debugf("ConfigMap spec: %v", configMap)

	log.Debug("Apply buildscirpt ConfigMap to Kubernetes")
	cm, err := k8sJob.k8sClient.CoreV1().ConfigMaps(k8sJob.namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		log.WithError(err).Error("Failed to create ConfigMap")
		return err
	}
	log.Infof("Successfully created buildscirpt ConfigMap %s", cm.Name)

	log.Debug("Assembling PodSpec")
	jobPodSpec := corev1.Pod{

		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sJob.job.ID.String(),
			Namespace: k8sJob.namespace,
		},

		Spec: corev1.PodSpec{
			InitContainers: k8sJob.containerSpec(), // each step is represented by a init container to ensure that the build script is executed in the correct order
			Containers: []corev1.Container{ // dummy container to signal the end of the build - this is a temporary solution as we need to have a container defined in the pod spec
				{
					Name:    "dummy",
					Image:   "busybox",
					Command: []string{"sh", "-c", "echo 'build completed'"},
				},
			},
			Volumes:       k8sJob.volumeSpec(*configMap),
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	log.Debugf("PodSpec: %v", jobPodSpec)

	log.Infof("Apply PodSpec to Kubernetes")
	_, err = k8sJob.k8sClient.CoreV1().Pods(k8sJob.namespace).Create(ctx, &jobPodSpec, metav1.CreateOptions{})
	if err != nil {
		log.WithError(err).Error("Failed to create Pod")
		return err
	}

	return nil
}

// Creates a ConfigMapSpec containing the build script of each step of the job
// Based on this configMap, individual step volumes are created and mounted inside the respectiv container
func (k8sJob K8sJob) configMapSpec() *corev1.ConfigMap {
	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sJob.job.ID.String(),
			Namespace: k8sJob.namespace,
		},
		Data: map[string]string{},
	}

	for _, step := range k8sJob.job.Steps {
		log.Debugf("Creating ConfigMap Data item for step %d", step.ID)
		configMap.Data[step.IDstring()] = step.Script
	}

	return &configMap
}

// Creates the volumeSpec for the Hades Job PodSpec.
// For each step in the job config a volume containing the respective build script is created.
// The respective build script is stored in a ConfigMap and here mounted as a volume.
// Additionally, a shared volume is created to share data between the steps.
// Reference: https://kubernetes.io/docs/concepts/configuration/configmap/#configmaps-and-pods
func (k K8sJob) volumeSpec(cm corev1.ConfigMap) []corev1.Volume {
	volumeSpec := []corev1.Volume{}

	// Create a Volume for each build step containing the build script. The build script is stored in a ConfigMap.
	for _, step := range k.job.Steps {
		volumeSpec = append(volumeSpec, corev1.Volume{
			Name: step.IDstring(), // this is the name of the volume that needs to be mounted in a step container
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cm.Name, // this is the name of the ConfigMap that contains all the build scripts
					},
					Items: []corev1.KeyToPath{
						{
							Key:  step.IDstring(),
							Path: "buildscript.sh",
						},
					},
				},
			},
		})
	}
	volumeSpec = append(volumeSpec, corev1.Volume{
		Name: k.sharedVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	},
	)

	return volumeSpec
}

// Creates the containerSpec for the Hades Job PodSpec.
// Each step in the job config is represented by a container.
// This method combines the containerSpec of each step to a single containerSpec to be used in the PodSpec.
func (k K8sJob) containerSpec() []corev1.Container {
	containerSpec := []corev1.Container{}

	for _, step := range k.job.Steps {
		k8sStep := K8sStep{
			step:                 step,
			sharedVolumeName:     k.sharedVolumeName,
			buidScriptVolumeName: step.IDstring(),
		}
		containerSpec = append(containerSpec, k8sStep.containerSpec()...)
	}

	return containerSpec
}

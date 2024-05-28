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
	job       payload.QueuePayload
	k8sClient *kubernetes.Clientset
	namespace string
}

// Schedules a Hades Job on the Kubernetes cluster
func (k8sJob K8sJob) execute(ctx context.Context) error {

	log.Debugf("Scheduling job %s", k8sJob.job.ID)

	log.Debug("Create ConfigMap for build scripts")
	configMap := k8sJob.configMapSpec()

	log.Debug("Apply ConfigMap to Kubernetes")
	cm, err := k8sJob.k8sClient.CoreV1().ConfigMaps(k8sJob.namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		log.WithError(err).Error("Failed to create ConfigMap")
		return err
	}
	log.Debugf("Created ConfigMap %s", cm.Name)

	log.Debug("Assembling PodSpec")
	log.Error("Not implemented")

	log.Debug("Apply PodSpec to Kubernetes")
	log.Error("Not implemented")

	return nil
}

// Creates a ConfigMapSpec containing the build script of each step of the job
// Based on this configMap, individual step volumes are created and mounted inside the respectiv container
func (k8sJob K8sJob) configMapSpec() *corev1.ConfigMap {
	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: k8sJob.job.ID.String(),
		},
		Data: map[string]string{},
	}

	for _, step := range k8sJob.job.Steps {
		log.Debugf("Creating ConfigMap for step %d", step.ID)
		configMap.Data[step.IDstring()] = step.Script
	}

	return &configMap
}

func (k K8sJob) volumeSpec(cm corev1.ConfigMap) []corev1.Volume {
	volumeSpec := []corev1.Volume{}

	// Create a Volume for each build step containing the build script. The build script is stored in a ConfigMap.
	for _, step := range k.job.Steps {
		volumeSpec = append(volumeSpec, corev1.Volume{
			Name: step.IDstring(), // this is the name of the volume that needs to be mounted in a step container
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cm.Name, // this is the name of the ConfigMap that contains the build script
					},
					Items: []corev1.KeyToPath{
						{
							Key:  step.IDstring(),
							Path: BuidScriptPath,
						},
					},
				},
			},
		})
	}

	return volumeSpec
}

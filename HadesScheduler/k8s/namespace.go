package k8s

import (
	"context"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string) (*corev1.Namespace, error) {
	// Create a namespace in the Kubernetes cluster
	log.Infof("Creating namespace %s", namespace)

	ns, err := clientset.CoreV1().Namespaces().Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: namespace,
			},
		}, v1.CreateOptions{})

	if err != nil {
		log.WithError(err).Error("error creating namespace")
		return nil, err
	}
	log.Infof("Namespace %s created", namespace)

	return ns, nil
}

func deleteNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string) error {
	// Delete a namespace in the Kubernetes cluster
	log.Infof("Deleting namespace %s", namespace)

	err := clientset.CoreV1().Namespaces().Delete(ctx, namespace, v1.DeleteOptions{})
	if err != nil {
		log.WithError(err).Error("error deleting namespace")
		return err
	}
	log.Infof("Namespace %s deleted - It may take some time until namespace is no longer in terminating state", namespace)

	return nil
}

package k8s

import (
	"context"

	"log/slog"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func createNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string) (*corev1.Namespace, error) {
	slog.Debug("Creating namespace", "namespace", namespace)

	ns, err := clientset.CoreV1().Namespaces().Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: namespace,
			},
		}, v1.CreateOptions{})

	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			slog.Debug("Namespace already exists", "namespace", namespace)
			return nil, nil
		}
		slog.With("error", err).Error("error creating namespace")
		return nil, err
	}
	slog.Info("Namespace created", "namespace", namespace)

	return ns, nil
}

func deleteNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string) error {
	slog.Debug("Deleting namespace", "namespace", namespace)

	err := clientset.CoreV1().Namespaces().Delete(ctx, namespace, v1.DeleteOptions{})
	if err != nil {
		slog.With("error", err).Error("error deleting namespace")
		return err
	}
	slog.Info("Namespace deleted - It may take some time until namespace is no longer in terminating state", "namespace", namespace)

	return nil
}

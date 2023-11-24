package kube

import (
	"testing"
)

func TestKubeconfigInitialization(t *testing.T) {
	client := initializeKubeconfig()

	if client == nil {
		t.Error("client is nil")
	}
}

func TestGetPods(t *testing.T) {
	client := initializeKubeconfig()
	namespace := "default"
	getPods(client, namespace)
}

func TestCreateNamespace(t *testing.T) {
	client := initializeKubeconfig()
	createNamespace(client, "test")
}

func TestDeleteNamespace(t *testing.T) {
	client := initializeKubeconfig()
	deleteNamespace(client, "test")
}


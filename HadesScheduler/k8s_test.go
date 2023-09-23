package main

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

func TestCreateJob(t *testing.T) {
	client := initializeKubeconfig()

	namespace := "default"
	jobName := "testJob"
	jobImage := "ubuntu"
	cmd := "sleep 100"

	createJob(client, namespace, &jobName, &jobImage, &cmd)
}

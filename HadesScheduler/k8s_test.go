package main

import (
	"testing"

	"github.com/Mtze/HadesCI/shared/payload"
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

	testBuildJob := payload.BuildJob{
		Name: "Test Build",
		Credentials: struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}{
			Username: "testuser",
			Password: "testpassword",
		},
		BuildConfig: struct {
			Repositories       []payload.Repository `json:"repositories" binding:"required,dive"`
			ExecutionContainer string               `json:"executionContainer" binding:"required"`
		}{
			Repositories: []payload.Repository{
				{
					Path: "/tmp/testrepo1",
					URL:  "https://github.com/testuser/testrepo1.git",
				},
				{
					Path: "/tmp/testrepo2",
					URL:  "https://github.com/testuser/testrepo2.git",
				},
			},
			ExecutionContainer: "docker",
		},
	}

	namespace := "default"

	createJob(client, namespace, testBuildJob)
}

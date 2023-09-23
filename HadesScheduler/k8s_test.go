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
	getPods(client)
}

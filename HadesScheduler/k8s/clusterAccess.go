package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"log/slog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// initializeKubeconfig initializes a Kubernetes clientset based on the provided configuration.
// If the Kubeconfig field in the provided configuration is not empty, it will be used as the path.
// Otherwise, the kubeconfig file will be loaded from the default location (~/.kube/config).
func initializeKubeconfig(k8sCfg K8sConfigKubeconfig) (*kubernetes.Clientset, error) {
	var kubeConfig *rest.Config
	var err error

	if k8sCfg.Kubeconfig != "" {
		slog.Info("Using explicit kubeconfig", "config", k8sCfg.Kubeconfig)
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", k8sCfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("building config from kubeconfig %s: %w", k8sCfg.Kubeconfig, err)
		}
	} else {
		slog.Info("Kubeconfig not set - using default location")
		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting user home dir: %w", err)
		}
		kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
		slog.Info("Using kubeconfig", "path", kubeConfigPath)
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("building config from default kubeconfig: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("creating Kubernetes clientset: %w", err)
	}

	return clientset, nil
}

func initializeInCluster() *kubernetes.Clientset {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		slog.Error("Failed to load in-cluster config", "error", err)
		return nil
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		slog.Error("Failed to create in-cluster clientset", "error", err)
		return nil
	}
	return cs
}

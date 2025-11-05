package utils

import (
	"os"
	"path/filepath"

	"log/slog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sConfigServiceaccount is used as configuration if used with a service account
type K8sConfigServiceaccount struct {
	K8sConfig
}

type K8sConfig struct {
	// K8sNamespace is the namespace in which the jobs should be scheduled (default: hades-executor)
	// This may change in the future to allow for multiple namespaces
	K8sNamespace string `env:"K8S_NAMESPACE,notEmpty" envDefault:"hades-executor"`

	// K8sConfigMode is used to determine how the Kubernetes client should be configured ("kubeconfig", "serviceaccount" or "operator")
	ConfigMode string `env:"K8S_CONFIG_MODE,notEmpty" envDefault:"kubeconfig"`
}

// K8sConfigKubeconfig is used as configuration if used with a kubeconfig file
type K8sConfigKubeconfig struct {
	K8sConfig
	kubeconfig string `env:"KUBECONFIG"`
}

// initializeKubeconfig initializes a Kubernetes clientset based on the provided configuration.
// If the kubeconfig field in the provided configuration is not empty, it will be used as the path to the kubeconfig file.
// Otherwise, the kubeconfig file will be loaded from the default location in the user's home directory.
// The function will panic if there is an error creating the Kubernetes clientset or getting the user's home directory.
// Returns a pointer to the created Kubernetes clientset.
func InitializeKubeconfig(k8sCfg K8sConfigKubeconfig) *kubernetes.Clientset {

	var kubeConfig *rest.Config

	// Check if kubeconfig is explicitly set
	if k8sCfg.kubeconfig != "" {
		slog.Info("Using explicit kubeconfig", "config", k8sCfg.kubeconfig)
		var err error
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", k8sCfg.kubeconfig)
		if err != nil {
			slog.With("error", err).Error("Error creating Kubernetes clientset")
		}
	} else {
		// TODO: Discuss if this is a good idea - it may be risky to simply assume the default k8s config is ok to use. If the developer has a prod config set up, we would simply use it without asking.
		slog.Info("Kubeconfig not set - using default location")
		// Load kubeconfig from default location
		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			slog.With("error", err).Error("error getting user home dir")
		}
		kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
		slog.Info("Using kubeconfig", "path", kubeConfigPath)

		// Create kubeconfig object
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			slog.With("error", err).Error("error getting Kubernetes clientset")
		}
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		slog.With("error", err).Error("error getting Kubernetes clientset")
	}

	return clientset

}

func InitializeInCluster() *kubernetes.Clientset {
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

package k8s

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	log "github.com/sirupsen/logrus"
)

// initializeKubeconfig initializes a Kubernetes clientset based on the provided configuration.
// If the kubeconfig field in the provided configuration is not empty, it will be used as the path to the kubeconfig file.
// Otherwise, the kubeconfig file will be loaded from the default location in the user's home directory.
// The function will panic if there is an error creating the Kubernetes clientset or getting the user's home directory.
// Returns a pointer to the created Kubernetes clientset.
func initializeKubeconfig(k8sCfg K8sConfigKubeconfig) *kubernetes.Clientset {

	var kubeConfig *rest.Config

	// Check if kubeconfig is explicitly set
	if k8sCfg.kubeconfig != "" {
		log.Infof("Using explicit kubeconfig: %s", k8sCfg.kubeconfig)
		var err error
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", k8sCfg.kubeconfig)
		if err != nil {
			log.WithError(err).Panic("Error creating Kubernetes clientset")
		}
	} else {
		log.Info("Kubeconfig not set - using default location")
		// Load kubeconfig from default location
		userHomeDir, err := os.UserHomeDir()
		if err != nil {
			log.WithError(err).Panic("error getting user home dir")
		}
		kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
		log.Infof("Using kubeconfig: %s", kubeConfigPath)

		// Create kubeconfig object
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			log.WithError(err).Panic("error getting Kubernetes clientset")
		}
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.WithError(err).Panic("error getting Kubernetes clientset")
	}

	return clientset

}

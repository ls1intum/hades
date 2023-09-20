package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	log "github.com/sirupsen/logrus"
)

// This function inizializes the kubeconfig clientset using the kubeconfig file in the useres home directory
func initializeKubeconfig() *kubernetes.Clientset {

	fmt.Println("Get Kubernetes pods")

	// Load kubeconfig from default location
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		log.WithError(err).Error("error getting user home dir")
		os.Exit(1)
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	log.Infof("Using kubeconfig: %s\n", kubeConfigPath)

	// Create kubeconfig object
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		log.WithError(err).Error("error getting Kubernetes clientset")
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.WithError(err).Error("error getting Kubernetes clientset")
		os.Exit(1)
	}

	return clientset

}

func main() {

	clientset := initializeKubeconfig()

	pods, err := clientset.CoreV1().Pods("kube-system").List(context.Background(), v1.ListOptions{})
	if err != nil {
		log.Infof("error getting pods: %v\n", err)
		os.Exit(1)
	}
	for _, pod := range pods.Items {
		log.Infof("Pod name: %s\n", pod.Name)
	}
}

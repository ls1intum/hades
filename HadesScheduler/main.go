package main

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Mtze/HadesCI/shared/queue"
	"github.com/Mtze/HadesCI/shared/utils"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

var BuildQueue *queue.Queue

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
	var cfg utils.RabbitMQConfig
	utils.LoadConfig(&cfg)

	var err error
	rabbitmqURL := fmt.Sprintf("amqp://%s:%s@%s/", cfg.User, cfg.Password, cfg.Url)
	log.Debug("Connecting to RabbitMQ: ", rabbitmqURL)
	BuildQueue, err = queue.Init("builds", rabbitmqURL)

	if err != nil {
		log.Panic(err)
	}

	var forever chan struct{}

	f := func(ch <-chan amqp.Delivery) {
		for d := range ch {
			log.Printf("Received a message: %s", d.Body)
		}
	}
	BuildQueue.Dequeue(f)

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever

	//clientset := initializeKubeconfig()

	//pods, err := clientset.CoreV1().Pods("kube-system").List(context.Background(), v1.ListOptions{})
	//if err != nil {
	//	log.Infof("error getting pods: %v\n", err)
	//	os.Exit(1)
	//}
	//for _, pod := range pods.Items {
	//	log.Infof("Pod name: %s\n", pod.Name)
	//}
}

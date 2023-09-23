package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Mtze/HadesCI/shared/payload"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type JobScheduler interface {
	ScheduleJob(job payload.BuildJob) error
}

type K8sScheduler struct{}

var clientset *kubernetes.Clientset
var namespace *corev1.Namespace

func init() {
	var err error

	clientset = initializeKubeconfig()
	namespace, err = createNamespace(clientset, "hadestesting")

	if err != nil {
		log.Warn("Failed to create hades namespace - Using the existing one")
	}
}

func (k *K8sScheduler) ScheduleJob(buildJob payload.BuildJob) error {

	log.Infof("Scheduling job %s", buildJob.BuildConfig.ExecutionContainer)

	nsName := namespace.Name
	jobName := "testjob"
	jobImage := buildJob.BuildConfig.ExecutionContainer
	cmd := "sleep 100"

	job, err := createJob(clientset,
		nsName,
		&jobName,
		&jobImage,
		&cmd)

	if err != nil {
		log.WithError(err).Error("error creating job")
		return err
	}

	_ = job

	log.Infof("Job %v scheduled to the Cluster", buildJob)

	return nil
}

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

func getPods(clientset *kubernetes.Clientset, namespace string) *corev1.PodList {

	pods, err := clientset.CoreV1().Pods("kube-system").List(context.Background(), v1.ListOptions{})

	if err != nil {
		log.WithError(err).Error("error getting pods")
	}
	for _, pod := range pods.Items {
		log.Infof("Pod name: %s\n", pod.Name)

	}

	return pods
}

func createNamespace(clientset *kubernetes.Clientset, namespace string) (*corev1.Namespace, error) {
	log.Infof("Creating namespace %s", namespace)

	ns, err := clientset.CoreV1().Namespaces().Create(
		context.Background(),
		&corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: namespace,
			},
		}, v1.CreateOptions{})

	if err != nil {
		log.WithError(err).Error("error creating namespace")
		return nil, err
	}

	// sleep for 5 seconds to give the namespace time to be created
	time.Sleep(5 * time.Second)

	return ns, nil
}

func getNamespaces(clientset *kubernetes.Clientset) *corev1.NamespaceList {
	log.Infof("Getting namespaces")

	namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), v1.ListOptions{})

	if err != nil {
		log.WithError(err).Error("error getting namespaces")
	}

	for _, namespace := range namespaces.Items {
		log.Debugf("Namespace name: %s\n", namespace.Name)
	}
	return namespaces
}

func deleteNamespace(clientset *kubernetes.Clientset, namespace string) {
	log.Infof("Deleting namespace %s\n", namespace)

	err := clientset.CoreV1().Namespaces().Delete(context.Background(), namespace, v1.DeleteOptions{})

	if err != nil {
		log.WithError(err).Error("error deleting namespace")
	}
}

func createJob(clientset *kubernetes.Clientset, namespace string, jobName *string, image *string, cmd *string) (*batchv1.Job, error) {
	log.Infof("Creating job %s in namespace %s", *jobName, namespace)

	jobs := clientset.BatchV1().Jobs(namespace)
	var backOffLimit int32 = 0

	jobSpec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *jobName,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    *jobName,
							Image:   *image,
							Command: strings.Split(*cmd, " "),
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
			BackoffLimit: &backOffLimit,
		},
	}

	job, err := jobs.Create(context.TODO(), jobSpec, metav1.CreateOptions{})
	if err != nil {
		log.WithError(err).Error("error creating job")
		return nil, err
	}

	//print job details
	log.Infof("Created K8s job  %s successfully", jobName)
	log.Debugf("Job details: %v", job)
	return job, nil
}

package kube

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/Mtze/HadesCI/shared/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	waitForNamespace = 5 * time.Second
)

type JobScheduler interface {
	ScheduleJob(job payload.BuildJob) error
}

type Scheduler struct{}

var clientset *kubernetes.Clientset
var namespace *corev1.Namespace

func init() {
	log.Debug("Kube init function called")
	var k8sCfg utils.K8sConfig
	utils.LoadConfig(&k8sCfg)

	var err error

	hadesCInamespace := k8sCfg.HadesCInamespace

	clientset = initializeKubeconfig()

	// Ensure that the namespace exists

	log.Debugf("Ensure that %s namespace exists", hadesCInamespace)
	namespace, err = getNamespace(clientset, hadesCInamespace)
	if err != nil {
		log.Infof("Namespace '%s' does not exist - Trying creating a new one", hadesCInamespace)

		namespace, err = createNamespace(clientset, hadesCInamespace)
		if err != nil {
			log.WithError(err).Panic("error getting existing namespace - no more options to try - exiting")
		}
		log.Infof("Namespace '%s' created", namespace.Name)
	}
	log.Debugf("Using namespace '%s'", namespace.Name)

}

func (k *Scheduler) ScheduleJob(buildJob payload.BuildJob) error {

	log.Infof("Scheduling job %s", buildJob.BuildConfig.ExecutionContainer)

	job, err := createJob(clientset, namespace.Name, buildJob)

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

	// Load kubeconfig from default location
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		log.WithError(err).Panic("error getting user home dir")
	}
	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	log.Infof("Using kubeconfig: %s", kubeConfigPath)

	// Create kubeconfig object
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		log.WithError(err).Panic("error getting Kubernetes clientset")
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.WithError(err).Panic("error getting Kubernetes clientset")
	}

	return clientset

}

func getPods(clientset *kubernetes.Clientset, namespace string) *corev1.PodList {

	pods, err := clientset.CoreV1().Pods("kube-system").List(context.Background(), v1.ListOptions{})

	if err != nil {
		log.WithError(err).Error("error getting pods")
	}
	for _, pod := range pods.Items {
		log.Infof("Pod name: %s", pod.Name)

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
	time.Sleep(waitForNamespace)

	return ns, nil
}

func getNamespaces(clientset *kubernetes.Clientset) *corev1.NamespaceList {
	log.Debugf("Getting namespaces")

	namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), v1.ListOptions{})

	if err != nil {
		log.WithError(err).Error("error getting namespaces")
	}

	for _, namespace := range namespaces.Items {
		log.Debugf("Namespace name: %s", namespace.Name)
	}
	return namespaces
}

func getNamespace(clientset *kubernetes.Clientset, namespace string) (*corev1.Namespace, error) {
	log.Debugf("Getting namespace %s", namespace)

	ns, err := clientset.CoreV1().Namespaces().Get(context.Background(), namespace, v1.GetOptions{})

	if err != nil {
		log.WithError(err).Error("error getting namespace")
		return nil, err
	}

	return ns, nil
}

func deleteNamespace(clientset *kubernetes.Clientset, namespace string) {
	log.Infof("Deleting namespace %s", namespace)

	err := clientset.CoreV1().Namespaces().Delete(context.Background(), namespace, v1.DeleteOptions{})

	if err != nil {
		log.WithError(err).Error("error deleting namespace")
	}
}

func createJob(clientset *kubernetes.Clientset, namespace string, buildJob payload.BuildJob) (*batchv1.Job, error) {
	log.Infof("Creating job %v in namespace %s", buildJob, namespace)

	buildCommand := "sleep 10"

	jobs := clientset.BatchV1().Jobs(namespace)
	var backOffLimit int32 = 0

	jobSpec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildJob.Name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    buildJob.Name,
							Image:   buildJob.BuildConfig.ExecutionContainer,
							Command: strings.Split(buildCommand, " "),
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
	log.Infof("Created K8s job  %s successfully", buildJob.Name)
	log.Debugf("Job details: %v", job)
	return job, nil
}
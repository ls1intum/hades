package k8s

import (
	"context"
	"errors"
	"fmt"
	"k8s.io/client-go/rest"

	"log/slog"

	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
	"k8s.io/client-go/kubernetes"
)

type Scheduler struct {
	// TODO: This may be problematic - We need to clarify how to access the cluster with the service account and find a solution that is compatible with both modes
	k8sClient *kubernetes.Clientset
	namespace string
}

type K8sConfig struct {
	// K8sNamespace is the namespace in which the jobs should be scheduled (default: hades-executor)
	// This may change in the future to allow for multiple namespaces
	K8sNamespace string `env:"K8S_NAMESPACE,notEmpty" envDefault:"hades-executor"`

	// K8sConfigMode is used to determine how the Kubernetes client should be configured ("kubeconfig" or "serviceaccount")
	ConfigMode string `env:"K8S_CONFIG_MODE,notEmpty" envDefault:"kubeconfig"`
}

// K8sConfigKubeconfig is used as configuration if used with a kubeconfig file
type K8sConfigKubeconfig struct {
	K8sConfig
	Kubeconfig string `env:"KUBECONFIG"`
}

// K8sConfigServiceaccount is used as configuration if used with a service account
type K8sConfigServiceaccount struct {
	K8sConfig
}

func NewK8sScheduler() (Scheduler, error) {
	slog.Debug("Initializing Kubernetes scheduler")

	// Load the user provided Kubernetes configuration
	var k8sCfg K8sConfig
	utils.LoadConfig(&k8sCfg)
	slog.Debug("Kubernetes config", "config", k8sCfg)

	// Initialize the Kubernetes scheduler
	slog.Info("Initializing Kubernetes client")
	scheduler, err := initializeClusterAccess(k8sCfg)
	if err != nil {
		return Scheduler{}, fmt.Errorf("initialize k8s access: %w", err)
	}

	// TODO: Check cluster connection and print cluster nodes to log

	return scheduler, nil
}

// Create a Kubernetes clientset based on the provided configuration
func initializeClusterAccess(k8sCfg K8sConfig) (Scheduler, error) {
	switch k8sCfg.ConfigMode {
	case "kubeconfig":
		slog.Info("Using kubeconfig for Kubernetes access")

		var K8sConfigKub K8sConfigKubeconfig
		utils.LoadConfig(&K8sConfigKub)

		client := initializeKubeconfig(K8sConfigKub)
		if client == nil {
			return Scheduler{}, errors.New("failed to build client from kubeconfig")
		}
		return Scheduler{k8sClient: client, namespace: k8sCfg.K8sNamespace}, nil

	case "serviceaccount":
		slog.Info("Using service account for Kubernetes access")

		config, err := rest.InClusterConfig()
		if err != nil {
			return Scheduler{}, fmt.Errorf("load in-cluster config: %w", err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return Scheduler{}, fmt.Errorf("create clientset: %w", err)
		}

		return Scheduler{k8sClient: clientset, namespace: k8sCfg.K8sNamespace}, nil

	default:
		return Scheduler{}, fmt.Errorf("invalid config mode %q", k8sCfg.ConfigMode)
	}
}

func (k Scheduler) ScheduleJob(ctx context.Context, job payload.QueuePayload) error {
	slog.Debug("Scheduling job in Kubernetes")
	k8sJob := K8sJob{
		QueuePayload:     job,
		k8sClient:        k.k8sClient,
		namespace:        k.namespace,
		sharedVolumeName: "shared",
	}
	return k8sJob.execute(ctx)
}

package k8s

import (
	"context"

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
	kubeconfig string `env:"KUBECONFIG"`
}

// K8sConfigServiceaccount is used as configuration if used with a service account
type K8sConfigServiceaccount struct {
}

func NewK8sScheduler() Scheduler {
	slog.Debug("Initializing Kubernetes scheduler")

	// Load the user provided Kubernetes configuration
	var k8sCfg K8sConfig
	utils.LoadConfig(&k8sCfg)
	slog.Debug("Kubernetes config", "config", k8sCfg)

	// Initialize the Kubernetes scheduler
	slog.Info("Initializing Kubernetes client")
	scheduler := initializeClusterAccess(k8sCfg)

	// Add the namespace to the scheduler
	slog.Info("Creating namespace in Kubernetes")
	_, err := createNamespace(context.Background(), scheduler.k8sClient, k8sCfg.K8sNamespace)
	if err != nil {
		// TODO: This may fail if the namespace already exists - we need to handle that case with a check
		slog.With("error", err).Info("Failed to create namespace in Kubernetes")
	}

	return scheduler
}

// Create a Kubernetes clientset based on the provided configuration
func initializeClusterAccess(k8sCfg K8sConfig) Scheduler {
	switch k8sCfg.ConfigMode {
	case "kubeconfig":
		slog.Info("Using kubeconfig for Kubernetes access")

		var K8sConfigKub K8sConfigKubeconfig
		utils.LoadConfig(&K8sConfigKub)

		return Scheduler{
			k8sClient: initializeKubeconfig(K8sConfigKub),
			namespace: k8sCfg.K8sNamespace,
		}

	case "serviceaccount":
		slog.Info("Using service account for Kubernetes access")

		var K8sConfigSvc K8sConfigServiceaccount
		utils.LoadConfig(&K8sConfigSvc)

		slog.Warn("Service account mode not yet implemented")
		return Scheduler{}

	default:
		slog.Error("Invalid Kubernetes config mode specified", "config_mode", k8sCfg.ConfigMode)
		return Scheduler{}
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

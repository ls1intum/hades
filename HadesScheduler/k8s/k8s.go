package k8s

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Scheduler struct {
	// TODO: This may be problematic - We need to clarify how to access the cluster with the service account and find a solution that is compatible with both modes
	k8sClient   *kubernetes.Clientset
	dynClient   dynamic.Interface
	namespace   string
	useOperator bool
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

// K8sConfigServiceaccount is used as configuration if used with a service account
type K8sConfigServiceaccount struct {
	K8sConfig
}

var buildJobGVR = schema.GroupVersionResource{
	Group:    "build.hades.tum.de",
	Version:  "v1",
	Resource: "buildjobs",
}

//// BuildJobGVRConfig 通过环境变量配置 BuildJob 的 Group/Version/Resource
//type BuildJobGVRConfig struct {
//	Group    string `env:"BUILDJOB_GROUP,notEmpty"    envDefault:"build.hades.tum.de"`
//	Version  string `env:"BUILDJOB_VERSION,notEmpty"  envDefault:"v1"`
//	Resource string `env:"BUILDJOB_RESOURCE,notEmpty" envDefault:"buildjobs"`
//}
//
//// ToGVR 转换为 Kubernetes 的 schema.GroupVersionResource
//func (c BuildJobGVRConfig) ToGVR() schema.GroupVersionResource {
//	return schema.GroupVersionResource{
//		Group:    c.Group,
//		Version:  c.Version,
//		Resource: c.Resource,
//	}
//}

func NewK8sScheduler() Scheduler {
	slog.Debug("Initializing Kubernetes scheduler")

	// Load the user provided Kubernetes configuration
	var k8sCfg K8sConfig
	utils.LoadConfig(&k8sCfg)
	slog.Debug("Kubernetes config", "config", k8sCfg)

	// Initialize the Kubernetes scheduler
	slog.Info("Initializing Kubernetes client")
	scheduler := initializeClusterAccess(k8sCfg)

	// TODO: Check cluster connection and print cluster nodes to log

	// Add the namespace to the scheduler, ignore if we are using the operator mode
	if !scheduler.useOperator && scheduler.k8sClient != nil {
		slog.Info("Creating namespace in Kubernetes")
		_, err := createNamespace(context.Background(), scheduler.k8sClient, k8sCfg.K8sNamespace)
		if err != nil {
			// TODO: This may fail if the namespace already exists - we need to handle that case with a check
			slog.With("error", err).Info("Failed to create namespace in Kubernetes")
		}
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

		return Scheduler{
			k8sClient: initializeInCluster(),
			namespace: k8sCfg.K8sNamespace,
		}
	case "operator":
		slog.Info("Using operator mode (dynamic client)")
		rc, err := rest.InClusterConfig()
		if err != nil {
			slog.Warn("InClusterConfig failed, fallback to KUBECONFIG", "error", err)
			kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: clientcmd.RecommendedHomeFile},
				&clientcmd.ConfigOverrides{},
			)
			rc, err = kubeconfig.ClientConfig()
			if err != nil {
				slog.Error("Failed to build rest.Config for operator mode", "error", err)
				return Scheduler{}
			}
		}

		dyn, err := dynamic.NewForConfig(rc)
		if err != nil {
			slog.Error("Failed to create dynamic client", "error", err)
			return Scheduler{}
		}

		return Scheduler{
			dynClient:   dyn,
			namespace:   k8sCfg.K8sNamespace,
			useOperator: true,
		}

	default:
		slog.Error("Invalid Kubernetes config mode specified", "config_mode", k8sCfg.ConfigMode)
		return Scheduler{}
	}
}

func (k Scheduler) ScheduleJob(ctx context.Context, job payload.QueuePayload) error {
	if k.useOperator {
		slog.Debug("Scheduling job via Operator (creating BuildJob CR)")
		return k.createBuildJobCR(ctx, job)
	}

	slog.Debug("Scheduling job in Kubernetes (legacy direct mode)")
	k8sJob := K8sJob{
		QueuePayload:     job,
		k8sClient:        k.k8sClient,
		namespace:        k.namespace,
		sharedVolumeName: "shared",
	}
	return k8sJob.execute(ctx)
}

func (k Scheduler) createBuildJobCR(ctx context.Context, job payload.QueuePayload) error {
	if k.dynClient == nil {
		return fmt.Errorf("dynamic client is nil: operator mode not initialized")
	}

	// assemble steps
	steps := make([]map[string]interface{}, 0, len(job.Steps))
	for _, s := range job.Steps {
		sm := map[string]interface{}{
			"id":    s.ID,
			"name":  s.Name,
			"image": s.Image,
		}
		if s.Script != "" {
			sm["script"] = s.Script
		}
		if len(s.Metadata) > 0 {
			sm["metadata"] = s.Metadata
		}
		if s.CPULimit > 0 {
			sm["cpuLimit"] = fmt.Sprintf("%d", s.CPULimit)
		}
		if s.MemoryLimit != "" {
			sm["memoryLimit"] = s.MemoryLimit
		}
		steps = append(steps, sm)
	}

	// assemble the BuildJob CR object
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "build.hades.tum.de/v1",
			"kind":       "BuildJob",
			"metadata": map[string]interface{}{
				"name":      job.ID.String(),
				"namespace": k.namespace,
				"labels": map[string]interface{}{
					"hades/job-id": job.ID.String(),
					"hades/source": "scheduler",
				},
			},
			"spec": map[string]interface{}{
				"name":     job.Name,
				"priority": jobPriorityInt32(job),
				"metadata": job.Metadata,
				"steps":    steps,
			},
		},
	}

	_, err := k.dynClient.Resource(buildJobGVR).Namespace(k.namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			slog.Info("BuildJob already exists (idempotent)", "name", job.ID.String())
			return nil
		}
		return err
	}

	slog.Info("Created BuildJob CR", "name", job.ID.String(), "namespace", k.namespace)
	return nil
}

func jobPriorityInt32(job payload.QueuePayload) int32 {
	//TODO: Operator might not need the priority value
	return int32(0)
}

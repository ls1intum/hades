package k8s

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
	"github.com/nats-io/nats.go"
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
	k8sClient *kubernetes.Clientset
	dynClient dynamic.Interface
	namespace string
	config    K8sConfig
	publisher log.NATSPublisher
}

type K8sConfig struct {
	// K8sNamespace is the namespace in which the jobs should be scheduled (default: hades-executor)
	K8sNamespace string `env:"K8S_NAMESPACE,notEmpty" envDefault:"hades-executor"`

	// ConfigMode is used to determine how the Kubernetes client should be configured ("kubeconfig", "serviceaccount" or "operator")
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

type BuildJobGVRConfig struct {
	Group    string `env:"BUILDJOB_GROUP,notEmpty"    envDefault:"build.hades.tum.de"`
	Version  string `env:"BUILDJOB_VERSION,notEmpty"  envDefault:"v1"`
	Resource string `env:"BUILDJOB_RESOURCE,notEmpty" envDefault:"buildjobs"`
}

func NewK8sScheduler(nc *nats.Conn) (*Scheduler, error) {
	slog.Debug("Initializing Kubernetes scheduler")

	var k8sCfg K8sConfig
	utils.LoadConfig(&k8sCfg)
	slog.Debug("Kubernetes config", "config", k8sCfg)

	slog.Info("Initializing Kubernetes client")
	scheduler, err := initializeClusterAccess(k8sCfg)
	if err != nil {
		return nil, err
	}

	if k8sCfg.ConfigMode != "operator" && scheduler.k8sClient != nil {
		slog.Info("Creating namespace in Kubernetes")
		_, err := createNamespace(context.Background(), scheduler.k8sClient, k8sCfg.K8sNamespace)
		if err != nil {
			slog.Error("Failed to create namespace in Kubernetes", "error", err)
			return nil, err
		}
	}

	if nc != nil {
		publisher, err := log.NewNATSPublisher(nc)
		if err != nil {
			return nil, fmt.Errorf("creating NATS publisher: %w", err)
		}
		scheduler.publisher = *publisher
	} else {
		slog.Warn("NATS connection is nil, publisher not created, logs will not be published")
	}

	return &scheduler, nil
}

func initializeClusterAccess(k8sCfg K8sConfig) (Scheduler, error) {
	switch k8sCfg.ConfigMode {
	case "kubeconfig":
		return initializeKubeconfigAccess(k8sCfg)
	case "serviceaccount":
		return initializeServiceAccountAccess(k8sCfg)
	case "operator":
		return initializeOperatorAccess(k8sCfg)
	default:
		slog.Error("Invalid Kubernetes config mode specified", "config_mode", k8sCfg.ConfigMode)
		return Scheduler{}, fmt.Errorf("invalid Kubernetes config mode: %s", k8sCfg.ConfigMode)
	}
}

func initializeKubeconfigAccess(k8sCfg K8sConfig) (Scheduler, error) {
	slog.Info("Using kubeconfig for Kubernetes access")

	var k8sConfigKub K8sConfigKubeconfig
	utils.LoadConfig(&k8sConfigKub)

	clientset, err := initializeKubeconfig(k8sConfigKub)
	if err != nil {
		return Scheduler{}, err
	}

	return Scheduler{
		k8sClient: clientset,
		namespace: k8sCfg.K8sNamespace,
		config:    k8sCfg,
	}, nil
}

func initializeServiceAccountAccess(k8sCfg K8sConfig) (Scheduler, error) {
	slog.Info("Using service account for Kubernetes access")

	var k8sConfigSvc K8sConfigServiceaccount
	utils.LoadConfig(&k8sConfigSvc)

	clientset := initializeInCluster()
	if clientset == nil {
		return Scheduler{}, fmt.Errorf("failed to initialize in-cluster Kubernetes client")
	}

	return Scheduler{
		k8sClient: clientset,
		namespace: k8sCfg.K8sNamespace,
		config:    k8sCfg,
	}, nil
}

func initializeOperatorAccess(k8sCfg K8sConfig) (Scheduler, error) {
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
			return Scheduler{}, fmt.Errorf("building rest.Config for operator mode: %w", err)
		}
	}

	dyn, err := dynamic.NewForConfig(rc)
	if err != nil {
		return Scheduler{}, fmt.Errorf("creating dynamic client: %w", err)
	}

	return Scheduler{
		dynClient: dyn,
		namespace: k8sCfg.K8sNamespace,
		config:    k8sCfg,
	}, nil
}

func (k Scheduler) ScheduleJob(ctx context.Context, job payload.QueuePayload) error {
	if k.config.ConfigMode == "operator" {
		slog.Debug("Scheduling job via Operator (creating BuildJob CR)")
		return k.createBuildJobCR(ctx, job)
	}

	slog.Debug("Scheduling job in Kubernetes (legacy direct mode)")
	k8sJob := K8sJob{
		QueuePayload:     job,
		k8sClient:        k.k8sClient,
		namespace:        k.namespace,
		sharedVolumeName: "shared",
		publisher:        k.publisher,
	}
	return k8sJob.execute(ctx)
}

func (c BuildJobGVRConfig) ToGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    c.Group,
		Version:  c.Version,
		Resource: c.Resource,
	}
}

func (k Scheduler) createBuildJobCR(ctx context.Context, job payload.QueuePayload) error {
	if k.dynClient == nil {
		return fmt.Errorf("dynamic client is nil: operator mode not initialized")
	}

	var gvrCfg BuildJobGVRConfig
	utils.LoadConfig(&gvrCfg)
	buildJobGVR := gvrCfg.ToGVR()

	labels := map[string]interface{}{
		"hades/job-id": job.ID.String(),
		"hades/source": "scheduler",
	}

	if job.Metadata != nil {
		if v, ok := job.Metadata["hades.tum.de/priority"]; ok && v != "" {
			labels["hades.tum.de/priority"] = v
		}
	}

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

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "build.hades.tum.de/v1",
			"kind":       "BuildJob",
			"metadata": map[string]interface{}{
				"name":      job.ID.String(),
				"namespace": k.namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"name":     job.Name,
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

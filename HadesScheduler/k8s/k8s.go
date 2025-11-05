package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"sync"

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
	// TODO: This may be problematic - We need to clarify how to access the cluster with the service account and find a solution that is compatible with both modes
	k8sClient   *kubernetes.Clientset
	dynClient   dynamic.Interface
	namespace   string
	useOperator bool
	nc          *nats.Conn
	subOnce     sync.Once
	eventSub    *nats.Subscription
}

type BuildJobGVRConfig struct {
	Group    string `env:"BUILDJOB_GROUP,notEmpty"    envDefault:"build.hades.tum.de"`
	Version  string `env:"BUILDJOB_VERSION,notEmpty"  envDefault:"v1"`
	Resource string `env:"BUILDJOB_RESOURCE,notEmpty" envDefault:"buildjobs"`
}

func (c BuildJobGVRConfig) ToGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    c.Group,
		Version:  c.Version,
		Resource: c.Resource,
	}
}

func NewK8sScheduler() (*Scheduler, error) {
	slog.Debug("Initializing Kubernetes scheduler")

	// Load the user provided Kubernetes configuration
	var k8sCfg utils.K8sConfig
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
			return nil, err
		}
	}

	return &scheduler, nil
}

func (k *Scheduler) SetNatsConnection(nc *nats.Conn) *Scheduler {
	if nc != nil {
		k.nc = nc
	} else {
		slog.Warn("NATS connection is nil, logs will not be published")
	}
	return k
}

func (k *Scheduler) ensureEventSub() {
	k.subOnce.Do(func() {
		if k.nc == nil {
			slog.Error("NATS connection is nil; cannot subscribe buildjob events")
			return
		}
		// Queue subscription to avoid multiple schedulers processing the same event in case of multi-replica deployment
		sub, err := k.nc.QueueSubscribe("buildjob.events.*", "hades-scheduler", k.handleBuildJobEvent)
		if err != nil {
			slog.Error("Failed to subscribe buildjob.events.*", "error", err)
			return
		}
		k.eventSub = sub
		slog.Info("Subscribed to buildjob.events.* (queue=hades-scheduler)")
	})
}

// Create a Kubernetes clientset based on the provided configuration
func initializeClusterAccess(k8sCfg utils.K8sConfig) Scheduler {
	switch k8sCfg.ConfigMode {
	case "kubeconfig":
		slog.Info("Using kubeconfig for Kubernetes access")

		var K8sConfigKub utils.K8sConfigKubeconfig
		utils.LoadConfig(&K8sConfigKub)

		return Scheduler{
			k8sClient: utils.InitializeKubeconfig(K8sConfigKub),
			namespace: k8sCfg.K8sNamespace,
		}

	case "serviceaccount":
		slog.Info("Using service account for Kubernetes access")

		var K8sConfigSvc utils.K8sConfigServiceaccount
		utils.LoadConfig(&K8sConfigSvc)

		return Scheduler{
			k8sClient: utils.InitializeInCluster(),
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

		kcs, err := kubernetes.NewForConfig(rc)
		if err != nil {
			slog.Error("Failed to create typed clientset", "error", err)
			return Scheduler{}
		}

		dyn, err := dynamic.NewForConfig(rc)
		if err != nil {
			slog.Error("Failed to create dynamic client", "error", err)
			return Scheduler{}
		}

		return Scheduler{
			k8sClient:   kcs,
			dynClient:   dyn,
			namespace:   k8sCfg.K8sNamespace,
			useOperator: true,
		}

	default:
		slog.Error("Invalid Kubernetes config mode specified", "config_mode", k8sCfg.ConfigMode)
		return Scheduler{}
	}
}

// Event received from Operator
func (k *Scheduler) handleBuildJobEvent(msg *nats.Msg) {
	var event map[string]any
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal BuildJob event: %v", err)
		return
	}

	buildJobName, ok := event["buildJob"].(string)
	if !ok {
		log.Printf("BuildJob event missing or invalid 'buildJob' field: %v", event["buildJob"])
		return
	}
	status, ok := event["status"].(string)
	if !ok {
		log.Printf("BuildJob event missing or invalid 'status' field: %v", event["status"])
		return
	}

	switch status {
	case "pod_running":
		log.Printf("BuildJob %s pod is running", buildJobName)
	case "completed":
		log.Printf("BuildJob %s completed, succeeded %v", buildJobName, event["succeeded"])
	}
}

func (k *Scheduler) ScheduleJob(ctx context.Context, job payload.QueuePayload) error {
	if k.useOperator {
		slog.Debug("Scheduling job via Operator (creating BuildJob CR)")
		k.ensureEventSub()
		return k.createBuildJobCR(ctx, job)
	}

	slog.Debug("Scheduling job in Kubernetes (legacy direct mode)")
	k8sJob := K8sJob{
		QueuePayload:     job,
		k8sClient:        k.k8sClient,
		namespace:        k.namespace,
		sharedVolumeName: "shared",
		nc:               k.nc,
	}
	return k8sJob.execute(ctx)
}

func (k *Scheduler) createBuildJobCR(ctx context.Context, job payload.QueuePayload) error {
	if k.dynClient == nil {
		return fmt.Errorf("dynamic client is nil: operator mode not initialized")
	}

	var gvrCfg BuildJobGVRConfig
	utils.LoadConfig(&gvrCfg)
	buildJobGVR := gvrCfg.ToGVR()

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

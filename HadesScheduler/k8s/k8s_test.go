package k8s

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var K8sVersion = "v1.27.1-k3s1"

type K8sTestSuite struct {
	suite.Suite
	clientset    *kubernetes.Clientset
	k3sContainer *k3s.K3sContainer
	ctx          context.Context
}

func (suite *K8sTestSuite) SetupSuite() {
	ctx := context.Background()
	suite.ctx = ctx

	k3sContainer, err := k3s.RunContainer(ctx,
		testcontainers.WithImage("docker.io/rancher/k3s:"+K8sVersion),
	)
	if err != nil {
		suite.T().Fatalf("failed to start container: %s", err)
	}
	suite.k3sContainer = k3sContainer

	state, err := k3sContainer.State(ctx)
	if err != nil {
		suite.T().Fatalf("failed to get container state: %s", err) // nolint:gocritic
	}

	fmt.Println(state.Running)

	kubeConfigYaml, err := k3sContainer.GetKubeConfig(ctx)
	if err != nil {
		suite.T().Fatalf("failed to get kubeconfig: %s", err)
	}

	restcfg, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	if err != nil {
		suite.T().Fatalf("failed to create rest config: %s", err)
	}

	k8s, err := kubernetes.NewForConfig(restcfg)
	if err != nil {
		suite.T().Fatalf("failed to create k8s client: %s", err)
	}

	suite.clientset = k8s
}

func (suite *K8sTestSuite) TearDownSuite() {
	if err := suite.k3sContainer.Terminate(suite.ctx); err != nil {
		suite.T().Fatalf("failed to terminate container: %s", err)
	}
}

// --- JobK8sSuite ---
// TODO: For some reason this cannot be put in a separate file - The K8sTestSuite is not found in that case
type JobK8sSuite struct {
	K8sTestSuite
}

func (suite *JobK8sSuite) TestConfigMapCreate() {
	job := K8sJob{
		QueuePayload: payload.QueuePayload{
			ID: uuid.New(),
			Steps: []payload.Step{
				{ID: 1, Script: "echo 'Step 1'"},
				{ID: 2, Script: "echo 'Step 2'"},
			},
		},
	}

	// Call configMapSpec on the K8sJob
	configMap := job.configMapSpec()

	// Create the ConfigMap
	//TODO: Is it a good idea to have a background context here?
	scheduledCM, err := suite.clientset.CoreV1().ConfigMaps("default").Create(context.Background(), configMap, metav1.CreateOptions{})
	assert.Nil(suite.T(), err)
	suite.T().Log("ConfigMap created: ", scheduledCM)

	// Get configMap from Kubernetes
	actualCM, err := suite.clientset.CoreV1().ConfigMaps("default").Get(context.Background(), job.ID.String(), metav1.GetOptions{})
	assert.Nil(suite.T(), err)
	suite.T().Log("ConfigMap retrieved: ", actualCM)

	assert.Equal(suite.T(), scheduledCM, actualCM)
}

func TestJobs(t *testing.T) {
	suite.Run(t, new(JobK8sSuite))
}

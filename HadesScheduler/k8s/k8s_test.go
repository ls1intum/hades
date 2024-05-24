package k8s

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
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

func setupK3sCluster(suite *K8sTestSuite) {
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

func tearDownK3sCluster(suite *K8sTestSuite) {
	if err := suite.k3sContainer.Terminate(suite.ctx); err != nil {
		suite.T().Fatalf("failed to terminate container: %s", err)
	}
}

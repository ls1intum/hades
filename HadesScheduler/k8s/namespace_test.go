package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceTestSuite struct {
	K8sTestSuite
}

func (suite *NamespaceTestSuite) TearDownSuite() {
	// Show all namespaces in the cluster
	namespaces, _ := suite.clientset.CoreV1().Namespaces().List(suite.ctx, v1.ListOptions{})
	for _, ns := range namespaces.Items {
		suite.T().Logf("Namespace: %s - %v", ns.Name, ns.Status)
	}
}

func (suite *NamespaceTestSuite) TestCreateNamespace() {

	namespace := "test-create"
	_, err := createNamespace(suite.ctx, suite.clientset, namespace)
	assert.Nil(suite.T(), err)

	// Check if the namespace was created
	_, err = suite.clientset.CoreV1().Namespaces().Get(suite.ctx, namespace, v1.GetOptions{})

	assert.Nil(suite.T(), err)
}

func (suite *NamespaceTestSuite) TestDeleteNamespace() {

	namespace := "test-delete"
	_, err := createNamespace(suite.ctx, suite.clientset, namespace)
	assert.Nil(suite.T(), err)

	err = deleteNamespace(suite.ctx, suite.clientset, namespace)
	assert.Nil(suite.T(), err)

	// Check if the namespace is in terminating state - It may take some time until the namespace is deleted completely
	ns, _ := suite.clientset.CoreV1().Namespaces().Get(suite.ctx, namespace, v1.GetOptions{})
	assert.Equal(suite.T(), corev1.NamespacePhase("Terminating"), ns.Status.Phase, "Namespace should be in Terminating state")

}

func TestNamespaces(t *testing.T) {
	suite.Run(t, new(NamespaceTestSuite))
}

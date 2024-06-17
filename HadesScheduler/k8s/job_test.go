package k8s

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobUnitTestSuite struct {
	suite.Suite
}

func (suite *JobUnitTestSuite) TestConfigMapSpec() {
	// Create a new K8sJob with a QueuePayload that has a few steps
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

	// Assert that the ConfigMap's name is the same as the QueuePayload's ID
	assert.Equal(suite.T(), job.ID.String(), configMap.ObjectMeta.Name)

	// Assert that the ConfigMap's data contains a key for each step in the QueuePayload
	for _, step := range job.Steps {
		script, ok := configMap.Data[step.IDstring()]
		assert.True(suite.T(), ok, "ConfigMap should contain key for step")
		assert.Equal(suite.T(), step.Script, script, "Script for step in ConfigMap should match step's script")
	}
}
func (suite *JobUnitTestSuite) TestVolumeSpec() {
	// Create a new K8sJob with a QueuePayload that has a few steps
	job := K8sJob{
		QueuePayload: payload.QueuePayload{
			ID: uuid.New(),
			Steps: []payload.Step{
				{ID: 1, Script: "echo 'Step 1'"},
				{ID: 2, Script: "echo 'Step 2'"},
			},
		},
	}

	// Create a ConfigMap for testing
	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: job.ID.String(),
		},
		Data: map[string]string{
			"1": "echo 'Step 1'",
			"2": "echo 'Step 2'",
		},
	}
	suite.T().Logf("ConfigMap: %+v", configMap)

	// Call volumeSpec on the K8sJob
	volumeSpec := job.volumeSpec(configMap)

	// Assert that the number of volumes matches the number of steps in the QueuePayload
	assert.Len(suite.T(), volumeSpec, len(job.Steps))

	// Assert that each volume has the correct name and ConfigMap reference
	for i, step := range job.Steps {
		assert.Equal(suite.T(), step.IDstring(), volumeSpec[i].Name)
		assert.Equal(suite.T(), configMap.Name, volumeSpec[i].VolumeSource.ConfigMap.LocalObjectReference.Name)
	}

	// Assert that each volume has the correct key and path
	for i, step := range job.Steps {
		assert.Equal(suite.T(), step.IDstring(), volumeSpec[i].VolumeSource.ConfigMap.Items[0].Key)
	}
}

func TestJob(t *testing.T) {
	suite.Run(t, new(JobUnitTestSuite))
}

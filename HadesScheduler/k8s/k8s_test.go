package k8s

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildBuildJobObject verifies the payload-to-BuildJob mapping used by the
// operator-mode scheduler: name, namespace, labels and steps.
func TestBuildBuildJobObject(t *testing.T) {
	jobID := uuid.New()
	job := payload.QueuePayload{
		ID:   jobID,
		Name: "test-job",
		Steps: []payload.Step{
			{ID: 1, Name: "build", Image: "golang:1.21", Script: "go build"},
			{ID: 2, Name: "test", Image: "golang:1.21", Script: "go test"},
		},
	}

	obj := buildBuildJobObject(job, "hades-executor")

	assert.Equal(t, "build.hades.tum.de/v1", obj.GetAPIVersion())
	assert.Equal(t, "BuildJob", obj.GetKind())
	assert.Equal(t, jobID.String(), obj.GetName())
	assert.Equal(t, "hades-executor", obj.GetNamespace())
	assert.Equal(t, jobID.String(), obj.GetLabels()["hades/job-id"])
	assert.Equal(t, "scheduler", obj.GetLabels()["hades/source"])

	spec, ok := obj.Object["spec"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-job", spec["name"])

	steps, ok := spec["steps"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, steps, 2)
	assert.Equal(t, "build", steps[0]["name"])
	assert.Equal(t, "go test", steps[1]["script"])
}

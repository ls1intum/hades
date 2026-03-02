package main

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/ls1intum/hades/shared/payload"
)

// baseJob returns a QueuePayload with sensitive metadata on the job and its steps.
func baseJob() payload.QueuePayload {
	return payload.QueuePayload{
		ID:       uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:     "test-job",
		Metadata: map[string]string{"secret": "value", "token": "abc123"},
		Steps: []payload.Step{
			{ID: 1, Name: "build", Image: "alpine", Metadata: map[string]string{"key": "val"}},
			{ID: 2, Name: "test", Image: "golang", Metadata: map[string]string{"pass": "word"}},
		},
	}
}

// assertSanitized verifies that the result is valid JSON with empty metadata.
func assertSanitized(t *testing.T, result string) {
	t.Helper()

	if result == "" {
		t.Fatal("expected non-empty result, got empty string")
	}

	var out payload.QueuePayload
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if len(out.Metadata) != 0 {
		t.Errorf("expected job metadata to be empty, got %v", out.Metadata)
	}
	for i, step := range out.Steps {
		if len(step.Metadata) != 0 {
			t.Errorf("expected step[%d] metadata to be empty, got %v", i, step.Metadata)
		}
	}
}

// assertPreserved verifies that non-metadata fields survive sanitization.
func assertPreserved(t *testing.T, result string, original payload.QueuePayload) {
	t.Helper()

	var out payload.QueuePayload
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if out.ID != original.ID {
		t.Errorf("ID changed: got %v, want %v", out.ID, original.ID)
	}
	if out.Name != original.Name {
		t.Errorf("Name changed: got %q, want %q", out.Name, original.Name)
	}
	if len(out.Steps) != len(original.Steps) {
		t.Errorf("Steps length changed: got %d, want %d", len(out.Steps), len(original.Steps))
		return
	}
	for i, step := range out.Steps {
		if step.Name != original.Steps[i].Name {
			t.Errorf("step[%d].Name changed: got %q, want %q", i, step.Name, original.Steps[i].Name)
		}
		if step.Image != original.Steps[i].Image {
			t.Errorf("step[%d].Image changed: got %q, want %q", i, step.Image, original.Steps[i].Image)
		}
	}
}

func TestSafePayloadFormat_MetadataIsStripped(t *testing.T) {
	job := baseJob()
	result := SafePayloadFormat(job)
	assertSanitized(t, result)
}

func TestSafePayloadFormat_NonMetadataFieldsPreserved(t *testing.T) {
	job := baseJob()
	result := SafePayloadFormat(job)
	assertPreserved(t, result, job)
}

func TestSafePayloadFormat_AllInputTypesAreEquivalent(t *testing.T) {
	job := baseJob()
	jobBytes, _ := json.Marshal(job)
	jobString := string(jobBytes)

	fromStruct := SafePayloadFormat(job)
	fromBytes := SafePayloadFormat(jobBytes)
	fromString := SafePayloadFormat(jobString)

	if fromStruct != fromBytes {
		t.Errorf("QueuePayload and []byte results differ:\n  struct: %s\n  bytes:  %s", fromStruct, fromBytes)
	}
	if fromStruct != fromString {
		t.Errorf("QueuePayload and string results differ:\n  struct: %s\n  string: %s", fromStruct, fromString)
	}
}

func TestSafePayloadFormat_InvalidJSONReturnsEmpty(t *testing.T) {
	if result := SafePayloadFormat([]byte("invalid json")); result != "" {
		t.Errorf("expected empty string for invalid []byte, got %q", result)
	}
	if result := SafePayloadFormat("invalid json"); result != "" {
		t.Errorf("expected empty string for invalid string, got %q", result)
	}
}

func TestSafePayloadFormat_EmptyPayload(t *testing.T) {
	result := SafePayloadFormat(payload.QueuePayload{})
	assertSanitized(t, result)
}

func TestSafePayloadFormat_OriginalNotMutated(t *testing.T) {
	job := baseJob()
	_ = SafePayloadFormat(job)

	if len(job.Metadata) == 0 {
		t.Error("SafePayloadFormat mutated the original job's Metadata")
	}
	for i, step := range job.Steps {
		if len(step.Metadata) == 0 {
			t.Errorf("SafePayloadFormat mutated step[%d].Metadata", i)
		}
	}
}

package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestJobBuilder_DefaultTemplate(t *testing.T) {
	b, err := newJobBuilder("")
	if err != nil {
		t.Fatalf("newJobBuilder: %v", err)
	}

	ctx := EventContext{
		Platform:     "github",
		EventType:    "push",
		Action:       "push",
		RepoFullName: "org/repo",
		Branch:       "main",
		SHA:          "abc123def456abcd",
		ShortSHA:     "abc123de",
		SenderLogin:  "alice",
	}

	payload, err := b.build(ctx)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !json.Valid(payload) {
		t.Errorf("default template produced invalid JSON: %s", payload)
	}
}

func TestJobBuilder_EnvFunction(t *testing.T) {
	t.Setenv("HADES_TEST_TOKEN", "super secret")

	const tmpl = `{"token": {{ env "HADES_TEST_TOKEN" | json }}}`
	b := builderFromString(t, tmpl)

	payload, err := b.build(EventContext{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["token"] != "super secret" {
		t.Errorf("token = %q, want %q", result["token"], "super secret")
	}
}

func TestJobBuilder_JSONEscaping(t *testing.T) {
	const tmpl = `{"msg": {{ .HeadCommitMessage | json }}}`
	b := builderFromString(t, tmpl)

	payload, err := b.build(EventContext{HeadCommitMessage: `say "hello" & goodbye`})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !json.Valid(payload) {
		t.Errorf("payload with special characters is invalid JSON: %s", payload)
	}
	var result map[string]string
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["msg"] != `say "hello" & goodbye` {
		t.Errorf("msg round-trip failed: %q", result["msg"])
	}
}

func TestJobBuilder_InvalidJSONOutput(t *testing.T) {
	// A template where the author forgot | json — produces bare unquoted string.
	const tmpl = `{"name": {{ .RepoFullName }}}`
	b := builderFromString(t, tmpl)

	_, err := b.build(EventContext{RepoFullName: "org/repo"})
	if err == nil {
		t.Error("expected error for invalid JSON output, got nil")
	}
}

func TestJobBuilder_CustomTemplateFile(t *testing.T) {
	const tmpl = `{"platform": {{ .Platform | json }}, "event": {{ .EventType | json }}}`

	f, err := os.CreateTemp(t.TempDir(), "*.json.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(tmpl); err != nil {
		t.Fatal(err)
	}
	f.Close()

	b, err := newJobBuilder(f.Name())
	if err != nil {
		t.Fatalf("newJobBuilder: %v", err)
	}

	payload, err := b.build(EventContext{Platform: "github", EventType: "push"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["platform"] != "github" {
		t.Errorf("platform = %q, want github", result["platform"])
	}
	if result["event"] != "push" {
		t.Errorf("event = %q, want push", result["event"])
	}
}

func TestJobBuilder_MissingTemplateFile(t *testing.T) {
	_, err := newJobBuilder("/nonexistent/path/template.json.tmpl")
	if err == nil {
		t.Error("expected error for missing template file, got nil")
	}
}

// builderFromString writes tmpl to a temp file and returns a jobBuilder for it.
func builderFromString(t *testing.T, tmpl string) *jobBuilder {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.json.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(tmpl); err != nil {
		t.Fatal(err)
	}
	f.Close()
	b, err := newJobBuilder(f.Name())
	if err != nil {
		t.Fatalf("newJobBuilder: %v", err)
	}
	return b
}

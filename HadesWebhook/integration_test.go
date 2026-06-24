package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestFullLoop verifies the complete path from an incoming webhook event through
// template rendering to the payload that actually arrives at HadesAPI.
//
// It uses a template that embeds several EventContext fields so we can assert
// that parsing + rendering produced the right values end-to-end.
func TestFullLoop(t *testing.T) {
	const tmpl = `{
  "name": {{ printf "%s@%s" .RepoFullName .ShortSHA | json }},
  "priority": 2,
  "steps": [
    {
      "id": 1,
      "name": "clone",
      "image": "alpine:latest",
      "metadata": {
        "REPO_URL": {{ .RepoURL     | json }},
        "BRANCH":   {{ .Branch      | json }},
        "SHA":      {{ .SHA         | json }},
        "SENDER":   {{ .SenderLogin | json }},
        "PLATFORM": {{ .Platform    | json }}
      },
      "script": {{ printf "echo %s" .ShortSHA | json }}
    }
  ]
}`

	// Write the template to a temp file (Config takes a path, not a string).
	f, err := os.CreateTemp(t.TempDir(), "*.json.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(tmpl); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Capture whatever payload the handler sends to HadesAPI.
	var receivedBody []byte
	fakeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]string{"job_id": "loop-test-job"})
	}))
	defer fakeAPI.Close()

	cfg := Config{
		HadesAPIURL:     fakeAPI.URL,
		JobTemplatePath: f.Name(),
		AllowedEvents:   "push,pull_request",
	}
	h, err := newHandler(cfg, map[string]PlatformAdapter{
		"github": &GitHubAdapter{},
		"gitlab": &GitLabAdapter{},
	}, parseAllowedEvents(cfg.AllowedEvents))
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook/{platform}", h.handle)

	// Send a realistic GitHub push event (payload defined in github_test.go).
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(ghPushPayload))
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("handler returned %d, want 202; body: %s", w.Code, w.Body.String())
	}

	// Parse the payload that arrived at HadesAPI and check the rendered values.
	type step struct {
		Metadata map[string]string `json:"metadata"`
	}
	type jobPayload struct {
		Name     string `json:"name"`
		Priority int    `json:"priority"`
		Steps    []step `json:"steps"`
	}

	var payload jobPayload
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("unmarshal received body: %v\nraw: %s", err, receivedBody)
	}

	// ghPushPayload has SHA "abc123def456abcd", branch "main", sender "alice".
	if want := "org/myrepo@abc123de"; payload.Name != want {
		t.Errorf("name = %q, want %q", payload.Name, want)
	}
	if payload.Priority != 2 {
		t.Errorf("priority = %d, want 2", payload.Priority)
	}
	if len(payload.Steps) != 1 {
		t.Fatalf("steps count = %d, want 1", len(payload.Steps))
	}

	meta := payload.Steps[0].Metadata
	for key, want := range map[string]string{
		"REPO_URL": "https://github.com/org/myrepo.git",
		"BRANCH":   "main",
		"SHA":      "abc123def456abcd",
		"SENDER":   "alice",
		"PLATFORM": "github",
	} {
		if got := meta[key]; got != want {
			t.Errorf("metadata[%s] = %q, want %q", key, got, want)
		}
	}
}

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildTestMux wires a webhookHandler into a ServeMux the same way main() does.
func buildTestMux(t *testing.T, hadesURL string, adapters map[string]PlatformAdapter, allowedEvents string) *http.ServeMux {
	t.Helper()
	cfg := Config{
		HadesAPIURL:   hadesURL,
		AllowedEvents: allowedEvents,
	}
	h, err := newHandler(cfg, adapters, parseAllowedEvents(cfg.AllowedEvents))
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook/{platform}", h.handle)
	return mux
}

func TestHandler_UnknownPlatform(t *testing.T) {
	mux := buildTestMux(t, "http://unused", map[string]PlatformAdapter{
		"github": &GitHubAdapter{},
	}, "push")

	req := httptest.NewRequest(http.MethodPost, "/webhook/bitbucket", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestHandler_InvalidSignature(t *testing.T) {
	mux := buildTestMux(t, "http://unused", map[string]PlatformAdapter{
		"github": &GitHubAdapter{secret: testGHSecret},
	}, "push")

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(ghPushPayload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=badhash000000000000000000000000000000000000000000000000000000000")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", w.Code)
	}
}

func TestHandler_SkippedEvent(t *testing.T) {
	mux := buildTestMux(t, "http://unused", map[string]PlatformAdapter{
		"github": &GitHubAdapter{},
	}, "push")

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(`{}`))
	req.Header.Set("X-GitHub-Event", "ping")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "skipped") {
		t.Errorf("body = %q, want mention of 'skipped'", w.Body.String())
	}
}

func TestHandler_EventTypeNotAllowed(t *testing.T) {
	// Only push is allowed; pull_request should be dropped.
	mux := buildTestMux(t, "http://unused", map[string]PlatformAdapter{
		"github": &GitHubAdapter{},
	}, "push")

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(ghPRPayload))
	req.Header.Set("X-GitHub-Event", "pull_request")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ALLOWED_EVENTS") {
		t.Errorf("body = %q, expected ALLOWED_EVENTS mention", w.Body.String())
	}
}

func TestHandler_Success(t *testing.T) {
	fakeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/build" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			http.Error(w, "missing Content-Type", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"job_id": "job-abc-123"})
	}))
	defer fakeAPI.Close()

	mux := buildTestMux(t, fakeAPI.URL, map[string]PlatformAdapter{
		"github": &GitHubAdapter{},
	}, "push,pull_request")

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(ghPushPayload))
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("code = %d, want 202; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["job_id"] != "job-abc-123" {
		t.Errorf("job_id = %q, want job-abc-123", resp["job_id"])
	}
}

func TestHandler_HadesAPIError(t *testing.T) {
	fakeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "queue full", http.StatusServiceUnavailable)
	}))
	defer fakeAPI.Close()

	mux := buildTestMux(t, fakeAPI.URL, map[string]PlatformAdapter{
		"github": &GitHubAdapter{},
	}, "push")

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(ghPushPayload))
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", w.Code)
	}
}

func TestHandler_BasicAuth(t *testing.T) {
	var gotAuth string
	fakeAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]string{"job_id": "ok"})
	}))
	defer fakeAPI.Close()

	cfg := Config{
		HadesAPIURL:   fakeAPI.URL,
		HadesAuthKey:  "s3cret",
		AllowedEvents: "push",
	}
	h, err := newHandler(cfg, map[string]PlatformAdapter{
		"github": &GitHubAdapter{},
	}, parseAllowedEvents(cfg.AllowedEvents))
	if err != nil {
		t.Fatalf("newHandler: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook/{platform}", h.handle)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(ghPushPayload))
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("code = %d, want 202", w.Code)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Errorf("Authorization header = %q, want Basic auth", gotAuth)
	}
}

func TestParseAllowedEvents(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"push,pull_request", []string{"push", "pull_request"}},
		{"push", []string{"push"}},
		{"", []string{}},
		{" push , pull_request ", []string{"push", "pull_request"}},
	}

	for _, tc := range tests {
		got := parseAllowedEvents(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("parseAllowedEvents(%q): got %d entries, want %d", tc.input, len(got), len(tc.want))
			continue
		}
		for _, key := range tc.want {
			if !got[key] {
				t.Errorf("parseAllowedEvents(%q): missing %q", tc.input, key)
			}
		}
	}
}

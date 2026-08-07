package dashboard

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	hades "github.com/ls1intum/hades/shared"
	"github.com/ls1intum/hades/shared/buildstatus"
	"github.com/ls1intum/hades/shared/redact"
	"golang.org/x/crypto/bcrypt"
)

func init() { gin.SetMode(gin.TestMode) }

const testPassword = "s3cret-pass"

// testServer builds a Server without NATS. When enabled it has real credentials.
func testServer(t *testing.T, enabled bool) *Server {
	t.Helper()
	cfg := Config{JobRetention: time.Hour}
	if enabled {
		hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Username = "admin"
		cfg.PasswordHash = string(hash)
		cfg.SessionSecret = "0123456789abcdef-secret"
	}
	auth, err := newAuthenticator(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &Server{
		cfg:      cfg,
		redactor: redact.Default(),
		tracker:  newTracker(cfg.JobRetention),
		hub:      newHub(),
		auth:     auth,
		http:     &http.Client{Timeout: 2 * time.Second},
	}
}

func newRouter(s *Server) *gin.Engine {
	r := gin.New()
	s.RegisterRoutes(r)
	return r
}

// login performs a login and returns the session cookie.
func login(t *testing.T, r *gin.Engine) *http.Cookie {
	t.Helper()
	body := `{"username":"admin","password":"` + testPassword + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func TestDisabledDashboardReturns503(t *testing.T) {
	r := newRouter(testServer(t, false))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/jobs", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAuthRequired(t *testing.T) {
	r := newRouter(testServer(t, true))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/jobs", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without cookie, got %d", w.Code)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	r := newRouter(testServer(t, true))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLoginThenSessionAndLogout(t *testing.T) {
	s := testServer(t, true)
	r := newRouter(s)
	cookie := login(t, r)

	// /api/session returns the username with a valid cookie.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "admin") {
		t.Fatalf("session check failed: %d %s", w.Code, w.Body.String())
	}

	// Logout clears the cookie.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout failed: %d", w.Code)
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	s := testServer(t, true)
	s.auth.ttl = -time.Minute // issue already-expired tokens
	token, _, err := s.auth.issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.auth.validate(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestTamperedSessionRejected(t *testing.T) {
	s := testServer(t, true)
	token, _, _ := s.auth.issue("admin")
	if _, err := s.auth.validate(token + "x"); err == nil {
		t.Fatal("expected tampered token to be rejected")
	}
}

func TestLoginLockout(t *testing.T) {
	s := testServer(t, true)
	r := newRouter(s)
	for i := 0; i < maxFailedAttempts; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
	}
	// Next attempt (even correct) is locked out.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"`+testPassword+`"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after lockout, got %d", w.Code)
	}
}

func TestListJobsReflectsTracker(t *testing.T) {
	s := testServer(t, true)
	r := newRouter(s)
	s.tracker.enqueue("job-1", "build", hades.HighPriority)
	s.tracker.observe("job-1", buildstatus.StatusRunning)

	cookie := login(t, r)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list jobs: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Jobs []JobSummary `json:"jobs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Jobs) != 1 || resp.Jobs[0].ID != "job-1" || resp.Jobs[0].Status != "Running" {
		t.Fatalf("unexpected jobs: %+v", resp.Jobs)
	}
	if resp.Jobs[0].Priority != "high" {
		t.Fatalf("expected high priority, got %q", resp.Jobs[0].Priority)
	}
}

func TestJobLogsProxy(t *testing.T) {
	// Stub log manager.
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.HasSuffix(req.URL.Path, "/jobs/job-1/logs") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"logs":[]}`))
	}))
	defer stub.Close()

	s := testServer(t, true)
	s.cfg.LogManagerURL = stub.URL
	r := newRouter(s)
	cookie := login(t, r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/job-1/logs", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "logs") {
		t.Fatalf("logs proxy: %d %s", w.Code, w.Body.String())
	}
}

func TestJobLogsProxyUnavailable(t *testing.T) {
	s := testServer(t, true)
	s.cfg.LogManagerURL = "http://127.0.0.1:0" // unreachable
	r := newRouter(s)
	cookie := login(t, r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/job-1/logs", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when log manager down, got %d", w.Code)
	}
}

// TestStreamSurvivesWriteTimeout proves the SSE handler clears the per-connection
// write deadline: with a short server WriteTimeout the stream still delivers an
// event pushed well after that timeout would have fired.
func TestStreamSurvivesWriteTimeout(t *testing.T) {
	s := testServer(t, true)
	r := newRouter(s)
	cookie := login(t, r)

	srv := httptest.NewUnstartedServer(r)
	srv.Config.WriteTimeout = 200 * time.Millisecond
	srv.Start()
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/stream", nil)
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	readData := func() (string, error) {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			if strings.HasPrefix(line, "data: ") {
				return line, nil
			}
		}
	}

	// Initial primed metrics event.
	if _, err := readData(); err != nil {
		t.Fatalf("initial event: %v", err)
	}

	// Wait past the server WriteTimeout, then push an event.
	time.Sleep(400 * time.Millisecond)
	s.hub.broadcast(event{Type: eventJob, Job: JobSummary{ID: "x", Status: "Queued"}})

	done := make(chan error, 1)
	go func() {
		_, err := readData()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stream severed after WriteTimeout: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for post-timeout event")
	}
}

func TestSummarizeDurations(t *testing.T) {
	d := summarizeDurations([]int64{100, 200, 300, 400, 500})
	if d.Count != 5 || d.AvgMs != 300 {
		t.Fatalf("unexpected durations: %+v", d)
	}
	if summarizeDurations(nil).Count != 0 {
		t.Fatal("expected empty durations for nil input")
	}
}

func TestTrackerSweep(t *testing.T) {
	tr := newTracker(time.Millisecond)
	tr.observe("j", buildstatus.StatusSucceeded)
	// Force updatedAt into the past.
	tr.mu.Lock()
	tr.jobs["j"].updatedAt = time.Now().Add(-time.Hour)
	tr.mu.Unlock()
	tr.sweep()
	if _, ok := tr.get("j"); ok {
		t.Fatal("expected finished job to be swept")
	}
}

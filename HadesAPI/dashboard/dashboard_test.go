package dashboard

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	hades "github.com/hades-scheduler/hades/shared"
	"github.com/hades-scheduler/hades/shared/buildstatus"
	"github.com/hades-scheduler/hades/shared/redact"
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
		cfg.SessionSecret = "0123456789abcdef-0123456789abcdef-secret"
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
	s.tracker.enqueue("job-1", "build", 3, hades.HighPriority)
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

const testJobID = "11111111-2222-3333-4444-555555555555"

func TestJobLogsProxy(t *testing.T) {
	// Stub log manager.
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.HasSuffix(req.URL.Path, "/jobs/"+testJobID+"/logs") {
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
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+testJobID+"/logs", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+testJobID+"/logs", nil)
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

func TestSummarizeDurations_P95NearestRank(t *testing.T) {
	// 10 samples 1..10: nearest-rank p95 = ceil(0.95*10)=10th value = 1000.
	ds := []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	if got := summarizeDurations(ds).P95Ms; got != 1000 {
		t.Fatalf("p95 = %d, want 1000", got)
	}
	// 20 samples: ceil(0.95*20)=19th value.
	twenty := make([]int64, 20)
	for i := range twenty {
		twenty[i] = int64((i + 1) * 10)
	}
	if got := summarizeDurations(twenty).P95Ms; got != 190 {
		t.Fatalf("p95(20) = %d, want 190", got)
	}
}

func TestLockoutMapBounded(t *testing.T) {
	s := testServer(t, true)
	// Far more distinct keys than the cap; the map must not grow unbounded.
	for i := 0; i < maxFailureEntries+500; i++ {
		s.auth.recordFailure("key-" + strconv.Itoa(i))
	}
	s.auth.mu.Lock()
	n := len(s.auth.failures)
	s.auth.mu.Unlock()
	if n > maxFailureEntries {
		t.Fatalf("failures map grew to %d, cap is %d", n, maxFailureEntries)
	}
}

func TestJobLogsRejectsNonUUID(t *testing.T) {
	s := testServer(t, true)
	r := newRouter(s)
	cookie := login(t, r)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/not-a-uuid/logs", nil)
	req.AddCookie(cookie)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-UUID job id, got %d", w.Code)
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

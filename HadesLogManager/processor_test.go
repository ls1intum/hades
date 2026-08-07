package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeResolver is a test double for jobCallbackResolver.
type fakeResolver struct {
	url string
	err error
}

func (f fakeResolver) CallbackURL(ctx context.Context, jobID string) (string, error) {
	return f.url, f.err
}

func testConfig() AggregatorConfig {
	return AggregatorConfig{BatchSize: 100, Retention: time.Hour, MaxJobLogs: 1000}
}

func newTestAggregator(t *testing.T, resolver jobCallbackResolver) buildlogs.LogAggregator {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return NewLogAggregator(ctx, nil, resolver, testConfig())
}

func TestSendJobLogs_ForwardsToResolvedURL(t *testing.T) {
	var hits int32
	var received []buildlogs.Log
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agg := newTestAggregator(t, fakeResolver{url: srv.URL})
	agg.AddLog(buildlogs.Log{
		JobID:       "job-1",
		ContainerID: "c1",
		Logs:        []buildlogs.LogEntry{{Timestamp: time.Now(), Message: "hello", OutputStream: "stdout"}},
	})

	require.NoError(t, agg.SendJobLogs(context.Background(), "job-1"))
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
	require.Len(t, received, 1)
	assert.Equal(t, "job-1", received[0].JobID)
}

func TestSendJobLogs_NoCallbackURL_IsNoop(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agg := newTestAggregator(t, fakeResolver{url: ""})
	agg.AddLog(buildlogs.Log{JobID: "job-1"})

	require.NoError(t, agg.SendJobLogs(context.Background(), "job-1"))
	assert.Equal(t, int32(0), atomic.LoadInt32(&hits), "no request should be sent when no callback URL is resolved")
}

func TestSendJobLogs_InvalidCallbackURL_IsNoop(t *testing.T) {
	agg := newTestAggregator(t, fakeResolver{url: "not-a-valid-url"})
	agg.AddLog(buildlogs.Log{JobID: "job-1"})

	// Invalid URL is skipped, not treated as a delivery failure.
	require.NoError(t, agg.SendJobLogs(context.Background(), "job-1"))
}

func TestSendJobLogs_ResolverError_IsNoop(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	agg := newTestAggregator(t, fakeResolver{url: srv.URL, err: assertErr("kv unavailable")})
	agg.AddLog(buildlogs.Log{JobID: "job-1"})

	require.NoError(t, agg.SendJobLogs(context.Background(), "job-1"))
	assert.Equal(t, int32(0), atomic.LoadInt32(&hits))
}

func TestSendJobLogs_NilResolver_IsNoop(t *testing.T) {
	agg := newTestAggregator(t, nil)
	agg.AddLog(buildlogs.Log{JobID: "job-1"})

	require.NoError(t, agg.SendJobLogs(context.Background(), "job-1"))
}

// assertErr is a tiny error helper to avoid importing errors just for a literal.
type assertErr string

func (e assertErr) Error() string { return string(e) }

package dashboard

import (
	"context"
	"log/slog"
	"sort"
	"time"

	hades "github.com/ls1intum/hades/shared"
)

// throughputWindow is the rolling window used for the jobs/min throughput stat.
const throughputWindow = time.Minute

// Metrics is the aggregate system snapshot shown on the dashboard.
type Metrics struct {
	StatusCounts     map[string]int `json:"statusCounts"`
	QueueDepth       QueueDepth     `json:"queueDepth"`
	Durations        Durations      `json:"durations"`
	ThroughputPerMin int            `json:"throughputPerMin"`
	StreamClients    int            `json:"streamClients"`
	Timestamp        time.Time      `json:"timestamp"`
}

// QueueDepth reports jobs waiting in the priority queues. It is approximate:
// JetStream's WorkQueue retention removes messages on ack, so pending counts can
// momentarily lag under high concurrency.
type QueueDepth struct {
	Total       int64            `json:"total"`
	ByPriority  map[string]int64 `json:"byPriority"`
	Approximate bool             `json:"approximate"`
}

// Durations summarizes completed-job execution times.
type Durations struct {
	AvgMs int64 `json:"avgMs"`
	P95Ms int64 `json:"p95Ms"`
	Count int   `json:"count"`
}

// metrics computes a fresh metrics snapshot.
func (s *Server) metrics(ctx context.Context) *Metrics {
	m := &Metrics{
		StatusCounts:     s.tracker.counts(),
		QueueDepth:       s.queueDepth(ctx),
		Durations:        summarizeDurations(s.tracker.finishedDurations()),
		ThroughputPerMin: s.tracker.finishedSince(throughputWindow),
		StreamClients:    s.hub.clientCount(),
		Timestamp:        time.Now().UTC(),
	}
	return m
}

// queueDepth reads the per-priority pending count from the JetStream consumers.
func (s *Server) queueDepth(ctx context.Context) QueueDepth {
	qd := QueueDepth{ByPriority: make(map[string]int64), Approximate: true}
	if s.js == nil {
		return qd
	}
	for _, p := range hades.Priorities {
		name := "HADES_JOBS_" + string(p)
		cons, err := s.js.Consumer(ctx, jobsStream, name)
		if err != nil {
			slog.Debug("queue depth: consumer unavailable", "consumer", name, "error", err)
			continue
		}
		info, err := cons.Info(ctx)
		if err != nil {
			slog.Debug("queue depth: consumer info failed", "consumer", name, "error", err)
			continue
		}
		pending := int64(info.NumPending)
		qd.ByPriority[string(p)] = pending
		qd.Total += pending
	}
	return qd
}

// summarizeDurations returns average and p95 of the given millisecond durations.
func summarizeDurations(ds []int64) Durations {
	if len(ds) == 0 {
		return Durations{}
	}
	sorted := make([]int64, len(ds))
	copy(sorted, ds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var sum int64
	for _, d := range sorted {
		sum += d
	}
	avg := sum / int64(len(sorted))

	// Nearest-rank p95.
	idx := (95*len(sorted))/100 - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return Durations{AvgMs: avg, P95Ms: sorted[idx], Count: len(sorted)}
}

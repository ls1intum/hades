package dashboard

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hades-scheduler/hades/shared/buildlogs"
)

// event types pushed over the SSE stream.
const (
	eventJob     = "job"
	eventMetrics = "metrics"
	eventLog     = "log"
)

// event is a single server-sent event payload.
type event struct {
	Type    string         `json:"type"`
	Job     JobSummary     `json:"job,omitempty"`
	Metrics *Metrics       `json:"metrics,omitempty"`
	Log     *buildlogs.Log `json:"log,omitempty"`
}

// subscriberBuffer bounds per-client backlog before a slow client is dropped.
const subscriberBuffer = 32

// hub fans events out to all connected SSE clients.
type hub struct {
	mu      sync.Mutex
	clients map[chan event]struct{}
}

func newHub() *hub {
	return &hub{clients: make(map[chan event]struct{})}
}

func (h *hub) add() chan event {
	ch := make(chan event, subscriberBuffer)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *hub) remove(ch chan event) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// broadcast delivers e to every client, dropping the event for any client whose
// buffer is full (a slow client must not block the publisher).
func (h *hub) broadcast(e event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- e:
		default:
		}
	}
}

func (h *hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		delete(h.clients, ch)
		close(ch)
	}
}

func (h *hub) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// handleStream serves the live update feed as Server-Sent Events. The shared
// http.Server runs with WriteTimeout disabled (see main.go), and this handler
// additionally clears any per-connection write deadline so the long-lived
// stream is never severed mid-flight.
func (s *Server) handleStream(c *gin.Context) {
	w := c.Writer
	// Best-effort: remove any write deadline so the stream can stay open.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch := s.hub.add()
	defer s.hub.remove(ch)

	// Prime the client with the current metrics snapshot.
	writeEvent(w, event{Type: eventMetrics, Metrics: s.metrics(c.Request.Context())})
	w.Flush()

	// Heartbeat keeps intermediaries from closing an idle connection.
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			writeEvent(w, e)
			w.Flush()
		case <-heartbeat.C:
			if _, err := w.WriteString(": ping\n\n"); err != nil {
				return
			}
			w.Flush()
		}
	}
}

// writeEvent serializes e as a JSON SSE "data:" frame.
func writeEvent(w gin.ResponseWriter, e event) {
	payload, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = w.WriteString("data: ")
	_, _ = w.Write(payload)
	_, _ = w.WriteString("\n\n")
}

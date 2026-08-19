// Package metrics exposes a Prometheus scrape endpoint over HTTP.
//
// Each Hades service that wants to be scraped starts a metrics server with
// Serve on a dedicated, cluster-internal port (never behind the public
// ingress). The handler serves the process-wide default registry, which
// already includes the Go runtime and process collectors, so goroutine, GC,
// and process metrics are exported without any extra registration. Services
// register their own domain counters on the default registry in their own
// packages.
package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Serve starts an HTTP server exposing the default Prometheus registry at
// /metrics on addr (e.g. ":8082") and blocks until ctx is cancelled, at which
// point it shuts the server down gracefully. A failure to bind or serve is
// returned so callers can treat it as a fatal startup error; a clean shutdown
// returns nil.
func Serve(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		slog.Info("Starting metrics server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
			return
		}
		errChan <- nil
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Metrics server shutdown error", "error", err)
			return err
		}
		slog.Info("Metrics server shutdown complete")
		return nil
	}
}

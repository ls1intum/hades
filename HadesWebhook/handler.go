package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type webhookHandler struct {
	builder  *jobBuilder
	client   *http.Client
	adapters map[string]PlatformAdapter
	allowed  map[string]bool
	endpoint string
	authKey  string
}

func newHandler(cfg Config, adapters map[string]PlatformAdapter, allowed map[string]bool) (*webhookHandler, error) {
	builder, err := newJobBuilder(cfg.JobTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("job builder: %w", err)
	}
	return &webhookHandler{
		builder:  builder,
		client:   &http.Client{Timeout: 30 * time.Second},
		adapters: adapters,
		allowed:  allowed,
		endpoint: cfg.HadesAPIURL + "/build",
		authKey:  cfg.HadesAuthKey,
	}, nil
}

// handle is the single HTTP handler for all platforms, routed via /webhook/{platform}.
func (h *webhookHandler) handle(w http.ResponseWriter, r *http.Request) {
	platform := r.PathValue("platform")
	adapter, ok := h.adapters[platform]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown platform %q", platform), http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	if err := adapter.Validate(r, body); err != nil {
		slog.Warn("Webhook validation failed", "platform", platform, "error", err, "ip", r.RemoteAddr)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	ctx, err := adapter.Parse(r, body)
	if errors.Is(err, ErrEventSkipped) {
		slog.Debug("Event skipped", "platform", platform)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "event skipped")
		return
	}
	if err != nil {
		slog.Error("Failed to parse event", "platform", platform, "error", err)
		http.Error(w, "failed to parse event", http.StatusBadRequest)
		return
	}

	if !h.allowed[ctx.EventType] {
		slog.Info("Event type not in ALLOWED_EVENTS", "type", ctx.EventType, "platform", platform)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "event type %q not in ALLOWED_EVENTS", ctx.EventType)
		return
	}

	payload, err := h.builder.build(ctx)
	if err != nil {
		slog.Error("Failed to render job template", "error", err)
		http.Error(w, "failed to build job payload", http.StatusInternalServerError)
		return
	}

	jobID, err := h.submitJob(payload)
	if err != nil {
		slog.Error("Failed to submit job to HadesAPI", "error", err)
		http.Error(w, "failed to submit job", http.StatusInternalServerError)
		return
	}

	slog.Info("Job submitted",
		"job_id", jobID,
		"platform", platform,
		"event", ctx.EventType,
		"action", ctx.Action,
		"repo", ctx.RepoFullName,
		"branch", ctx.Branch,
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"job_id": jobID})
}

func (h *webhookHandler) submitJob(payload []byte) (string, error) {
	req, err := http.NewRequest(http.MethodPost, h.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if h.authKey != "" {
		req.SetBasicAuth("hades", h.authKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST to HadesAPI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HadesAPI returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse response JSON: %w", err)
	}
	return result["job_id"], nil
}

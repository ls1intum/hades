package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Port            uint   `env:"WEBHOOK_PORT,notEmpty" envDefault:"8083"`
	HadesAPIURL     string `env:"HADES_API_URL,notEmpty" envDefault:"http://localhost:8080"`
	HadesAuthKey    string `env:"HADES_AUTH_KEY"`
	JobTemplatePath string `env:"JOB_TEMPLATE_PATH"`
	// Comma-separated list of normalized event types to forward (e.g. "push,pull_request").
	// Events not in this list are acknowledged but ignored.
	AllowedEvents string `env:"ALLOWED_EVENTS" envDefault:"push,pull_request"`
	// Per-platform webhook secrets.
	// GitHub: HMAC-SHA256 of the request body, sent in X-Hub-Signature-256.
	// GitLab: static token sent in X-Gitlab-Token.
	GitHubSecret string `env:"GITHUB_WEBHOOK_SECRET"`
	GitLabSecret string `env:"GITLAB_WEBHOOK_SECRET"`
}

func main() {
	if os.Getenv("DEBUG") == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Warn("DEBUG MODE ENABLED")
	}

	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		slog.Error("Failed to parse config", "error", err)
		os.Exit(1)
	}

	adapters := map[string]PlatformAdapter{
		"github": &GitHubAdapter{secret: cfg.GitHubSecret},
		"gitlab": &GitLabAdapter{secret: cfg.GitLabSecret},
	}

	h, err := newHandler(cfg, adapters, parseAllowedEvents(cfg.AllowedEvents))
	if err != nil {
		slog.Error("Failed to initialize handler", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	// All platforms share a single route; the {platform} segment selects the adapter.
	mux.HandleFunc("POST /webhook/{platform}", h.handle)
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "pong")
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("Starting HadesWebhook", "port", cfg.Port, "hadesAPI", cfg.HadesAPIURL,
		"platforms", "github, gitlab")

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	slog.Info("Received shutdown signal", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
	slog.Info("HadesWebhook shutdown complete")
}

func parseAllowedEvents(raw string) map[string]bool {
	allowed := make(map[string]bool)
	for _, ev := range strings.Split(raw, ",") {
		if ev = strings.TrimSpace(ev); ev != "" {
			allowed[ev] = true
		}
	}
	return allowed
}

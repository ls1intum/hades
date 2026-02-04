package nats

import (
	"crypto/tls"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// ConnectionConfig holds NATS server connection configuration.
type ConnectionConfig struct {
	URL      string `env:"NATS_URL,notEmpty" envDefault:"nats://localhost:4222"`
	Username string `env:"NATS_USERNAME"`
	Password string `env:"NATS_PASSWORD"`
	TLS      bool   `env:"NATS_TLS_ENABLED" envDefault:"false"`
}

const (
	natsName          = "HadesAPI"
	natsTimeout       = 10 * time.Second
	natsReconnectWait = 5 * time.Second
	natsMaxReconnects = 10
)

// SetupNatsConnection creates a connection to the NATS server with the provided configuration.
// It configures timeouts, reconnection behavior, and optional authentication/TLS.
func SetupNatsConnection(config ConnectionConfig) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name(natsName),
		nats.Timeout(natsTimeout),
		nats.ReconnectWait(natsReconnectWait),
		nats.MaxReconnects(natsMaxReconnects),
	}

	// Add credentials if provided
	if config.Username != "" && config.Password != "" {
		opts = append(opts, nats.UserInfo(config.Username, config.Password))
	}

	// Add TLS if enabled
	if config.TLS {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12, // Ensure TLS 1.2 or higher
		}
		opts = append(opts, nats.Secure(tlsConfig))
	}

	// Connect to NATS
	nc, err := nats.Connect(config.URL, opts...)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		return nil, err
	}

	slog.Info("Connected to NATS server", "url", config.URL)
	return nc, nil
}

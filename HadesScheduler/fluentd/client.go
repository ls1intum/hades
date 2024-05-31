package fluentd

import (
	"net"
	"strconv"

	"github.com/fluent/fluent-logger-golang/fluent"
)

type FluentdOptions struct {
	Addr     string
	MaxRetry uint
}

func GetFluentdClient(opt FluentdOptions) (*fluent.Fluent, error) {
	host, port, err := net.SplitHostPort(opt.Addr)
	if err != nil {
		return nil, err
	}
	portInt, err := strconv.Atoi(port) // Convert port from string to int
	if err != nil {
		return nil, err
	}
	client, err := fluent.New(fluent.Config{
		FluentHost:    host,
		FluentPort:    portInt, // Use the converted port
		FluentNetwork: "tcp",
		MaxRetry:      int(opt.MaxRetry),
	})
	return client, err
}

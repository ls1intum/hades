module github.com/ls1intum/hades/hadesScheduler

go 1.22

replace github.com/ls1intum/hades/shared => ../shared

require (
	github.com/docker/docker v27.1.1+incompatible
	github.com/fluent/fluent-logger-golang v1.9.0
	github.com/hibiken/asynq v0.24.1
	github.com/ls1intum/hades/shared v0.0.0-00010101000000-000000000000
	github.com/samber/slog-fluentd/v2 v2.5.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/caarlos0/env/v11 v11.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/redis/go-redis/v9 v9.4.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/samber/lo v1.44.0 // indirect
	github.com/samber/slog-common v0.17.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/tinylib/msgp v1.1.8 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel v1.24.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.22.0 // indirect
	go.opentelemetry.io/otel/metric v1.24.0 // indirect
	go.opentelemetry.io/otel/sdk v1.22.0 // indirect
	go.opentelemetry.io/otel/trace v1.24.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gotest.tools/v3 v3.5.1 // indirect
)

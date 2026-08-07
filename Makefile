SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO_MODULES := HadesAPI HadesScheduler HadesScheduler/HadesOperator HadesLogManager shared
GO_PATHS   := $(addsuffix /...,$(addprefix ./,$(GO_MODULES)))

GOLANGCI_LINT_VERSION ?= v2.1.0

COMPOSE      ?= docker compose
COMPOSE_FILE ?= compose.yml

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Run (CLI)

.PHONY: run
run: docker-run-nats ## Run api, scheduler, and logmanager locally via go run (Ctrl-C stops all).
	@echo "Starting api, scheduler, logmanager (Ctrl-C to stop all)..."
	@trap 'kill 0' INT TERM EXIT; \
		(cd HadesAPI       && go run .) & \
		(cd HadesScheduler && go run .) & \
		(cd HadesLogManager && go run .) & \
		wait

.PHONY: run-api
run-api: ## Run HadesAPI locally via go run.
	cd HadesAPI && go run .

.PHONY: run-scheduler
run-scheduler: ## Run HadesScheduler locally via go run.
	cd HadesScheduler && go run .

.PHONY: run-logmanager
run-logmanager: ## Run HadesLogManager locally via go run.
	cd HadesLogManager && go run .

.PHONY: run-operator
run-operator: ## Run HadesOperator locally via go run (requires a Kubernetes context).
	cd HadesScheduler/HadesOperator && go run ./cmd

##@ Run (Docker)

.PHONY: docker-run
docker-run: ## Start the full Hades stack via docker compose.
	$(COMPOSE) -f $(COMPOSE_FILE) up -d

.PHONY: docker-run-api
docker-run-api: ## Start only the HadesAPI service via docker compose.
	$(COMPOSE) -f $(COMPOSE_FILE) up -d hadesAPI

.PHONY: docker-run-scheduler
docker-run-scheduler: ## Start only the HadesScheduler service via docker compose.
	$(COMPOSE) -f $(COMPOSE_FILE) up -d hadesScheduler

.PHONY: docker-run-nats
docker-run-nats: ## Start only the NATS service via docker compose.
	$(COMPOSE) -f $(COMPOSE_FILE) up -d nats

.PHONY: docker-stop
docker-stop: ## Stop the local docker compose stack.
	$(COMPOSE) -f $(COMPOSE_FILE) down

.PHONY: docker-logs
docker-logs: ## Tail logs from the local docker compose stack.
	$(COMPOSE) -f $(COMPOSE_FILE) logs -f

##@ Build

.PHONY: build
build: ## Build all Go modules.
	go build $(GO_PATHS)

.PHONY: ui-install
ui-install: ## Install the dashboard SPA dependencies (HadesAPI/web).
	cd HadesAPI/web && npm ci

.PHONY: ui-build
ui-build: ## Build the dashboard SPA into HadesAPI/web/dist (embedded by the API).
	cd HadesAPI/web && npm ci && npm run build

.PHONY: ui-dev
ui-dev: ## Run the dashboard SPA dev server (proxies /api to localhost:8080).
	cd HadesAPI/web && npm run dev

.PHONY: ui-test
ui-test: ## Run the dashboard SPA tests.
	cd HadesAPI/web && npm ci && npm test

.PHONY: docker-build
docker-build: ## Build all Hades container images.
	docker build -t hades-api:dev      -f HadesAPI/Dockerfile .
	docker build -t hades-scheduler:dev -f HadesScheduler/Dockerfile .
	docker build -t hades-operator:dev  -f HadesScheduler/HadesOperator/Dockerfile HadesScheduler/HadesOperator

##@ Quality

.PHONY: test
test: ## Run unit tests across all Go modules.
	go test $(GO_PATHS)

.PHONY: test-race
test-race: ## Run unit tests with the race detector.
	go test -race $(GO_PATHS)

.PHONY: cover
cover: ## Run tests and open the coverage report for HadesAPI.
	cd HadesAPI && go test -coverprofile=cover.out ./... && go tool cover -html=cover.out

.PHONY: test-operator
test-operator: ## Run HadesOperator unit tests (uses envtest).
	$(MAKE) -C HadesScheduler/HadesOperator test

.PHONY: test-operator-e2e
test-operator-e2e: ## Run HadesOperator e2e tests (requires Kind).
	$(MAKE) -C HadesScheduler/HadesOperator test-e2e

.PHONY: fmt
fmt: ## Format all Go code.
	gofmt -s -w $(GO_MODULES)

.PHONY: lint
lint: ## Run go vet and golangci-lint across all Go modules.
	go vet $(GO_PATHS)
	@GOLANGCI_LINT=$$(command -v golangci-lint || echo "$$(go env GOPATH)/bin/golangci-lint"); \
	if ! [ -x "$$GOLANGCI_LINT" ]; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
		GOLANGCI_LINT="$$(go env GOPATH)/bin/golangci-lint"; \
	fi; \
	for m in $(GO_MODULES); do \
		echo "==> golangci-lint $$m"; \
		(cd $$m && "$$GOLANGCI_LINT" run --config $(CURDIR)/.golangci.yml ./...); \
	done

.PHONY: vuln
vuln: ## Run govulncheck across all Go modules.
	@command -v govulncheck >/dev/null 2>&1 || { \
		echo "Installing govulncheck..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	}
	govulncheck $(GO_PATHS)

##@ Dependencies

.PHONY: deps-check
deps-check: ## List outdated direct dependencies across the workspace.
	@for m in $(GO_MODULES); do \
		echo "==> $$m"; \
		(cd $$m && go list -u -m -f '{{if and .Update (not .Indirect)}}{{.Path}}: {{.Version}} -> {{.Update.Version}}{{end}}' all 2>/dev/null | grep -v '^$$') || true; \
	done

.PHONY: deps-update
deps-update: ## Bump direct dependencies in every module and tidy.
	@for m in $(GO_MODULES); do \
		echo "==> $$m"; \
		(cd $$m && go get -u ./... && go mod tidy); \
	done
	go work sync

.PHONY: deps-tidy
deps-tidy: ## Run go mod tidy in every module.
	@for m in $(GO_MODULES); do \
		echo "==> $$m"; \
		(cd $$m && go mod tidy); \
	done
	go work sync

.PHONY: helm-deps
helm-deps: ## Refresh Helm chart subchart lock file.
	helm dependency update ./helm/hades

##@ CI

.PHONY: ci
ci: lint test ## Run lint and test (mirrors CI).

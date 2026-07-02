BINARY := agent-mongo
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test test-integration lint dev tidy

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agent-mongo

test:
	go test ./... -count=1

test-integration:
	go test ./internal/integration/ -count=1 -tags=integration

lint:
	golangci-lint run ./...

dev:
	go run ./cmd/agent-mongo $(ARGS)

tidy:
	go mod tidy

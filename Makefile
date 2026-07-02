BINARY := agent-mongo
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test test-integration lint dev tidy

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agent-mongo

test:
	go test ./... -count=1

MONGO_TEST_PORT ?= 27099

test-integration:
	@docker inspect agent-mongo-test >/dev/null 2>&1 || \
		docker run -d --rm --name agent-mongo-test -p 127.0.0.1:$(MONGO_TEST_PORT):27017 mongo:8
	AGENT_MONGO_TEST_URI=mongodb://localhost:$(MONGO_TEST_PORT) \
		go test ./internal/integration/ -count=1 -tags=integration -v

test-integration-down:
	docker stop agent-mongo-test

lint:
	golangci-lint run ./...

dev:
	go run ./cmd/agent-mongo $(ARGS)

tidy:
	go mod tidy

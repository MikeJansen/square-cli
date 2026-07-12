BINARY      := square-cli
PKG         := github.com/MikeJansen/square-cli
CMD         := ./cmd/square-cli
BIN_DIR     := bin
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X $(PKG)/internal/cli.version=$(VERSION) \
	-X $(PKG)/internal/cli.commit=$(COMMIT) \
	-X $(PKG)/internal/cli.date=$(DATE)

.PHONY: all build install generate test check fmt fmt-check vet lint tidy clean run

all: build

## build: compile the CLI into ./bin
build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)

## install: install the CLI into GOBIN
install:
	go install -ldflags "$(LDFLAGS)" $(CMD)

## generate: regenerate models and commands from the OpenAPI spec
generate:
	go run ./internal/codegen
	gofmt -w internal/commands pkg/square

## test: run the test suite
test:
	go test ./...

## check: what CI runs — formatting, vet, and tests
check: fmt-check vet test

## fmt: format all Go code
fmt:
	gofmt -w .

## fmt-check: fail if any Go code is not gofmt-clean
fmt-check:
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then echo "Not gofmt-clean:"; echo "$$out"; exit 1; fi

## vet: run go vet
vet:
	go vet ./...

## tidy: tidy go.mod/go.sum
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR) dist

## run: build and run (pass ARGS="...")
run: build
	$(BIN_DIR)/$(BINARY) $(ARGS)

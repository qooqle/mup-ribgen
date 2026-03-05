# Makefile for mup-ribgen (req 9.3)
# Usage: make [target]

MODULE  := github.com/qooqle/mup-ribgen
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

BIN_DIR   := bin
DIST_DIR  := dist
BINARY    := mup-ribgen
DSLC      := dslc

GOOS_LIST   := linux darwin windows
GOARCH_LIST := amd64 arm64

.DEFAULT_GOAL := build

# ─── Build ────────────────────────────────────────────────────────────────────

.PHONY: build
build: ## Build binaries for the host OS/arch
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) ./cmd/$(BINARY)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(DSLC)   ./cmd/$(DSLC)
	@echo "Built: $(BIN_DIR)/$(BINARY)  $(BIN_DIR)/$(DSLC)"

# ─── Test ─────────────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all tests
	go test ./...

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose output
	go test -v ./...

.PHONY: bench
bench: ## Run benchmark tests
	go test -bench=. -benchmem ./pkg/pfcp/ ./pkg/ir/

.PHONY: cover
cover: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ─── Lint ─────────────────────────────────────────────────────────────────────

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: vet ## Run go vet (install staticcheck separately if needed)
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed; run: go install honnef.co/go/tools/cmd/staticcheck@latest"

# ─── Cross-compile ────────────────────────────────────────────────────────────

.PHONY: cross
cross: ## Cross-compile for Linux and macOS (amd64, arm64)
	@mkdir -p $(BIN_DIR)
	@for os in linux darwin; do \
	  for arch in amd64 arm64; do \
	    ext=""; \
	    outdir="$(BIN_DIR)/$$os-$$arch"; \
	    mkdir -p $$outdir; \
	    GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) \
	      -o $$outdir/$(BINARY)$$ext ./cmd/$(BINARY) && \
	    GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) \
	      -o $$outdir/$(DSLC)$$ext   ./cmd/$(DSLC) && \
	    echo "  $$os/$$arch: OK"; \
	  done; \
	done
	@echo "Cross-compile complete: $(BIN_DIR)/"

.PHONY: cross-windows
cross-windows: ## Cross-compile for Windows (amd64)
	@mkdir -p $(BIN_DIR)/windows-amd64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) \
	  -o $(BIN_DIR)/windows-amd64/$(BINARY).exe ./cmd/$(BINARY)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) \
	  -o $(BIN_DIR)/windows-amd64/$(DSLC).exe   ./cmd/$(DSLC)
	@echo "Windows/amd64: OK"

# ─── Distribution ─────────────────────────────────────────────────────────────

.PHONY: dist
dist: cross ## Create distribution archives (req 9.4)
	@mkdir -p $(DIST_DIR)
	@for os in linux darwin; do \
	  for arch in amd64 arm64; do \
	    name="$(BINARY)-$(VERSION)-$$os-$$arch"; \
	    srcdir="$(BIN_DIR)/$$os-$$arch"; \
	    tarball="$(DIST_DIR)/$$name.tar.gz"; \
	    tar -czf $$tarball \
	      -C $$srcdir $(BINARY) $(DSLC) \
	      -C ../../ static_context.example.json \
	      dsl/ \
	      2>/dev/null || \
	    tar -czf $$tarball -C $$srcdir $(BINARY) $(DSLC); \
	    echo "  $$tarball"; \
	  done; \
	done
	@echo "Distribution archives: $(DIST_DIR)/"

# ─── Code generation ──────────────────────────────────────────────────────────

.PHONY: generate
generate: ## Compile all DSL files to Go dialect transformers
	@for f in dsl/*.dsl; do \
	  base=$$(basename $$f .dsl); \
	  out="pkg/dialect/$${base}_transformer.go"; \
	  echo "  $$f -> $$out"; \
	  go run ./cmd/dslc compile -pkg dialect -out $$out $$f; \
	done

# ─── Clean ────────────────────────────────────────────────────────────────────

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html

# ─── Help ─────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

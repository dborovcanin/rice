# Rice build targets. Binaries land in build/.

BINARY      := rice
BUILD_DIR   := build
CMD         := ./cmd/rice

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS        ?= $(shell go env GOOS)
GOARCH      ?= $(shell go env GOARCH)

# Release builds are pure Go, stripped and reproducible: a single binary that
# carries its own templates and themes.
LDFLAGS     := -s -w
GOFLAGS     := -trimpath

PREFIX      ?= $(HOME)/.local
TESTROOT    ?= /tmp/rice-test

.PHONY: all
all: build

## build: compile the CLI into build/
.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD)
	@echo "built $(BUILD_DIR)/$(BINARY) ($(VERSION), $(GOOS)/$(GOARCH))"

## release: cross-compile linux/amd64 and linux/arm64 into build/
.PHONY: release
release:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-$(VERSION)-linux-amd64 $(CMD)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-$(VERSION)-linux-arm64 $(CMD)
	@cd $(BUILD_DIR) && sha256sum $(BINARY)-$(VERSION)-linux-* > SHA256SUMS
	@echo "release artifacts in $(BUILD_DIR)/"

## install: install the binary into $(PREFIX)/bin
.PHONY: install
install: build
	@mkdir -p $(PREFIX)/bin
	install -m 0755 $(BUILD_DIR)/$(BINARY) $(PREFIX)/bin/$(BINARY)
	@echo "installed $(PREFIX)/bin/$(BINARY)"

## uninstall: remove the installed binary
.PHONY: uninstall
uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)

## test: run the full test suite
.PHONY: test
test:
	go test ./...

## test-race: run the tests with the race detector
.PHONY: test-race
test-race:
	go test -race ./...

## cover: write a coverage profile to build/coverage.out
.PHONY: cover
cover:
	@mkdir -p $(BUILD_DIR)
	go test -coverprofile=$(BUILD_DIR)/coverage.out ./...
	go tool cover -func=$(BUILD_DIR)/coverage.out | tail -1

## golden: accept intentional template changes
.PHONY: golden
golden:
	go test . -update
	@echo "golden files updated; review the diff before committing"

## fmt: format the tree
.PHONY: fmt
fmt:
	gofmt -w .

## vet: run go vet
.PHONY: vet
vet:
	go vet ./...

## check: fmt verification, vet and tests
.PHONY: check
check: vet test
	@test -z "$$(gofmt -l .)" || { echo "unformatted files:"; gofmt -l .; exit 1; }

## tidy: prune go.mod and go.sum
.PHONY: tidy
tidy:
	go mod tidy

## demo: build a generation in $(TESTROOT) without touching the real config
.PHONY: demo
demo: build
	rm -rf $(TESTROOT)
	$(BUILD_DIR)/$(BINARY) --root $(TESTROOT) init
	$(BUILD_DIR)/$(BINARY) --root $(TESTROOT) apply
	@echo
	@find $(TESTROOT)/current/ -type f | sort

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

## help: list the available targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /' | sort

GO ?= go
BIN ?= dist/exodock

.DEFAULT_GOAL := help
.PHONY: help run demo test integration check format-check links build clean

help:
	@printf '%s\n' \
	  'run          Run Exodock' \
	  'demo         Run with sample data (no Docker daemon required)' \
	  'test         Run Go tests with the race detector and coverage' \
	  'integration  Run opt-in tests against a local Docker Engine' \
	  'check        Check docs links, formatting, vet, and tests' \
	  'links        Check local links in Markdown docs' \
	  'build        Build the CLI to $(BIN)' \
	  'clean        Remove local build output'

run:
	$(GO) run ./cmd/exodock $(ARGS)

demo:
	$(GO) run ./cmd/exodock --demo $(ARGS)

test:
	$(GO) test -race -cover ./...

integration:
	EXODOCK_INTEGRATION=1 $(GO) test -race -cover ./...

check: format-check links
	$(GO) vet ./...
	$(GO) test -race -cover ./...

links:
	python3 scripts/check-doc-links.py

format-check:
	@test -z "$(shell gofmt -l .)"

build:
	mkdir -p $(dir $(BIN))
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '-s -w' -o $(BIN) ./cmd/exodock

clean:
	rm -rf dist

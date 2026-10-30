BIN     := lazygram
PREFIX  ?= $(HOME)/.local
STATE   ?= $(CURDIR)/.state
GOFILES  = $(shell find . -name '*.go' -not -path './.state/*')

.PHONY: help build test lint check run smoke fmt install uninstall clean
.DEFAULT_GOAL := help

help: ## list the targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | sed -E 's/^([a-z-]+):.*## (.*)/  \1\t\2/'

build: ## build ./lazygram
	go build -o $(BIN) ./cmd/lazygram

test: ## run all tests
	go test ./...

lint: ## gofmt check, go vet, golangci-lint (staticcheck, errcheck, unused)
	@out="$$(gofmt -l $(GOFILES))"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; else echo "golangci-lint not installed, skipped"; fi

check: lint test ## lint and test; must pass before a commit

run: build ## build and start the TUI (demo provider unless configured)
	LAZYGRAM_DATA_DIR=$(STATE) ./$(BIN)

smoke: build ## non-interactive: sync the demo once, print one screen
	LAZYGRAM_DATA_DIR=$(STATE) ./$(BIN) -smoke

fmt: ## format in place
	gofmt -w $(GOFILES)

install: build ## install to $(PREFIX)/bin
	install -d $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/$(BIN)

uninstall: ## remove from $(PREFIX)/bin
	rm -f $(PREFIX)/bin/$(BIN)

clean: ## remove build output and local demo state
	rm -rf $(BIN) .state

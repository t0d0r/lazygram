BIN     := lazygram
PREFIX  ?= $(HOME)/.local
STATE   ?= $(CURDIR)/.state
GOFILES  = $(shell find . -name '*.go' -not -path './.state/*')

# Optional: embed your own Telegram application registration so a personal
# build never asks for it. Never commit these values.
TG_API_ID   ?=
TG_API_HASH ?=
LDFLAGS      = -X main.buildAppID=$(TG_API_ID) -X main.buildAppHash=$(TG_API_HASH)

.PHONY: help build test lint check run demo login smoke fmt install uninstall clean
.DEFAULT_GOAL := help

help: ## list the targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | sed -E 's/^([a-z-]+):.*## (.*)/  \1\t\2/'

build: ## build ./lazygram
	go build -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/lazygram

test: ## run all tests
	go test ./...

lint: ## gofmt check, go vet, golangci-lint (staticcheck, errcheck, unused)
	@out="$$(gofmt -l $(GOFILES))"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; else echo "golangci-lint not installed, skipped"; fi

check: lint test ## lint and test; must pass before a commit

run: build ## build and start with the linked Telegram account (state in .state/)
	./$(BIN) -data $(STATE)

demo: build ## build and start with invented offline data
	./$(BIN) -data $(STATE) demo

login: build ## link a Telegram account by QR code (state in .state/)
	./$(BIN) -data $(STATE) login

smoke: build ## non-interactive: sync the demo once, print one screen
	./$(BIN) -data $(STATE) -smoke demo

fmt: ## format in place
	gofmt -w $(GOFILES)

install: build ## install to $(PREFIX)/bin
	install -d $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/$(BIN)

uninstall: ## remove from $(PREFIX)/bin
	rm -f $(PREFIX)/bin/$(BIN)

clean: ## remove build output and local demo state
	rm -rf $(BIN) .state

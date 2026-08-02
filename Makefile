APP_NAME    := yt-downloader
BIN_DIR     := build/bin

# Single version source: VERSION file, overridable by env (CI/release tags).
VERSION_FILE := VERSION
VERSION      ?= $(shell tr -d ' \n' < $(VERSION_FILE) 2>/dev/null || echo "dev")
BUILD_DATE   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS      := -s -w -X main.version=$(VERSION)

# Wails native builds need CGO for platform webview/toolchain dependencies.
export CGO_ENABLED := 1

.DEFAULT_GOAL := help

.PHONY: help dev run generate deps tidy \
	version sync-version check-ytdlp-pin \
	build build-linux build-windows build-mac build-mac-amd64 build-mac-arm64 build-all \
	sign-mac sign-windows \
	test test-verbose test-cover race fmt lint vet govulncheck npm-audit security check \
	frontend-install frontend-build frontend-typecheck \
	clean clean-all debug

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

version: ## Print resolved build version
	@echo $(VERSION)

sync-version: ## Write VERSION into wails.json productVersion
	@test -f "$(VERSION_FILE)" || { echo "missing $(VERSION_FILE)"; exit 1; }
	@command -v python3 >/dev/null 2>&1 || { echo "python3 required for sync-version"; exit 1; }
	@VERSION="$(VERSION)" python3 scripts/sync_version.py

check-ytdlp-pin: ## Reject non-reproducible local ytdlp replacements
	@if grep -qE 'replace[[:space:]]+github.com/ytget/ytdlp/v2[[:space:]]*=>[[:space:]]*\.\./\.\./ytdlp-src' go.mod; then \
		echo "error: go.mod uses a local ytdlp replacement" >&2; exit 1; \
	fi

dev: ## Run Wails development mode
	wails dev

run: dev ## Alias for dev

debug: ## Run Wails dev and tee logs
	@set -o pipefail; wails dev 2>&1 | tee debug.log

generate: ## Regenerate Wails TypeScript bindings
	wails generate module

deps: ## Install Go and frontend dependencies
	go mod download
	go mod tidy
	cd frontend && npm install

tidy: ## Tidy Go modules
	go mod tidy

frontend-install: ## Install frontend dependencies
	cd frontend && npm install

frontend-typecheck: ## TypeScript check (no emit)
	cd frontend && npx tsc --noEmit

frontend-build: ## Build frontend only
	cd frontend && npm run build

build: sync-version ## Build native app for current platform
	@echo "Building $(APP_NAME) $(VERSION)..."
	wails build -ldflags "$(LDFLAGS)"

build-linux: sync-version ## Build Linux amd64 binary
	@echo "Building Linux amd64 $(VERSION)..."
	wails build -platform linux/amd64 -ldflags "$(LDFLAGS)"

build-windows: sync-version ## Build Windows amd64 binary
	@echo "Building Windows amd64 $(VERSION)..."
	wails build -platform windows/amd64 -ldflags "$(LDFLAGS)"

build-mac: sync-version ## Build macOS universal binary
	@echo "Building macOS universal $(VERSION)..."
	wails build -platform darwin/universal -ldflags "$(LDFLAGS)"

build-mac-amd64: sync-version ## Build macOS amd64 binary
	@echo "Building macOS amd64 $(VERSION)..."
	wails build -platform darwin/amd64 -ldflags "$(LDFLAGS)"

build-mac-arm64: sync-version ## Build macOS arm64 binary
	@echo "Building macOS arm64 $(VERSION)..."
	wails build -platform darwin/arm64 -ldflags "$(LDFLAGS)"

build-all: build-linux build-windows build-mac ## Build all release targets

sign-mac: ## codesign/notarize .app in build/bin (needs Apple identity/creds)
	bash scripts/sign_macos.sh

sign-windows: ## signtool sign .exe in build/bin (needs WINDOWS_CERTIFICATE_PFX_PATH)
	bash scripts/sign_windows.sh

test: ## Run Go tests
	go test ./... -count=1

test-verbose: ## Run Go tests with verbose output
	go test ./... -v -count=1

test-cover: ## Run Go tests with coverage report
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

race: ## Run Go tests with the race detector
	go test -race ./...

fmt: ## Format Go code
	gofmt -s -w .

lint: vet ## Alias for vet

vet: ## Run go vet
	go vet ./...

govulncheck: ## Run govulncheck on module packages
	# Pin toolchain to the module Go version so govulncheck can load packages.
	GOTOOLCHAIN=go1.26.5 go run golang.org/x/vuln/cmd/govulncheck@latest ./...

npm-audit: ## npm audit fails on high+ findings
	cd frontend && npm audit --audit-level=high

security: govulncheck npm-audit ## Dependency vulnerability scans

check: fmt vet test frontend-typecheck ## Format, vet, test backend + frontend typecheck

clean: ## Remove generated build artifacts
	rm -rf "$(BIN_DIR)"/*
	rm -rf frontend/dist
	rm -f coverage.out coverage.html debug.log
	@echo "Cleaned"

clean-all: clean ## Deep clean dependencies and caches
	rm -rf frontend/node_modules
	go clean -cache
	@echo "Deep cleaned"

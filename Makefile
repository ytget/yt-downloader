SHELL:=bash

.DEFAULT_GOAL := help

# App metadata
APP_ID ?= com.github.ytget.ytdownloader
MAIN_DIR ?= cmd/yt-downloader
BINARY_NAME ?= yt-downloader

# Install locations
GOBIN_PATH := $(shell go env GOBIN)
GOPATH_PATH := $(shell go env GOPATH)
BIN_DIR := $(if $(GOBIN_PATH),$(GOBIN_PATH),$(GOPATH_PATH)/bin)

.PHONY: help
help: ## Available commands
	@clear
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[0;33m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""

##@ Development

.PHONY: run
run: check-deps ## Run application
	go run cmd/yt-downloader/main.go

.PHONY: debug
debug: check-deps ## Run app and tee logs to debug.log
	@echo "Running with logs -> debug.log"
	@set -o pipefail; go run cmd/yt-downloader/main.go 2>&1 | tee debug.log

.PHONY: check-deps
check-deps: ## Check that all required dependencies are available
	@echo "Checking dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "Error: Go is not installed or not in PATH"; exit 1; }
	@if [ -f "./bin/yt-dlp" ]; then \
		echo "Using local yt-dlp: ./bin/yt-dlp"; \
	elif command -v yt-dlp >/dev/null 2>&1; then \
		echo "Using system yt-dlp: $(shell command -v yt-dlp)"; \
	else \
		echo "Error: yt-dlp not found. Run 'make deps' to download it locally."; \
		exit 1; \
	fi
	@echo "Dependencies OK"

.PHONY: test
test: ## Run tests
	go test -v ./...

.PHONY: lint
lint: ## Run linter (golangci-lint)
	golangci-lint run ./...

.PHONY: format
format: ## Format code
	go install golang.org/x/tools/cmd/goimports@latest
	goimports -l -w .

.PHONY: docker-run
docker-run: ## Run application in docker with health check
	@echo "Starting application in Docker..."
	@docker-compose up -d --build
	@echo "Waiting for services to be ready..."
	@timeout 30s sh -c 'until docker-compose ps | grep -q "Up"; do sleep 1; done' || echo "Warning: Services may still be starting"
	@echo "Application started. Use 'make docker-logs' to view logs"

.PHONY: docker-stop
docker-stop: ## Stop application in docker
	@echo "Stopping application in Docker..."
	@docker-compose down
	@echo "Application stopped"

.PHONY: docker-logs
docker-logs: ## View docker application logs
	@docker-compose logs -f

.PHONY: docker-clean
docker-clean: ## Clean up docker containers and images
	@echo "Cleaning up Docker resources..."
	@docker-compose down --rmi all --volumes --remove-orphans
	@echo "Docker cleanup completed"

.PHONY: docker-status
docker-status: ## Check docker application status
	@docker-compose ps

##@ Tools

.PHONY: install-fyne-cli
install-fyne-cli: ## Install Fyne CLI (fyne)
	go install fyne.io/tools/cmd/fyne@latest

.PHONY: install-fyne-cross
install-fyne-cross: ## Install fyne-cross (multi-platform build helper)
	go install github.com/fyne-io/fyne-cross@latest

.PHONY: bundle-resources
bundle-resources: install-fyne-cli ## Bundle resources (images, fonts) into Go code
	@echo "Bundling resources..."
	@if [ -f "yt-downloader.png" ]; then \
		echo "Note: Using dynamic resource loading instead of bundling large PNG"; \
		echo "Logo will be loaded from file at runtime"; \
	else \
		echo "Warning: yt-downloader.png not found in current directory"; \
	fi

.PHONY: bundle-resources-optimized
bundle-resources-optimized: install-fyne-cli ## Bundle optimized resources (requires optimized images)
	@echo "Bundling optimized resources..."
	@if [ -f "yt-downloader-optimized.png" ]; then \
		fyne bundle -o internal/ui/bundled.go yt-downloader-optimized.png; \
		echo "Optimized resources bundled to internal/ui/bundled.go"; \
	else \
		echo "Note: yt-downloader-optimized.png not found. Use bundle-resources for dynamic loading."; \
	fi

##@ Packaging (local host)

.PHONY: package-linux
package-linux: install-fyne-cli ## Package app for Linux (requires Linux host)
	cd $(MAIN_DIR) && fyne package -os linux

.PHONY: package-windows
package-windows: install-fyne-cli ## Package app for Windows (requires Windows host)
	cd $(MAIN_DIR) && fyne package -os windows

.PHONY: package-android
package-android: install-fyne-cli ## Package app for Android (requires Android SDK/NDK)
	cd $(MAIN_DIR) && fyne package -os android

.PHONY: package-ios
package-ios: install-fyne-cli ## Package app for iOS (requires Xcode/toolchain)
	cd $(MAIN_DIR) && fyne package -os ios

.PHONY: package-ios-simulator
package-ios-simulator: install-fyne-cli ## Package app for iOS Simulator (requires Xcode)
	cd $(MAIN_DIR) && fyne package -os iossimulator

.PHONY: package-desktop
package-desktop: package-linux package-windows ## Package desktop platforms (run on respective hosts)

.PHONY: package-mobile
package-mobile: package-android package-ios ## Package mobile platforms

##@ Release

.PHONY: release-linux
release-linux: install-fyne-cli ## Create release package for Linux
	cd $(MAIN_DIR) && fyne release -os linux

.PHONY: release-windows
release-windows: install-fyne-cli ## Create release package for Windows
	cd $(MAIN_DIR) && fyne release -os windows

.PHONY: release-android
release-android: install-fyne-cli ## Create release package for Android
	cd $(MAIN_DIR) && fyne release -os android

.PHONY: release-ios
release-ios: install-fyne-cli ## Create release package for iOS
	cd $(MAIN_DIR) && fyne release -os ios

.PHONY: release-all
release-all: release-linux release-windows release-android release-ios ## Create release packages for all platforms

.PHONY: package-all
package-all: package-desktop package-mobile ## Package all platforms

##@ Cross-platform (fyne-cross + Docker)

.PHONY: cross-linux
cross-linux: install-fyne-cross ## Cross-build Linux (all arches)
	fyne-cross linux -arch=* -output $(BINARY_NAME) ./$(MAIN_DIR)

.PHONY: cross-windows
cross-windows: install-fyne-cross ## Cross-build Windows (all arches)
	fyne-cross windows -arch=* -output $(BINARY_NAME) ./$(MAIN_DIR)

.PHONY: cross-android
cross-android: install-fyne-cross ## Cross-build Android (all ABIs)
	fyne-cross android -arch=* -output $(BINARY_NAME) ./$(MAIN_DIR)

.PHONY: cross-ios
cross-ios: install-fyne-cross ## Cross-build iOS
	fyne-cross ios -arch=* -output $(BINARY_NAME) ./$(MAIN_DIR)

# Specific arch shortcuts
.PHONY: cross-linux-amd64
cross-linux-amd64: install-fyne-cross ## Cross-build Linux amd64
	fyne-cross linux -arch=amd64 -output $(BINARY_NAME) ./$(MAIN_DIR)

.PHONY: cross-linux-386
cross-linux-386: install-fyne-cross ## Cross-build Linux 386 (32-bit)
	fyne-cross linux -arch=386 -output $(BINARY_NAME) ./$(MAIN_DIR)

.PHONY: cross-linux-arm64
cross-linux-arm64: install-fyne-cross ## Cross-build Linux arm64
	fyne-cross linux -arch=arm64 -output $(BINARY_NAME) ./$(MAIN_DIR)

.PHONY: cross-windows-amd64
cross-windows-amd64: install-fyne-cross ## Cross-build Windows amd64
	fyne-cross windows -arch=amd64 -output $(BINARY_NAME) ./$(MAIN_DIR)

.PHONY: cross-windows-386
cross-windows-386: install-fyne-cross ## Cross-build Windows 386 (32-bit)
	fyne-cross windows -arch=386 -output $(BINARY_NAME) ./$(MAIN_DIR)

.PHONY: cross-windows-arm64
cross-windows-arm64: install-fyne-cross ## Cross-build Windows arm64
	fyne-cross windows -arch=arm64 -output $(BINARY_NAME) ./$(MAIN_DIR)

.PHONY: cross-all-desktop
cross-all-desktop: cross-linux cross-windows ## Cross-build all desktop targets

.PHONY: cross-all-mobile
cross-all-mobile: cross-android cross-ios ## Cross-build all mobile targets

.PHONY: cross-all
cross-all: cross-all-desktop cross-all-mobile ## Cross-build desktop + mobile

##@ Dependencies

.PHONY: deps-yt-dlp
deps-yt-dlp: ## Download yt-dlp locally to ./bin
	@mkdir -p bin
	@case "$$(${SHELL} -c 'uname -s')" in \
	  Darwin|Linux) URL=https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp ;; \
	  MINGW*|MSYS*|CYGWIN*) URL=https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe ;; \
	  *) echo "Unsupported OS"; exit 1 ;; \
	esac; \
	curl -fsSL $$URL -o bin/yt-dlp && chmod +x bin/yt-dlp && echo "yt-dlp -> bin/yt-dlp"

.PHONY: deps
deps: deps-yt-dlp ## Download dependencies
	go mod download
	go mod tidy

.PHONY: deps-update
deps-update: ## Update dependencies
	go get -u ./...
	go mod tidy

##@ Build

.PHONY: build
build: ## Build binary
	go build -o bin/yt-downloader cmd/yt-downloader/main.go

.PHONY: install
install: ## Install binary to $$GOBIN or $$GOPATH/bin
	@echo "Installing to $(BIN_DIR)"
	@mkdir -p "$(BIN_DIR)"
	go build -o "$(BIN_DIR)/$(BINARY_NAME)" cmd/yt-downloader/main.go
	@echo "Installed: $(BIN_DIR)/$(BINARY_NAME)"

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/

##@ Aliases

.PHONY: r
r: ## Run app
	@make run

.PHONY: t
t: ## Run tests
	@make test

.PHONY: l
l: ## Run linter (golangci-lint)
	@make lint

.PHONY: f
f: ## Format code
	@make format

.PHONY: dr
dr: ## Run app in docker
	@make docker-run

.PHONY: ds
ds: ## Stop app in docker
	@make docker-stop

.PHONY: dl
dl: ## View docker logs
	@make docker-logs

.PHONY: dc
dc: ## Clean docker resources
	@make docker-clean

.PHONY: dst
dst: ## Check docker status
	@make docker-status

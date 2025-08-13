SHELL:=bash

.DEFAULT_GOAL := help

# App metadata
APP_ID ?= com.github.romanitalian.ytdownloader
MAIN_DIR ?= cmd/yt-downloader
BINARY_NAME ?= yt-downloader

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
docker-run: ## Run application in docker
	docker-compose up -d --build

.PHONY: docker-stop
docker-stop: ## Stop application in docker
	docker-compose down

##@ Tools

.PHONY: install-fyne-cli
install-fyne-cli: ## Install Fyne CLI (fyne)
	go install fyne.io/tools/cmd/fyne@latest

.PHONY: install-fyne-cross
install-fyne-cross: ## Install fyne-cross (multi-platform build helper)
	go install github.com/fyne-io/fyne-cross@latest

##@ Packaging (local host)

.PHONY: package-linux
package-linux: install-fyne-cli ## Package app for Linux (requires Linux host)
	cd $(MAIN_DIR) && fyne package -os linux

.PHONY: package-windows
package-windows: install-fyne-cli ## Package app for Windows (requires Windows host)
	cd $(MAIN_DIR) && fyne package -os windows

.PHONY: package-android
package-android: install-fyne-cli ## Package app for Android (requires Android SDK/NDK)
	cd $(MAIN_DIR) && fyne package -os android -app-id $(APP_ID)

.PHONY: package-ios
package-ios: install-fyne-cli ## Package app for iOS (requires Xcode/toolchain)
	cd $(MAIN_DIR) && fyne package -os ios -app-id $(APP_ID)

.PHONY: package-ios-simulator
package-ios-simulator: install-fyne-cli ## Package app for iOS Simulator (requires Xcode)
	cd $(MAIN_DIR) && fyne package -os iossimulator -app-id $(APP_ID)

.PHONY: package-desktop
package-desktop: package-linux package-windows ## Package desktop platforms (run on respective hosts)

.PHONY: package-mobile
package-mobile: package-android package-ios ## Package mobile platforms

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

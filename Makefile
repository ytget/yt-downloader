SHELL:=bash

.DEFAULT_GOAL := help

# App metadata
APP_ID ?= com.github.ytget.ytdownloader
BINARY_NAME ?= yt-downloader
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
ICON ?= Icon.png
OUTPUT_DIR ?= dist

# Install locations
GOBIN_PATH := $(shell go env GOBIN)
GOPATH_PATH := $(shell go env GOPATH)
BIN_DIR := $(if $(GOBIN_PATH),$(GOBIN_PATH),$(GOPATH_PATH)/bin)

.PHONY: help
help: ## Available commands
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[0;33m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""

##@ Development

.PHONY: run
run: check-deps ## Run application
	go run main.go

.PHONY: debug
debug: check-deps ## Run app and tee logs to debug.log
	@echo "Running with logs -> debug.log"
	@set -o pipefail; go run main.go 2>&1 | tee debug.log

.PHONY: check-deps
check-deps: ## Check that all required dependencies are available
	@echo "Checking dependencies..."
	@command -v go >/dev/null 2>&1 || { echo "Error: Go is not installed or not in PATH"; exit 1; }
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

##@ Dependencies

.PHONY: deps
deps: ## Download dependencies
	go mod download
	go mod tidy

.PHONY: deps-update
deps-update: ## Update dependencies
	go get -u ./...
	go mod tidy

##@ Build

.PHONY: build
build: ## Build binary with version information
	go build -ldflags "-X main.version=$(VERSION)" -o bin/yt-downloader main.go

.PHONY: install
install: ## Install binary to $$GOBIN or $$GOPATH/bin
	@echo "Installing to $(BIN_DIR)"
	@mkdir -p "$(BIN_DIR)"
	go build -ldflags "-X main.version=$(VERSION)" -o "$(BIN_DIR)/$(BINARY_NAME)" main.go
	@echo "Installed: $(BIN_DIR)/$(BINARY_NAME) (v$(VERSION))"

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/ $(OUTPUT_DIR)/ fyne-cross/

##@ Cross-platform Build

.PHONY: build-linux
build-linux: ## Cross-build Linux (amd64, arm64)
	@go install github.com/fyne-io/fyne-cross@latest
	@fyne-cross linux \
	  --arch=amd64,arm64 \
	  --name $(BINARY_NAME) \
	  --icon $(ICON) \
	  --output $(OUTPUT_DIR) \
	  --ldflags '-X=main.version=$(VERSION)'

.PHONY: build-windows
build-windows: ## Cross-build Windows (amd64)
	@go install github.com/fyne-io/fyne-cross@latest
	@fyne-cross windows \
	  --arch=amd64 \
	  --name $(BINARY_NAME) \
	  --icon $(ICON) \
	  --output $(OUTPUT_DIR) \
	  --ldflags '-X=main.version=$(VERSION)'

.PHONY: build-android
build-android: ## Cross-build Android (arm64, arm)
	@go install github.com/fyne-io/fyne-cross@latest
	@fyne-cross android \
	  --arch=arm64,arm \
	  --name $(BINARY_NAME) \
	  --icon $(ICON) \
	  --app-id $(APP_ID) \
	  --output $(OUTPUT_DIR) \
	  --ldflags '-X=main.version=$(VERSION)'

##@ Android Device (USB)

# Android Installation Guide:
# - First time: make android-device-install
# - Update existing: make android-device-install  
# - Clean reinstall: make android-device-reinstall
# - View logs: make android-device-logs

.PHONY: android-device-build
android-device-build: ## Build APK for Android device
	@echo "Building APK for Android device..."
	@make build-android

.PHONY: android-device-install
android-device-install: android-device-build ## Install APK on connected Android device
	@echo "Installing APK on Android device..."
	@echo "Note: This command installs/updates the app (keeps existing data)."
	@echo "For clean installation, use 'make android-device-reinstall' instead."
	@if ! command -v adb >/dev/null 2>&1; then \
		echo "Error: ADB not found. Install Android SDK or platform-tools."; \
		echo "Install: brew install android-platform-tools"; \
		exit 1; \
	fi
	@if ! adb devices | grep -q "device$$"; then \
		echo "Error: No Android device connected or authorized."; \
		echo "Make sure USB debugging is enabled and device is connected."; \
		echo "Enable: Settings > Developer Options > USB Debugging"; \
		exit 1; \
	fi
	@apk_path="fyne-cross/dist/android-arm64/dist.apk"; \
	if [ -f "$$apk_path" ]; then \
		echo "Installing $$apk_path..."; \
		adb install -r "$$apk_path"; \
		echo "Installation completed!"; \
	else \
		echo "Error: APK not found. Run 'make android-device-build' first."; \
		exit 1; \
	fi

.PHONY: android-device-reinstall
android-device-reinstall: android-device-build ## Reinstall APK on Android device (uninstall + install)
	@echo "Reinstalling APK on Android device..."
	@echo "Note: This command will uninstall existing app and install fresh version."
	@echo "For first-time installation, use 'make android-device-install' instead."
	@if ! command -v adb >/dev/null 2>&1; then \
		echo "Error: ADB not found. Install Android SDK or platform-tools."; \
		echo "Install: brew install android-platform-tools"; \
		exit 1; \
	fi
	@if ! adb devices | grep -q "device$$"; then \
		echo "Error: No Android device connected or authorized."; \
		echo "Make sure USB debugging is enabled and device is connected."; \
		echo "Enable: Settings > Developer Options > USB Debugging"; \
		exit 1; \
	fi
	@echo "Uninstalling current app..."; \
	adb uninstall $(APP_ID) 2>/dev/null || echo "App not installed or already removed"; \
	@apk_path="fyne-cross/dist/android-arm64/dist.apk"; \
	if [ -f "$$apk_path" ]; then \
		echo "Installing $$apk_path..."; \
		adb install "$$apk_path"; \
		echo "Reinstallation completed!"; \
	else \
		echo "Error: APK not found. Run 'make android-device-build' first."; \
		exit 1; \
	fi

.PHONY: android-device-logs
android-device-logs: ## View logs from Android device
	@echo "Viewing logs from Android device..."
	@echo "Press Ctrl+C to stop monitoring"
	@if ! command -v adb >/dev/null 2>&1; then \
		echo "Error: ADB not found. Install Android SDK or platform-tools."; \
		exit 1; \
	fi
	@if ! adb devices | grep -q "device$$"; then \
		echo "Error: No Android device connected or authorized."; \
		echo "Make sure USB debugging is enabled and device is connected."; \
		exit 1; \
	fi
	@adb logcat | grep -E "(com.github.ytget.ytdownloader|ytdlp|Download|Error|Exception|Fatal)"

##@ Android Emulator

.PHONY: android-emulator-build
android-emulator-build: ## Build APK for Android emulator
	@echo "Building APK for Android emulator..."
	@make build-android

.PHONY: android-emulator-start
android-emulator-start: ## Start Android emulator (pixel34arm)
	@echo "Starting Android emulator..."
	@emulator -avd pixel34arm -no-snapshot-load &
	@echo "Emulator starting in background..."
	@echo "Use 'make android-emulator-wait' to wait for it to be ready"

.PHONY: android-emulator-wait
android-emulator-wait: ## Wait for emulator to be ready
	@echo "Waiting for emulator to be ready..."
	@adb wait-for-device
	@echo "Emulator is ready!"

.PHONY: android-emulator-install
android-emulator-install: android-emulator-build ## Install APK on Android emulator
	@echo "Installing APK on Android emulator..."
	@if ! command -v adb >/dev/null 2>&1; then \
		echo "Error: ADB not found. Install Android SDK or platform-tools."; \
		exit 1; \
	fi
	@if ! adb devices | grep -q "emulator"; then \
		echo "Error: No Android emulator running."; \
		echo "Run 'make android-emulator-start' first."; \
		exit 1; \
	fi
	@apk_path="fyne-cross/dist/android-arm64/dist.apk"; \
	if [ -f "$$apk_path" ]; then \
		echo "Installing $$apk_path..."; \
		adb install -r "$$apk_path"; \
		echo "Installation completed!"; \
	else \
		echo "Error: APK not found. Run 'make android-emulator-build' first."; \
		exit 1; \
	fi

.PHONY: android-emulator-reinstall
android-emulator-reinstall: android-emulator-build ## Reinstall APK on Android emulator (uninstall + install)
	@echo "Reinstalling APK on Android emulator..."
	@if ! command -v adb >/dev/null 2>&1; then \
		echo "Error: ADB not found. Install Android SDK or platform-tools."; \
		exit 1; \
	fi
	@if ! adb devices | grep -q "emulator"; then \
		echo "Error: No Android emulator running."; \
		echo "Run 'make android-emulator-start' first."; \
		exit 1; \
	fi
	@echo "Uninstalling current app..."; \
	adb uninstall $(APP_ID) 2>/dev/null || echo "App not installed or already removed"; \
	@apk_path="fyne-cross/dist/android-arm64/dist.apk"; \
	if [ -f "$$apk_path" ]; then \
		echo "Installing $$apk_path..."; \
		adb install "$$apk_path"; \
		echo "Reinstallation completed!"; \
	else \
		echo "Error: APK not found. Run 'make android-emulator-build' first."; \
		exit 1; \
	fi

.PHONY: android-emulator-logs
android-emulator-logs: ## View logs from Android emulator
	@echo "Viewing logs from Android emulator..."
	@echo "Press Ctrl+C to stop monitoring"
	@if ! command -v adb >/dev/null 2>&1; then \
		echo "Error: ADB not found. Install Android SDK or platform-tools."; \
		exit 1; \
	fi
	@if ! adb devices | grep -q "emulator"; then \
		echo "Error: No Android emulator running."; \
		echo "Run 'make android-emulator-start' first."; \
		exit 1; \
	fi
	@adb logcat | grep -E "(com.github.ytget.ytdownloader|ytdlp|Download|Error|Exception|Fatal)"

.PHONY: android-emulator-stop
android-emulator-stop: ## Stop Android emulator
	@echo "Stopping Android emulator..."
	@adb emu kill
	@echo "Emulator stopped"

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

.PHONY: b
b: ## Build binary
	@make build

.PHONY: i
i: ## Install binary
	@make install

.PHONY: c
c: ## Clean artifacts
	@make clean
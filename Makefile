SHELL:=bash

.DEFAULT_GOAL := help

.PHONY: help
help: ## Available commands
	@clear
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[0;33m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""

##@ Development

.PHONY: run
run: ## Run application
	go run cmd/yt-downloader/main.go

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

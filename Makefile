.PHONY: help \
http \
http-hot \
job \
lint \
test \
test-verbose \
test-coverage \
test-coverage-html \
test-clean \
test-local \
build \
build-all \
docs \
install-pre-push-hook \
uninstall-pre-push-hook

help:
	@echo "Makefile commands:"
	@echo "  make http                    - Start the HTTP server"
	@echo "  make http-hot                - Start the HTTP server with hot reload (requires air)"
	@echo "  make job                     - Run migrations"
	@echo "  make lint                    - Run golangci-lint on the codebase"
	@echo "  make test                    - Run all tests"
	@echo "  make test-verbose            - Run all tests with verbose output"
	@echo "  make test-coverage           - Run all tests with coverage report"
	@echo "  make test-coverage-html      - Run all tests and generate HTML coverage report"
	@echo "  make test-clean              - Clean test cache and run tests"
	@echo "  make test-local              - Clean test cache and run tests with rtk"
	@echo "  make build                   - Build HTTP server for production"
	@echo "  make build-all               - Build all programs for production"
	@echo "  make docs                    - Generate Swagger + Markdown docs"
	@echo "  make install-pre-push-hook   - Install git pre-push hook for linting and testing"
	@echo "  make uninstall-pre-push-hook - Uninstall git pre-push hook"

http:
	go run ./cmd/http

http-hot:
	air --build.cmd "go build -o bin/http ./cmd/http" --build.bin "./bin/http"

job:
	go run ./cmd/job

lint:
	golangci-lint run ./...

test:
	@echo "Running all tests..."
	go test -p 1 ./internal/tests/...
	go test $$(go list ./internal/... | grep -v '/domain/repository' | grep -v '/tests')

test-verbose:
	@echo "Running all tests with verbose output..."
	go test -v -p 1 ./internal/domain/repository/... ./internal/tests/...
	go test -v $$(go list ./internal/... | grep -v '/domain/repository' | grep -v '/tests')

test-coverage:
	@echo "Running all tests with coverage report..."
	go test -v -p 1 -coverprofile=coverage.out -covermode=atomic ./internal/domain/repository/... ./internal/tests/...
	go test -v -coverprofile=coverage2.out -covermode=atomic $$(go list ./internal/... | grep -v '/domain/repository' | grep -v '/tests')
	@tail -n +2 coverage2.out >> coverage.out && rm coverage2.out

test-coverage-html:
	@echo "Running all tests and generating HTML coverage report..."
	go test -v -p 1 -coverprofile=coverage.out -covermode=atomic ./internal/domain/repository/... ./internal/tests/...
	go test -v -coverprofile=coverage2.out -covermode=atomic $$(go list ./internal/... | grep -v '/domain/repository' | grep -v '/tests')
	@tail -n +2 coverage2.out >> coverage.out && rm coverage2.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-clean:
	@echo "Cleaning test cache and running tests..."
	go clean -testcache
	go test -v -p 1 ./internal/tests/...
	go test -v $$(go list ./internal/... | grep -v '/domain/repository' | grep -v '/tests')

test-local:
	@echo "Cleaning test cache and running tests..."
	go clean -testcache
	rtk go test -v -p 1 ./internal/tests/...
	rtk go test -v $$(go list ./internal/... | grep -v '/domain/repository' | grep -v '/tests')

build-all:
	@echo "Building all programs..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false -ldflags='-w -s' -o bin/http ./cmd/http
	CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false -ldflags='-w -s' -o bin/job ./cmd/job
	@echo "Build success! Binaries are located in bin/"
	@ls -lh bin/

docs:
	@echo "Generating Swagger docs..."
	@./scripts/swag-init.sh
	@echo "Docs generated: docs/swagger.json, docs/swagger.yaml, docs/docs.go"

install-pre-push-hook:
	@mkdir -p .git/hooks
	@cp scripts/git-pre-push.sh .git/hooks/pre-push
	@chmod +x .git/hooks/pre-push
	@echo "Pre-push hook installed successfully!"

uninstall-pre-push-hook:
	@echo "Uninstalling pre-push git hook..."
	@rm -f .git/hooks/pre-push
	@echo "Pre-push hook uninstalled successfully!"

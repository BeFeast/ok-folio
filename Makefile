.PHONY: test coverage coverage-html coverage-report clean help retirement-preflight

# Default target
help:
	@echo "Available targets:"
	@echo "  test                  - Run all tests"
	@echo "  coverage              - Run tests with coverage"
	@echo "  coverage-html         - Generate HTML coverage report"
	@echo "  coverage-report       - Show coverage summary"
	@echo "  retirement-preflight  - Run the read-only Wave 6 legacy-retirement preflight"
	@echo "  clean                 - Clean coverage files"
	@echo "  help                  - Show this help message"

# Run the read-only Wave 6 legacy-retirement preflight verifier. Offline by
# default; pass LIVE_URL=http://host:8080 to also run the read-only connector
# probe against a running app.
retirement-preflight:
	@go run ./cmd/ok-folio-preflight $(if $(LIVE_URL),--live-connectors-url $(LIVE_URL),)

# Run all tests
test:
	@echo "Running tests..."
	@go test ./... -v

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	@go test ./... -coverprofile=coverage/coverage.out || true
	@echo ""
	@go tool cover -func=coverage/coverage.out | tail -1

# Generate HTML coverage report
coverage-html: coverage
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report: coverage/coverage.html"

# Show detailed coverage report
coverage-report: coverage
	@echo ""
	@echo "=== Package Coverage ==="
	@go tool cover -func=coverage/coverage.out | grep -E "^(ok-folio|total)" | \
		awk '{printf "%-40s %s\n", $$1, $$3}'
	@echo ""
	@echo "=== Overall Coverage ==="
	@go tool cover -func=coverage/coverage.out | tail -1

# Clean coverage files
clean:
	@echo "Cleaning coverage files..."
	@rm -rf coverage/
	@find . -name "*.test" -type f -delete
	@echo "Done!"

# Run tests for specific package
test-pkg:
	@if [ -z "$(PKG)" ]; then \
		echo "Usage: make test-pkg PKG=internal/api"; \
		exit 1; \
	fi
	@go test ./$(PKG) -v -coverprofile=coverage/$(PKG).out
	@go tool cover -func=coverage/$(PKG).out | tail -1

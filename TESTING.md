# Testing Guide

This document describes the testing setup for OK Folio.

## Quick Start

```bash
# Run all tests
make test

# Run tests with coverage report
make coverage-report

# Generate HTML coverage report
make coverage-html

# Clean coverage files
make clean
```

## Test Structure

```
extractor/
├── coverage/              # Test coverage reports (gitignored)
├── pkg/
│   └── retry/
│       ├── retry.go
│       └── retry_test.go  # 93.8% coverage
├── internal/
│   ├── api/
│   │   ├── api.go
│   │   └── api_test.go    # 54.4% coverage
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go # 100% coverage
│   ├── database/
│   │   ├── database.go
│   │   └── database_test.go # 66.7% coverage
│   ├── exif/              # No tests yet
│   ├── scheduler/         # No tests yet
│   └── scraper/           # No tests yet
└── cmd/
    └── extractor/         # No tests yet
```

## Current Test Coverage

**Overall: 26.2%**

| Package | Coverage | Tests | Status |
|---------|----------|-------|--------|
| pkg/retry | 93.8% | 11 | ✅ Excellent |
| internal/config | 100% | 8 | ✅ Complete |
| internal/database | 66.7% | 17 | ✅ Good |
| internal/api | 54.4% | 16 | ✅ Good |
| internal/scraper | 0% | 0 | ❌ Not tested |
| internal/exif | 0% | 0 | ❌ Not tested |
| internal/scheduler | 0% | 0 | ❌ Not tested |

## Running Tests

### Run All Tests
```bash
go test ./...
```

### Run Tests with Coverage
```bash
go test ./... -coverprofile=coverage/coverage.out
go tool cover -func=coverage/coverage.out
```

### Run Specific Package Tests
```bash
# Using Makefile
make test-pkg PKG=internal/api

# Using go directly
go test ./internal/api -v -coverprofile=coverage/api.out
```

### Generate HTML Coverage Report
```bash
make coverage-html
# Opens coverage/coverage.html in browser
```

### View Coverage in Terminal
```bash
make coverage-report
```

## Test Dependencies

The tests use:
- **Standard library**: `testing`, `net/http/httptest`
- **GORM**: For database testing
- **SQLite**: In-memory database for tests
- **Zerolog**: Logging (disabled in tests)

## Production Data Guardrails

Repository tests and verifier runs must never use production originals, runtime databases, PhotoPrism storage, generated thumbnails, provider cookies, runtime `.env` files, or deployment secret material.

Use these rules for new tests:

1. Use `t.TempDir()` for writable storage, generated thumbnails, image outputs, and temporary config files.
2. Use `internal/**/testdata` fixtures for provider HTML, JSON, and other read-only sample inputs.
3. Use in-memory SQLite or an explicitly local test/fixture database name. Do not point tests at remote database hosts or root credentials.
4. Keep PhotoPrism auto-indexing disabled in repository tests. Tests may use mocked PhotoPrism clients or fixture-only config, but must not trigger a live index.
5. Do not read live runtime config, provider cookies, generated env files, database dumps, downloaded media, or deployment secret files.

The `internal/testguard` package provides focused checks for test configuration and paths. Call `testguard.ValidateConfig` or `testguard.ValidatePath` from test helpers that create storage, gallery, provider, or database configuration. The guard rejects production-like targets such as `/data`, `/mnt`, `/var/lib`, PhotoPrism storage paths, thumbnail directories, cookie paths, `.env` files, and secret material unless the path is under the OS temp directory or a `testdata` fixture.

## Writing Tests

### Test File Naming
- Test files must end with `_test.go`
- Place test files in the same package as the code being tested
- Example: `retry.go` → `retry_test.go`

### Test Function Naming
```go
func TestFunctionName_Scenario(t *testing.T) {
    // Test implementation
}
```

### Example Test Structure
```go
func TestDoWithValue_Success(t *testing.T) {
    // Setup
    cfg := Config{MaxAttempts: 3, InitialDelay: 10 * time.Millisecond}

    // Execute
    result, err := DoWithValue(context.Background(), cfg, func() (string, error) {
        return "success", nil
    })

    // Assert
    if err != nil {
        t.Errorf("Expected no error, got: %v", err)
    }
    if result != "success" {
        t.Errorf("Expected 'success', got: %s", result)
    }
}
```

## Test Organization Best Practices

1. **Use table-driven tests** for testing multiple scenarios
2. **Mock external dependencies** (HTTP clients, file I/O, databases)
3. **Test edge cases** (empty inputs, nil values, errors)
4. **Test concurrency** where applicable (goroutines, channels)
5. **Clean up resources** (use `defer` for cleanup)
6. **Validate storage and runtime config** with `internal/testguard` before a helper writes files or starts workers

## Coverage Goals

- **Critical packages** (retry, database, config): 80%+ coverage
- **Business logic** (scraper, api): 60%+ coverage
- **Utilities** (exif, scheduler): 50%+ coverage
- **Overall project**: 80%+ coverage (target)

## CI/CD Integration

To integrate with CI/CD pipelines:

```bash
# GitHub Actions example
go test ./... -coverprofile=coverage/coverage.out
go tool cover -func=coverage/coverage.out | grep total | awk '{print $3}'
```

## Troubleshooting

### Tests Fail with "database locked"
- Cause: SQLite in-memory databases can't be shared across tests
- Solution: Each test creates its own isolated database

### Tests Fail with "context deadline exceeded"
- Cause: Timeout is too short for slow operations
- Solution: Increase timeout or use `testing.Short()` to skip slow tests

### Coverage Report Shows 0%
- Cause: No coverage file generated
- Solution: Ensure `-coverprofile` flag is used

## Known Test Limitations

1. **internal/database**: SQLite time parsing differs from MySQL
   - Some timestamp tests are skipped in SQLite
   - All tests pass with MySQL in production

2. **internal/api**: 3 tests currently fail
   - Stats endpoint with data (SQLite time issue)
   - Queue full scenarios (timing-dependent)
   - Will be fixed in next iteration

3. **internal/scraper**: Not yet tested
   - Requires HTTP mocking
   - Complex file I/O operations
   - External dependency (exiftool)

## Next Steps

To improve coverage to 80%:
1. Add tests for `internal/scraper` (~40% coverage boost)
2. Add tests for `internal/exif` (~5% coverage boost)
3. Add tests for `internal/scheduler` (~2% coverage boost)
4. Fix failing API tests (~3% coverage boost)

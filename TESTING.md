# Testing Documentation

This document describes the testing infrastructure for the Share application.

## Test Types

### 1. Unit Tests (Go)
**Location:** `*_test.go` files throughout the codebase

**Coverage:**
- Cryptographic functions (`src/crypto/crypto_test.go`)
- Client functionality (`src/client/client_test.go`, `src/client/metadata_test.go`, `src/client/zip_test.go`)
- Relay server (`src/relay/relay_test.go`, `src/relay/compatibility_test.go`, `src/relay/zero_knowledge_test.go`)
- Protocol handling (`main_test.go`)

**Running:**
```bash
go test -v ./...
```

**Coverage Report:**
```bash
go test -v -cover ./...
```

### 2. Integration Tests (Go)
**Location:** `integration_test.go`

**Coverage:**
- Full file transfer: CLI sender → CLI receiver
- Folder transfer: CLI sender → CLI receiver (with zip/unzip)
- Hash verification during transfer

**Running:**
```bash
go test -v ./...
# Or skip integration tests:
go test -v -short ./...
```

### 3. End-to-End Tests (Playwright)
**Location:** `tests/` directory

**Coverage:**
- **Web-to-Web Transfer** (`web-to-web.spec.js`):
  - Browser client → Browser client file transfer
  - E2E encryption verification
  - File integrity validation

- **Web-to-CLI Transfer** (`web-to-cli.spec.js`):
  - Browser client → CLI tool file transfer
  - CLI tool → Browser client file transfer
  - Bidirectional compatibility testing

**Running:**
```bash
cd tests
./run-tests.sh
```

See [tests/README.md](tests/README.md) for detailed Playwright test documentation.

## Test Coverage Summary

| Component | Test Type | Coverage |
|-----------|-----------|----------|
| Crypto (ECDH, AES-GCM) | Unit | ~81% |
| Client (Send/Receive) | Unit + Integration | ~50% |
| Relay Server | Unit | ~50% |
| Web UI | E2E (Playwright) | Web-to-web, Web-to-CLI |
| CLI | Integration + E2E | CLI-to-CLI, CLI-to-web |

## Testing Philosophy

1. **Unit Tests**: Fast, isolated tests for individual functions and components
2. **Integration Tests**: Test interactions between components (e.g., full CLI file transfer)
3. **E2E Tests**: Test complete user workflows through real interfaces (web browser, CLI)

## Running All Tests

### Local Development
```bash
# Go tests
go test -v ./...

# Playwright tests
cd tests && ./run-tests.sh
```

### CI/CD
All tests run automatically in GitHub Actions:
- `build.yml`: Builds and runs Go tests on macOS, Linux, and Windows
- `playwright.yml`: Runs E2E tests on Linux with Chromium

## Test Data

Tests create temporary files and directories:
- Go tests: Use `os.MkdirTemp()` with automatic cleanup
- Playwright tests: Create files in `tests/` directory, cleaned up after each test

All test artifacts are excluded via `.gitignore`.

## Adding New Tests

### Adding a Go Test
1. Create or update `*_test.go` file in the relevant package
2. Follow existing test patterns (use `testing.T`, `t.Fatalf`, etc.)
3. Run `go test -v ./...` to verify

### Adding a Playwright Test
1. Create `*.spec.js` file in `tests/` directory
2. Follow existing patterns (see `web-to-web.spec.js` example)
3. Start relay server in `beforeAll` hook
4. Clean up in `afterAll` hook
5. Run `./run-tests.sh` to verify

## Debugging Tests

### Go Tests
```bash
# Verbose output
go test -v ./...

# Run specific test
go test -v -run TestIntegrationFileTransfer ./...

# With race detection
go test -v -race ./...
```

### Playwright Tests
```bash
# Headed mode (see browser)
cd tests && ./run-tests.sh --headed

# Debug mode (step through tests)
cd tests && ./run-tests.sh --debug

# View test report
npm run test:report
```

## Known Issues

### Network Restrictions
Integration and E2E tests require network access to:
- Start local relay server
- Create WebSocket connections
- Download Playwright browsers (first run only)

In restricted environments, tests may be skipped automatically.

### Port Conflicts
Tests use random ports (8080+ range) to avoid conflicts. If you encounter port issues, ensure no other services are using this range.

## Future Test Improvements

- [ ] Add tests for folder transfers in Playwright
- [ ] Add tests for multiple file transfers
- [ ] Add performance/benchmark tests
- [ ] Add tests for error conditions (network failures, corrupted data)
- [ ] Add visual regression tests for web UI
- [ ] Increase overall test coverage to 80%+

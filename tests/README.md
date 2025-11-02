# Playwright End-to-End Tests

This directory contains Playwright tests for the Share application, testing both web-to-web and web-to-CLI file transfers.

## Prerequisites

1. **Build the Share binary**: 
   ```bash
   cd /path/to/e2ecp
   cd web && npm install
   make build
   ```

2. **Install Playwright dependencies**:
   ```bash
   npm install
   ```

3. **Install Playwright browsers**:
   
   The Playwright browsers need to be installed before running tests. Try one of these methods:
   
   **Method A - Direct install** (recommended for local development):
   ```bash
   npx playwright install chromium
   ```
   
   **Method B - With system dependencies** (if Method A fails):
   ```bash
   npx playwright install --with-deps chromium
   ```
   
   **Method C - System browser** (fallback):
   If the download fails due to network restrictions, you can configure Playwright to use system-installed Chromium/Chrome. See the [Playwright documentation](https://playwright.dev/docs/browsers#installing-browsers) for details.
   
   **Note**: In CI/CD environments, you may need to use the Playwright GitHub Action which handles browser installation automatically.

## Running Tests

### Quick Start (Recommended)
The easiest way to run tests is using the helper script:

```bash
cd tests
./run-tests.sh
```

This script will:
- Check if the e2ecp binary exists (and build it if needed)
- Install test dependencies
- Install Playwright browsers if needed
- Run all Playwright tests
- Show results and test report location

### Run all tests (manual)
```bash
npm run test:e2e
```

### Run tests in headed mode (see the browser)
```bash
./run-tests.sh --headed
# Or manually:
npm run test:e2e:headed
```

### Debug tests
```bash
./run-tests.sh --debug
# Or manually:
npm run test:e2e:debug
```

### View test report
```bash
npm run test:report
```

### Run specific test file
```bash
npx playwright test tests/web-to-web.spec.js
npx playwright test tests/web-to-cli.spec.js
```

## Test Suites

### 1. Web to Web Transfer (`web-to-web.spec.js`)
Tests file transfer between two browser clients:
- Starts a local relay server
- Opens two browser contexts (sender and receiver)
- Transfers a test file from sender to receiver
- Verifies the file content matches

### 2. Web to CLI Transfer (`web-to-cli.spec.js`)
Tests file transfer between web client and CLI:
- **Web to CLI**: Uploads file from browser, receives with CLI tool
- **CLI to Web**: Sends file with CLI tool, downloads in browser
- Verifies file integrity in both directions

## Test Architecture

- Each test suite starts its own relay server on a random port to avoid conflicts
- Tests use unique room names to prevent interference
- Server processes are properly cleaned up after tests complete
- Test files are created and deleted automatically

## Troubleshooting

### Browser not installed
If you get an error about browsers not being installed:
```bash
npx playwright install chromium
```

### Port conflicts
Tests use random ports (8080+ range) to avoid conflicts. If you still have issues, ensure no other services are using these ports.

### Server not starting
Make sure the `e2ecp` binary exists in the root directory:
```bash
ls -la ../e2ecp
```

If it doesn't exist, build it:
```bash
cd .. && make build
```

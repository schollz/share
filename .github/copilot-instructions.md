# Share - Copilot Agent Instructions

## Repository Overview

**Share** is a secure, end-to-end encrypted file transfer application that enables peer-to-peer file sharing through a web interface or command-line interface. The application uses ECDH P-256 key exchange and AES-GCM authenticated encryption, with a knowledge-free websocket-based relay server that cannot access transferred data.

**Repository Stats:**
- **Type:** Go application with React web frontend
- **Size:** ~15 Go source files, 3 React components
- **Languages:** Go 1.25, JavaScript/React 19, Node.js 20+
- **Frameworks:** Cobra CLI, Vite, Gorilla WebSocket, React
- **Runtime:** Go 1.25+ required
- **Build time:** ~2-3 seconds for full build

## Build & Development Workflow

### Prerequisites
- **Go:** 1.25 or higher (specified in go.mod)
- **Node.js:** 20+ (CI uses Node 24)
- **npm:** 10+ (bundled with Node.js)

### Initial Setup (CRITICAL - Always Run First)

**ALWAYS install web dependencies before any build operation:**

```bash
cd web && npm install
```

**Why this is critical:** The Makefile's `make server` target runs `make web` which calls `npm run build`. If `node_modules` is missing, you'll get `sh: 1: vite: not found` error and build will fail with exit code 127.

### Build Commands (In Order)

1. **Install web dependencies (required first):**
   ```bash
   cd web && npm install
   ```
   - Takes ~5 seconds
   - Creates `web/node_modules/`
   - Must be run before building

2. **Build web assets:**
   ```bash
   make web
   # OR
   cd web && npm run build
   ```
   - Takes ~1.5 seconds
   - Produces: `web/dist/` directory with bundled assets
   - Outputs: index.html, CSS, JS bundles, and font assets
   - Note: Vite warns about eval in protobufjs - this is expected and safe

3. **Build Go server (includes web build):**
   ```bash
   make server
   # OR
   make build  # Alias for server
   ```
   - Takes ~2-3 seconds total
   - Automatically runs `make web` first
   - Produces: `share` binary in root directory
   - Binary embeds web assets via `//go:embed all:web/dist`

4. **Clean build artifacts:**
   ```bash
   make clean
   ```
   - Removes `share` binary and `web/dist/` directory
   - Does NOT remove `web/node_modules/`

### Testing

**Run all tests:**
```bash
go test -v -cover ./...
```
- Takes ~3-4 seconds
- Tests are in: `main_test.go`, `integration_test.go`, `src/*/client_test.go`, `src/*/crypto_test.go`, `src/*/relay_test.go`
- Integration tests start a relay server and test full file transfer
- All tests should pass on clean checkout
- Coverage: ~42-48% overall

**Run without integration tests:**
```bash
go test -v -short ./...
```

**Run specific package tests:**
```bash
go test -v ./src/relay
go test -v ./src/client
go test -v ./src/crypto
```

### Development Server

**Web development server (for UI changes):**
```bash
cd web && npm run dev
```
- Starts Vite dev server on port 5173
- Hot-reload enabled
- Does NOT start the Go relay server

**Go development with hot-reload:**
```bash
air
```
- Uses `.air.toml` configuration
- Automatically rebuilds on Go file changes
- Runs `make server` on changes
- Starts server with `./share serve`

### Running the Application

**Start relay server:**
```bash
./share serve --port 8080
```

**Send a file:**
```bash
./share send <filename> [room-name]
```

**Receive a file:**
```bash
./share receive [room-name]
```

**View help:**
```bash
./share --help
./share serve --help
./share send --help
```

## Project Structure

### Root Directory Files
```
.air.toml              # Air hot-reload configuration
.gitignore            # Standard Node.js + Go ignores
LICENSE               # MIT License
Makefile              # Build automation
README.md             # User documentation
go.mod                # Go module definition (go 1.25)
go.sum                # Go dependency checksums
install.sh            # Installation script for Linux
main.go               # CLI entry point, Cobra commands
main_test.go          # Main package tests
integration_test.go   # End-to-end integration tests
```

### Source Code Layout

**`src/relay/`** - WebSocket relay server
- `relay.go` - Server implementation, room management, WebSocket handling
- `relay_test.go` - Server tests including room limits
- `protobuf.go` - Protobuf message serialization
- `messages.proto` - Protocol buffer definitions (shared with web client)
- `messages.pb.go` - Generated protobuf code (DO NOT EDIT)
- `compatibility_test.go` - JSON/Protobuf compatibility tests
- `benchmark_test.go` - Performance benchmarks

**`src/client/`** - CLI client implementation
- `send.go` - File sending logic with encryption
- `receive.go` - File receiving logic with decryption
- `protobuf.go` - Client-side protobuf handling
- `client_test.go` - Client unit tests

**`src/crypto/`** - Cryptographic operations
- `crypto.go` - ECDH key exchange, AES-GCM encryption/decryption
- `crypto_test.go` - Comprehensive crypto tests (81% coverage)

**`web/`** - React frontend
- `src/App.jsx` - Main React component
- `src/main.jsx` - React entry point
- `src/index.css` - Tailwind CSS styles
- `src/messages.proto` - Protobuf definitions (synced with src/relay)
- `index.html` - HTML entry point
- `package.json` - Node dependencies and scripts
- `vite.config.js` - Vite build configuration
- `postcss.config.js` - Tailwind CSS configuration
- `dist/` - Built assets (embedded in Go binary)

### Key Architectural Components

1. **Relay Server** (`src/relay/`): Zero-knowledge WebSocket relay that coordinates peer connections without accessing file content
2. **Client** (`src/client/`): CLI for sending/receiving files with E2E encryption
3. **Crypto** (`src/crypto/`): ECDH P-256 + AES-GCM implementation
4. **Web UI** (`web/src/`): React-based browser interface for file transfers
5. **Protobuf Protocol**: Shared message format between Go and JavaScript clients

## CI/CD Pipeline

**GitHub Actions Workflow:** `.github/workflows/build.yml`

The CI runs three parallel jobs (macos, linux, windows) on every push to main and PRs:

**Linux/macOS workflow:**
1. Checkout code
2. Setup Node.js 24
3. Setup Go (stable)
4. `cd web && npm install`
5. `make server LDFLAGS="-X main.Version=$VERSION"`
6. `go test -v -cover ./...`
7. Verify binary runs
8. Create platform-specific zip
9. Upload artifact

**Windows workflow:**
1. Checkout code
2. Setup Node.js 24
3. Setup Go (stable)
4. `cd web; npm install` (PowerShell)
5. `cd web; npm run build` (PowerShell)
6. `go test -v -cover ./...`
7. `go build -ldflags "-X main.Version=$VERSION" -o share.exe .`
8. Verify binary exists
9. Create zip
10. Upload artifact

**On tags:** A `release` job creates a GitHub release with all three platform binaries.

**To replicate CI locally:**
```bash
# Install dependencies (REQUIRED)
cd web && npm install && cd ..

# Build
make server LDFLAGS="-X main.Version=dev"

# Test
go test -v -cover ./...

# Verify
./share --help
```

## Common Issues & Solutions

### Build Failures

**Error: `sh: 1: vite: not found`**
- **Cause:** Missing `web/node_modules`
- **Solution:** Run `cd web && npm install` before building
- **Prevention:** ALWAYS run npm install after cloning or cleaning

**Error: `cannot find package`**
- **Cause:** Missing Go dependencies
- **Solution:** Run `go mod download` or just build (Go auto-downloads)

### Test Failures

**Integration test skipped in sandbox:**
- This is expected if network restrictions exist
- Integration tests check for permission errors and skip gracefully

### File Paths

- **ALWAYS** use absolute paths in commands when working programmatically
- Repository is cloned to: `/home/runner/work/share/share` (in CI)
- Use `$(pwd)` or `${PWD}` for relative operations

## Configuration Files

- **Go module:** `go.mod` (Go 1.25+, defines dependencies)
- **Air config:** `.air.toml` (hot-reload for development)
- **Vite config:** `web/vite.config.js` (React build, port 5173)
- **Tailwind:** `web/postcss.config.js` (@tailwindcss/postcss)
- **Git ignore:** `.gitignore` (excludes node_modules, share binary, web/dist/*)

**Note on web/dist/:** The `.gitignore` includes `web/dist/*` but has a `.keep` file. Build artifacts in `web/dist/` are generated and should not be committed except for the `.keep` marker.

## Protobuf Files

**Source files:**
- `src/relay/messages.proto` - Server/protocol definitions
- `web/src/messages.proto` - Should match relay version

**Generated files (DO NOT EDIT):**
- `src/relay/messages.pb.go` - Generated by protoc

**Regenerating protobuf (if needed):**
```bash
# Requires protoc installation
protoc --go_out=. --go_opt=paths=source_relative \
  src/relay/messages.proto
```

Note: Protobuf files are pre-generated and checked in. You typically don't need to regenerate unless changing the protocol.

## Code Quality

**Formatting:**
- Go code: Use `gofmt` (available at `/usr/bin/gofmt`)
- Run: `gofmt -w .` before committing Go changes

**No linting enforced in CI** - but gofmt is standard practice for Go code.

## Making Changes - Checklist

When modifying this repository:

1. **Before any build:** `cd web && npm install` (if node_modules missing)
2. **After Go changes:** 
   - Run `go test ./...` to verify tests pass
   - Run `make server` to ensure it builds
3. **After web changes:**
   - Run `cd web && npm run build` to verify bundle builds
   - Check `web/dist/` is created
   - Run full build with `make server`
4. **Before committing:**
   - Run `gofmt -w .` on changed Go files
   - Run full test suite: `go test -v -cover ./...`
   - Ensure `make clean && make build` works
5. **Remember:** The share binary embeds web/dist via go:embed, so web changes require rebuilding the Go binary

## Important Notes

- **Embedded Assets:** The Go binary embeds web/dist and install.sh via `//go:embed` directive in main.go
- **Cross-compilation:** The CI builds for macOS, Linux, and Windows separately
- **Version injection:** Use `LDFLAGS="-X main.Version=<version>"` to set version
- **Room limits:** Relay server supports configurable max rooms (--max-rooms flag)
- **E2E Encryption:** All file transfers use ECDH P-256 key exchange + AES-GCM
- **No external dependencies for relay:** Server has no knowledge of file contents

## Trust These Instructions

These instructions have been validated by running the build and test processes. If you encounter an issue not covered here, investigate it, but **start by following these instructions exactly** - they represent the verified happy path for working with this repository.

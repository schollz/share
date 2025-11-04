# e2ecp - Copilot Agent Instructions

## Repository Overview

**e2ecp** is a secure, end-to-end encrypted file transfer application that enables peer-to-peer file sharing through a web interface or command-line interface. The application uses ECDH P-256 key exchange and AES-GCM authenticated encryption, with a knowledge-free websocket-based relay server that cannot access transferred data.

**Repository Stats:**
- **Type:** Go application with React web frontend
- **Size:** ~9 main Go source files (plus tests), 4 React components
- **Languages:** Go 1.25, JavaScript/React 19.2.0, Node.js 20+
- **Frameworks:** Cobra CLI, Vite, Gorilla WebSocket, React
- **Runtime:** Go 1.25+ required
- **Build time:** ~2-3 seconds for full build

## Recent Major Features (2024-2025)

The following significant features have been added recently:

1. **SHA256 Hash Verification** (#40) - File integrity checking via SHA256 hashes
2. **Zero-Knowledge Metadata Encryption** (#38) - File metadata (name, size, hash) encrypted before relay transmission
3. **Folder Support** - Automatic zip/unzip for folder transfers, supports multiple files
4. **QR Code Generation** - Terminal and web-based QR codes for easy room sharing (rsc.io/qr)
5. **Drag and Drop** - Improved web UI with drag-and-drop file/folder support
6. **Overwrite Protection** - --force flag prevents accidental file overwrites
7. **Per-IP Room Limits** - --max-rooms-per-ip flag prevents abuse
8. **Path Traversal Protection** (#10) - Security fix for file reception
9. **Protobuf Protocol** (#8) - 5-75x performance improvement over JSON

**New Dependencies:**
- Go: `rsc.io/qr` (QR code generation)
- Web: `jszip` (folder handling), `qrcode.react` (QR display), React 19.2.0

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
   - Produces: `e2ecp` binary in root directory
   - Binary embeds web assets via `//go:embed all:web/dist`

4. **Clean build artifacts:**
   ```bash
   make clean
   ```
   - Removes `e2ecp` binary and `web/dist/` directory
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
- Starts server with `./e2ecp serve`

### Running the Application

**Start relay server:**
```bash
./e2ecp serve --port 8080
# Optional flags:
# --max-rooms <n>         # Maximum number of rooms (default: unlimited)
# --max-rooms-per-ip <n>  # Maximum rooms per IP address (prevents abuse)
```

**Send a file or folder:**
```bash
./e2ecp send <file-or-folder> [room-name]
# Folders are automatically zipped for transfer
# Displays QR code in terminal for easy room sharing
```

**Receive a file or folder:**
```bash
./e2ecp receive [room-name]
# Folders are automatically extracted
# Optional flags:
# --output <path>   # Specify output location
# --force          # Overwrite existing files without prompting
```

**View help:**
```bash
./e2ecp --help
./e2ecp serve --help
./e2ecp send --help
./e2ecp receive --help
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
- `relay.go` - Server implementation, room management, WebSocket handling (protobuf-only)
- `relay_test.go` - Server tests including room limits
- `protobuf.go` - Protobuf message serialization
- `messages.proto` - Protocol buffer definitions (e2ecpd with web client)
- `messages.pb.go` - Generated protobuf code (DO NOT EDIT)
- `compatibility_test.go` - Protobuf messaging tests
- `benchmark_test.go` - Performance benchmarks
- `zero_knowledge_test.go` - Tests for encrypted metadata transmission

**`src/client/`** - CLI client implementation
- `send.go` - File sending logic with encryption
- `receive.go` - File receiving logic with decryption
- `protobuf.go` - Client-side protobuf handling
- `metadata.go` - Encrypted file metadata structure (name, size, hash, folder info)
- `metadata_test.go` - Metadata handling tests
- `zip.go` - Folder compression/decompression for folder transfers
- `zip_test.go` - Zip functionality tests
- `client_test.go` - Client unit tests

**`src/crypto/`** - Cryptographic operations
- `crypto.go` - ECDH key exchange, AES-GCM encryption/decryption
- `crypto_test.go` - Comprehensive crypto tests (81% coverage)

**`src/qrcode/`** - QR code generation
- `qrcode.go` - Terminal QR code display using half-block characters
- `qrcode_test.go` - QR code generation tests

**`web/`** - React frontend
- `src/App.jsx` - Main React component with drag-and-drop support
- `src/main.jsx` - React entry point
- `src/index.css` - Tailwind CSS styles
- `src/messages.proto` - Protobuf definitions (synced with src/relay)
- `index.html` - HTML entry point
- `package.json` - Node dependencies (React 19.2.0, jszip, qrcode.react, protobufjs)
- `vite.config.js` - Vite build configuration
- `postcss.config.js` - Tailwind CSS configuration
- `dist/` - Built assets (embedded in Go binary)

### Key Architectural Components

1. **Relay Server** (`src/relay/`): Zero-knowledge WebSocket relay that coordinates peer connections without accessing file content or metadata
2. **Client** (`src/client/`): CLI for sending/receiving files with E2E encryption, folder support via automatic zip/unzip
3. **Crypto** (`src/crypto/`): ECDH P-256 + AES-GCM implementation
4. **Web UI** (`web/src/`): React-based browser interface with drag-and-drop, QR codes, and folder support
5. **Protobuf Protocol**: e2ecpd message format between Go and JavaScript clients
6. **QR Code Display** (`src/qrcode/`): Terminal-based QR code generation for easy room sharing
7. **Encrypted Metadata**: File names, sizes, and SHA256 hashes encrypted before relay transmission

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
7. `go build -ldflags "-X main.Version=$VERSION" -o e2ecp.exe .`
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
./e2ecp --help
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
- Repository is cloned to: `/home/runner/work/e2ecp/e2ecp` (in CI)
- Use `$(pwd)` or `${PWD}` for relative operations

## Configuration Files

- **Go module:** `go.mod` (Go 1.25+, defines dependencies)
- **Air config:** `.air.toml` (hot-reload for development)
- **Vite config:** `web/vite.config.js` (React build, port 5173)
- **Tailwind:** `web/postcss.config.js` (@tailwindcss/postcss)
- **Git ignore:** `.gitignore` (excludes node_modules, e2ecp binary, web/dist/*)

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
5. **Remember:** The e2ecp binary embeds web/dist via go:embed, so web changes require rebuilding the Go binary

## Important Notes

- **Embedded Assets:** The Go binary embeds web/dist and install.sh via `//go:embed` directive in main.go
- **Cross-compilation:** The CI builds for macOS, Linux, and Windows separately
- **Version injection:** Use `LDFLAGS="-X main.Version=<version>"` to set version
- **Room limits:** Relay server supports configurable max rooms (--max-rooms and --max-rooms-per-ip flags)
- **E2E Encryption:** All file transfers use ECDH P-256 key exchange + AES-GCM
- **Zero-knowledge relay:** Server has no knowledge of file contents or metadata (filename, size encrypted before transmission)
- **Folder support:** Folders automatically zipped/unzipped; supports multiple files
- **SHA256 verification:** File integrity verified via SHA256 hash transmitted in encrypted metadata
- **QR codes:** Both CLI and web display QR codes for easy room sharing
- **Overwrite protection:** --force flag required to overwrite existing files on receive

## Trust These Instructions

These instructions have been validated by running the build and test processes. If you encounter an issue not covered here, investigate it, but **start by following these instructions exactly** - they represent the verified happy path for working with this repository.

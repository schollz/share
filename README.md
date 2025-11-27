<p align="center">
  <a href="https://e2ecp.com"><img width="600" alt="share-screenshot" src="https://github.com/user-attachments/assets/cf57fe62-329e-4d04-b9a1-af8ec3516b3e" /></a>
  <br>
  <a href="https://github.com/schollz/e2ecp/releases/latest"><img src="https://img.shields.io/github/v/release/schollz/e2ecp" alt="Version"></a>
  <a href="https://github.com/schollz/e2ecp/actions/workflows/build.yml"><img src="https://github.com/schollz/e2ecp/actions/workflows/build.yml/badge.svg" alt="Build Status"></a>
  <a href="https://github.com/sponsors/schollz"><img src="https://img.shields.io/github/sponsors/schollz" alt="GitHub Sponsors"></a>
</p>

Transfer files between machines through the web, through a CLI, or both.

## Usage

Goto <https://e2ecp.com> and enter a room. Have another peer enter the same room to transfer files between the two.

You can use the [command-line tool](#command-line-tool), e2ecp, as a client that can send/receive from the website or from another command-line:

**Sending a file:**

```bash
e2ecp send <file>
```

**Receiving a file:**

```bash
e2ecp receive
```

**Uploading to your account:**

First, register at <https://e2ecp.com>. Then authenticate:

```bash
e2ecp auth
```

Upload encrypted files to your account:

```bash
e2ecp upload <file>
```

Files are end-to-end encrypted before upload. The password you enter must match your account password.

## Command-line tool

The command-line tool, e2ecp, is available as a single binary for Windows, Mac OS, or Linux. Simply download the [latest release](https://github.com/schollz/e2ecp/releases/latest) or install with:

### Homebrew

```bash
brew install schollz/tap/e2ecp
```

### Arch Linux

```bash
paru -S e2ecp
# or
yay -S e2ecp
```

### Other distros

```bash
curl https://e2ecp.com | bash
```

### Source code

Make sure you have node (>=v24) and Go installed, then clone and build:

```bash
git clone https://github.com/schollz/e2ecp
cd e2ecp && make
```


## Run your own relay server

You can run your own relay server if you want to self-host, using the [command-line tool](#command-line-tool).

```bash
e2ecp serve --port 8080
```

### Enable profile/storage features

By default, the relay only serves WebSocket rooms. To enable the profile, login, and encrypted file storage APIs, create a `.env` file:

```bash
ALLOW_STORAGE_PROFILE=yes

# Database configuration
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=your_password
DATABASE_NAME=e2ecp

# Email configuration (for registration/verification)
MAILJET_API_KEY=your_api_key
MAILJET_API_SECRET=your_api_secret
MAILJET_FROM_EMAIL=noreply@yourdomain.com
MAILJET_FROM_NAME=YourAppName
```

Requires PostgreSQL for user accounts and file storage. Get Mailjet credentials at <https://www.mailjet.com>. If `ALLOW_STORAGE_PROFILE` is not `yes`, profile endpoints stay disabled and neither database nor Mailjet are required.

### Using Docker

You can also run the relay server using Docker. There are two options:

#### Option 1: Build from pre-compiled binary (faster)

First, build the binary locally:

```bash
make build
```

Then build and run the Docker container:

```bash
# Build the Docker image
docker build -t e2ecp-relay .

# Run with default settings (port 3001)
docker run -p 3001:3001 e2ecp-relay

# Run with custom settings
docker run -p 8080:8080 e2ecp-relay --port 8080 --max-rooms 50 --max-rooms-per-ip 5 --log-level debug
```

#### Option 2: Build everything from source (no local dependencies required)

Use the multi-stage Dockerfile that builds both the web assets and Go binary:

```bash
# Build the Docker image from source
docker build -f Dockerfile.build -t e2ecp-relay .

# Run with default settings
docker run -p 3001:3001 e2ecp-relay

# Run with custom settings
docker run -p 8080:8080 e2ecp-relay --port 8080 --max-rooms 50 --max-rooms-per-ip 5 --log-level debug
```

**Available options:**
- `--port`: Port to listen on (default: 3001)
- `--max-rooms`: Maximum number of concurrent rooms (default: 10)
- `--max-rooms-per-ip`: Maximum rooms per IP address (default: 2)
- `--log-level`: Logging level - debug, info, warn, error (default: info)

### Using Docker Compose

#### Option 1: Build from pre-compiled binary (faster)

`compose.yml`
```yaml
services:
  e2ecp-relay:
    pull_policy: build
    build:
      context: https://github.com/schollz/e2ecp.git#${VERSION:-main}
      dockerfile: Dockerfile
      labels:
        - x-e2ecp-relay-version=${VERSION:-main}
    ports:
      - ${PORT:-3001}:${PORT:-3001}
    command:
      - --port
      - ${PORT:-3001}
      - --max-rooms
      - ${MAX_ROOMS:-10}
      - --max-rooms-per-ip
      - ${MAX_ROOMS_PER_IP:-2}
      - --log-level
      - ${LOG_LEVEL:-info}
```

`.env`
```
VERSION=v3.0.5
PORT=8080
MAX_ROOMS=50
MAX_ROOMS_PER_IP=5
LOG_LEVEL=debug
ALLOW_STORAGE_PROFILE=yes
```

#### Option 2: Build everything from source (no local dependencies required)

`compose.yml`
```yaml
services:
  e2ecp-relay:
    pull_policy: build
    build:
      context: https://github.com/schollz/e2ecp.git#${VERSION:-main}
      dockerfile: Dockerfile.build
      labels:
        - x-e2ecp-relay-version=${VERSION:-main}
    ports:
      - ${PORT:-3001}:${PORT:-3001}
    command:
      - --port
      - ${PORT:-3001}
      - --max-rooms
      - ${MAX_ROOMS:-10}
      - --max-rooms-per-ip
      - ${MAX_ROOMS_PER_IP:-2}
      - --log-level
      - ${LOG_LEVEL:-info}
```

`.env`
```
VERSION=v3.0.5
PORT=8080
MAX_ROOMS=50
MAX_ROOMS_PER_IP=5
LOG_LEVEL=debug
ALLOW_STORAGE_PROFILE=yes
```

## About

This project is created and maintained by [Zack](https://schollz.com). If you find it useful, please consider sponsoring me on [GitHub Sponsors](https://github.com/sponsors/schollz).

It works by using a simple peer-to-peer connection through a knowledge-free websocket-based relay server. All data transferred is encrypted using end-to-end encryption with ECDH P-256 key exchange and AES-GCM authenticated encryption, meaning the relay server does not have access to the data being transferred.

## Testing

e2ecp includes comprehensive test suites to ensure reliability:

### Unit and Integration Tests

Run Go tests for the core functionality:

```bash
go test -v ./...
```

### End-to-End Tests

Playwright tests verify web-to-web and web-to-CLI file transfers:

```bash
cd tests
./run-tests.sh
```

See [tests/README.md](tests/README.md) for detailed testing documentation.

## License

MIT — see [LICENSE](LICENSE).

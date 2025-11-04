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

## Command-line tool

The command-line tool, e2ecp, is available as a single binary for Windows, Mac OS, or Linux. Simply download the [latest release](https://github.com/schollz/e2ecp/releases/latest) or install with:

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

### Using Docker

You can also run the relay server using Docker. First, build the binary:

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

Available options:
- `--port`: Port to listen on (default: 3001)
- `--max-rooms`: Maximum number of concurrent rooms (default: 10)
- `--max-rooms-per-ip`: Maximum rooms per IP address (default: 2)
- `--log-level`: Logging level - debug, info, warn, error (default: info)

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

<p align="center">
  <a href="https://e2ecp.com">
    <img width="600" alt="share-screenshot" src="https://github.com/user-attachments/assets/cf57fe62-329e-4d04-b9a1-af8ec3516b3e" />
  </a>
  <br>
  <a href="https://github.com/schollz/e2ecp/releases/latest">
    <img src="https://img.shields.io/github/v/release/schollz/e2ecp" alt="Version">
  </a>
  <a href="https://github.com/schollz/e2ecp/actions/workflows/build.yml">
    <img src="https://github.com/schollz/e2ecp/actions/workflows/build.yml/badge.svg" alt="Build Status">
  </a>
  <a href="https://github.com/sponsors/schollz">
    <img src="https://img.shields.io/github/sponsors/schollz" alt="GitHub Sponsors">
  </a>
</p>

Transfer files between machines through the web, through a CLI, or both.

## Usage

Goto <https://e2ecp.com> and enter a room. Have another peer enter the same room to transfer files between the two.

### Command-line Interface

Optionally, you can also transfer files via the command-line between machines (through website or direct CLI-to-CLI).

Either download the [latest release](https://github.com/schollz/e2ecp/releases/latest) or install with:

### Arch Linux

```bash
paru -S e2ecp
# or
yay -S e2ecp
```

### other distros

```bash
curl https://e2ecp.com | bash
```

Or build from source with `make server`.

Then you can also send and receive files from the command-line between other command-line users or users on the web interface.

**Sending a file:**

```bash
e2ecp send <file>
```

**Receiving a file:**

```bash
e2ecp receive
```

### Run your own relay server

You can run your own relay server if you want to self-host.

```bash
e2ecp serve --port 8080
```

## About

This project is created and maintained by [Zack](https://schollz.com). If you find it useful, please consider sponsoring me on [GitHub Sponsors](https://github.com/sponsors/schollz).

It works by using a simple peer-to-peer connection through a knowledge-free websocket-based relay server. All data transferred is encrypted using end-to-end encryption with ECDH P-256 key exchange and AES-GCM authenticated encryption, meaning the relay server does not have access to the data being transferred.

### Reliability Features

e2ecp implements several features to ensure reliable file transfers:

- **Chunk Acknowledgments**: Each data chunk is acknowledged by the receiver
- **Automatic Retransmission**: Lost chunks are automatically retried (up to 3 attempts)
- **Chunk Ordering**: Out-of-order chunks are buffered and reordered correctly
- **Timeout Detection**: Transfers that stall for 30+ seconds are automatically aborted
- **Hash Verification**: File integrity is verified using SHA-256 checksums
- **Thread-Safe**: Concurrent operations are protected with mutexes

These features ensure that transfers complete successfully even over unreliable network connections.

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

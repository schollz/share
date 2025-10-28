<p align="center">
  <a href="https://www.youtube.com/watch?v=zViMACW6VbQ">
    <img width="600" alt="share-screenshot" src="https://github.com/user-attachments/assets/5c16e293-ce95-4f97-98a3-edae69a8c825" />
  </a>
  <br>
  <a href="https://github.com/schollz/share/releases/latest">
    <img src="https://img.shields.io/github/v/release/schollz/share" alt="Version">
  </a>
  <a href="https://github.com/schollz/share/actions/workflows/build.yml">
    <img src="https://github.com/schollz/share/actions/workflows/build.yml/badge.svg" alt="Build Status">
  </a>
  <a href="https://github.com/sponsors/schollz">
    <img src="https://img.shields.io/github/sponsors/schollz" alt="GitHub Sponsors">
  </a>
</p>

Share files between machines through the web, through a CLI, or both.


## Usage

Goto https://share.schollz.com and enter a room. Have another peer enter the same room to share files between the two.

### Command-line Interface

Optionally, you can also transfer files via the command-line between machines (through website or direct CLI-to-CLI).

Either download the [latest release](https://github.com/schollz/share/releases/latest) or install with:

```bash
curl https://share.schollz.com | bash
```

Or build from source with `make server`.

Then you can also send and receive files from the command-line between other command-line users or users on the web interface.

**Sending a file:**

```bash
share send <file>
```

**Receiving a file:**

```bash 
share receive
```

### Run your own relay server

You can run your own relay server if you want to self-host.

```bash
share serve --port 8080
```


## About

This project is created and maintained by [Zack](https://schollz.com). If you find it useful, please consider sponsoring me on [GitHub Sponsors](https://github.com/sponsors/schollz).

It works by using a simple peer-to-peer connection through a knowledge-free websocket-based relay server. All data transferred is encrypted using end-to-end encryption with ECDH P-256 key exchange and AES-GCM authenticated encryption, meaning the relay server does not have access to the data being transferred.

## License

MIT — see [LICENSE](LICENSE).

<p align="center">
  <a href="https://www.youtube.com/watch?v=zViMACW6VbQ">
    <img width="600" alt="vlcsnap-2025-08-23-18h24m04s244" src="https://github.com/user-attachments/assets/7d4c36c0-bd28-4611-a41b-ddf864af045c" />
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

Share moves files between machines with a clean web UI and a focused CLI.

## Install

Grab `share_linux.zip`, `share_macos.zip`, or `share_windows.zip` from the [latest release](https://github.com/schollz/share/releases/latest), unpack it, move the resulting binary into your `PATH`, and make it executable (`share.exe` on Windows ships ready to run).

## Usage

Run `share serve` to host the local web interface on `http://localhost:3001` for drag-and-drop transfers, or use the CLI directly:

- Send a file or folder with `share send <file> <room>` to push through the relay at `share.schollz.com` (override with `--domain` or `--server`).
- Receive from another peer with `share receive <room>` and optionally `--output` to choose the destination folder.

You can also join rooms from the hosted web app at https://share.schollz.com when a browser-only workflow is easier.

## License

MIT — see [LICENSE](LICENSE).

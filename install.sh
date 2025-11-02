#!/bin/bash

ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')

if [ "$ARCH" = "x86_64" ] || [ "$ARCH" = "amd64" ]; then
    ARCH="amd64"
else
    echo "The architecture $ARCH is not supported."
    exit 1
fi

if [ "$OS" != "linux" ]; then
    echo "The OS $OS is not supported."
    exit 1
fi

if ! command -v unzip >/dev/null 2>&1; then
    echo "The \"unzip\" command is required."
    exit 1
fi

echo "Downloading e2ecp for $OS $ARCH..."

DOWNLOAD_URL=$(curl -s https://api.github.com/repos/schollz/e2ecp/releases/latest | \
    grep 'browser_download_url' | \
    grep "e2ecp_${OS}.zip" | \
    cut -d '"' -f 4 | head -n 1)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "Failed to locate a release asset for e2ecp ($OS $ARCH)."
    exit 1
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

if ! curl -L "$DOWNLOAD_URL" -o "$TMPDIR/e2ecp.zip" --progress-bar; then
    echo "Failed to download e2ecp."
    exit 1
fi

if ! unzip -o "$TMPDIR/e2ecp.zip" -d "$TMPDIR" >/dev/null; then
    echo "Failed to extract e2ecp."
    exit 1
fi

BINARY_PATH="$TMPDIR/e2ecp"
if [ ! -f "$BINARY_PATH" ]; then
    echo "e2ecp binary was not found in the downloaded archive."
    exit 1
fi

chmod +x "$BINARY_PATH"

if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        SUDO="sudo"
    else
        echo "Please rerun as root or install sudo to write to /usr/local/bin."
        exit 1
    fi
else
    SUDO=""
fi

if ! $SUDO mv "$BINARY_PATH" /usr/local/bin/e2ecp; then
    echo "Failed to install e2ecp to /usr/local/bin."
    exit 1
fi

echo "Installed e2ecp to /usr/local/bin/e2ecp"
/usr/local/bin/e2ecp --version || true

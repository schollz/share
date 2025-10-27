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

echo "Downloading share for $OS $ARCH..."

DOWNLOAD_URL=$(curl -s https://api.github.com/repos/schollz/share/releases/latest | \
    grep 'browser_download_url' | \
    grep "share_${OS}.zip" | \
    cut -d '"' -f 4 | head -n 1)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "Failed to locate a release asset for share ($OS $ARCH)."
    exit 1
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

if ! curl -L "$DOWNLOAD_URL" -o "$TMPDIR/share.zip" --progress-bar; then
    echo "Failed to download share."
    exit 1
fi

if ! unzip -o "$TMPDIR/share.zip" -d "$TMPDIR" >/dev/null; then
    echo "Failed to extract share."
    exit 1
fi

BINARY_PATH="$TMPDIR/share"
if [ ! -f "$BINARY_PATH" ]; then
    echo "share binary was not found in the downloaded archive."
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

if ! $SUDO mv "$BINARY_PATH" /usr/local/bin/share; then
    echo "Failed to install share to /usr/local/bin."
    exit 1
fi

echo "Installed share to /usr/local/bin/share"
/usr/local/bin/share --version || true

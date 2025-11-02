#!/bin/bash

# Script to run Playwright tests for Share application
# This script ensures the server is built and browsers are installed before running tests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== Share Playwright Test Runner ==="
echo ""

# Step 1: Check if e2ecp binary exists
if [ ! -f "$ROOT_DIR/e2ecp" ]; then
    echo "❌ Share binary not found. Building..."
    cd "$ROOT_DIR"
    if [ -d "web/node_modules" ]; then
        echo "✓ Web dependencies already installed"
    else
        echo "Installing web dependencies..."
        cd web && npm install && cd ..
    fi
    echo "Building e2ecp binary..."
    make build
    echo "✓ Share binary built successfully"
else
    echo "✓ Share binary found"
fi

# Step 2: Check if test dependencies are installed
cd "$ROOT_DIR"
if [ ! -d "node_modules" ]; then
    echo "Installing test dependencies..."
    npm install
    echo "✓ Test dependencies installed"
else
    echo "✓ Test dependencies found"
fi

# Step 3: Check if Playwright browsers are installed
PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_BROWSERS_PATH:-$HOME/.cache/ms-playwright}"
CHROMIUM_PATH="$PLAYWRIGHT_BROWSERS_PATH/chromium-*"

if ! ls $CHROMIUM_PATH 1> /dev/null 2>&1; then
    echo "Installing Playwright Chromium browser..."
    npx playwright install chromium
    if [ $? -ne 0 ]; then
        echo "⚠️  Browser installation failed. Trying with system dependencies..."
        npx playwright install --with-deps chromium
        if [ $? -ne 0 ]; then
            echo "❌ Failed to install Playwright browsers"
            echo "Please install manually with: npx playwright install chromium"
            exit 1
        fi
    fi
    echo "✓ Playwright browsers installed"
else
    echo "✓ Playwright browsers found"
fi

# Step 4: Run tests
echo ""
echo "=== Running Playwright Tests ==="
echo ""

if [ "$1" = "--headed" ]; then
    npx playwright test --headed
elif [ "$1" = "--debug" ]; then
    npx playwright test --debug
elif [ "$1" = "--ui" ]; then
    npx playwright test --ui
else
    npx playwright test "$@"
fi

TEST_EXIT_CODE=$?

# Step 5: Show results
echo ""
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "✓ All tests passed!"
    echo ""
    echo "View HTML report with: npm run test:report"
else
    echo "❌ Some tests failed (exit code: $TEST_EXIT_CODE)"
    echo ""
    echo "View HTML report with: npm run test:report"
fi

exit $TEST_EXIT_CODE

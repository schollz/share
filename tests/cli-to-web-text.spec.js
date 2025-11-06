// @ts-check
const { test, expect } = require('@playwright/test');
const path = require('path');
const { spawn } = require('child_process');

/**
 * Test: CLI to Web text transfer
 *
 * This test verifies that text can be successfully transferred from the command-line
 * tool to a web client through the relay server.
 */
test.describe('CLI to Web Text Transfer', () => {
  let relayServer;
  let serverPort;
  let serverUrl;

  test.beforeAll(async () => {
    // Start the relay server
    serverPort = 8082 + Math.floor(Math.random() * 1000); // Random port to avoid conflicts

    return new Promise((resolve, reject) => {
      relayServer = spawn('./e2ecp', ['serve', '--port', serverPort.toString()], {
        cwd: path.join(__dirname, '..'),
      });

      let started = false;

      relayServer.stdout.on('data', (data) => {
        console.log(`Server: ${data}`);
        if (!started && data.toString().includes('Starting')) {
          started = true;
          serverUrl = `http://localhost:${serverPort}`;
          // Give server more time to fully initialize WebSocket handlers
          setTimeout(resolve, 3000);
        }
      });

      relayServer.stderr.on('data', (data) => {
        console.error(`Server Error: ${data}`);
      });

      relayServer.on('error', (error) => {
        reject(error);
      });

      // Timeout in case server doesn't start
      setTimeout(() => {
        if (!started) {
          started = true;
          serverUrl = `http://localhost:${serverPort}`;
          resolve();
        }
      }, 5000);
    });
  });

  test.afterAll(async () => {
    // Stop the relay server and wait for cleanup
    if (relayServer) {
      relayServer.kill('SIGTERM');
      // Wait for server to fully shutdown
      await new Promise(resolve => setTimeout(resolve, 2000));
    }
  });

  test('should transfer text from CLI sender to web receiver', async ({ browser }) => {
    // Test text to send
    const testText = 'Hello from CLI to web! This is a Playwright test for CLI-to-web text transfer.';

    const context = await browser.newContext();
    const page = await context.newPage();

    try {
      // Generate a unique room name
      const roomName = `test-cli-web-text-${Date.now()}`;

      // Navigate web receiver to the room first
      await page.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait for the page to be ready
      await page.waitForSelector('button:has-text("CLICK OR DROP FILES HERE"), div:has-text("WAITING FOR PEER")', { timeout: 15000 });

      // Extra wait to ensure WebSocket connection is established
      await new Promise(resolve => setTimeout(resolve, 2000));

      // Start CLI sender with text (since "test text message" is not a file, it will be sent as text)
      const wsUrl = `ws://localhost:${serverPort}`;
      const cliSender = spawn('./e2ecp', ['send', testText, roomName, '--server', wsUrl], {
        cwd: path.join(__dirname, '..'),
      });

      let senderOutput = '';

      cliSender.stdout.on('data', (data) => {
        const output = data.toString();
        senderOutput += output;
        console.log(`CLI Sender: ${output}`);
      });

      cliSender.stderr.on('data', (data) => {
        console.error(`CLI Sender Error: ${data}`);
      });

      // Wait for the text message modal to appear
      await page.waitForSelector('text=RECEIVED TEXT', { timeout: 30000 });

      // Get the text content from the modal
      const receivedText = await page.locator('.bg-gray-200 .font-bold.break-words').textContent();

      // Verify the received text matches what was sent
      expect(receivedText.trim()).toBe(testText);

      // Test copy button functionality
      await page.click('button[title="Copy to clipboard"]');

      // Look for success toast
      await page.waitForSelector('text=Text copied to clipboard!', { timeout: 5000 });

      // Close the modal
      await page.click('button:has-text("Close")');

      // Clean up
      cliSender.kill('SIGTERM');
      await new Promise(resolve => setTimeout(resolve, 1000));

    } finally {
      await page.close();
      await context.close();
    }
  });
});

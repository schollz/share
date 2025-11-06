// @ts-check
const { test, expect } = require('@playwright/test');
const path = require('path');
const { spawn } = require('child_process');

/**
 * Test: CLI to Web text message transfer
 *
 * This test verifies that a text message can be successfully sent from the command-line
 * tool to a web client through the relay server.
 */
test.describe('CLI to Web Text Messaging', () => {
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

  test('should send text message from CLI to web receiver', async ({ browser }) => {
    const testMessage = 'Hello from CLI! This is a text message sent from command line.';
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

      // Start CLI sender
      const wsUrl = `ws://localhost:${serverPort}`;
      const cliSender = spawn('./e2ecp', ['send', testMessage, roomName, '--server', wsUrl], {
        cwd: path.join(__dirname, '..'),
      });

      let senderOutput = '';

      cliSender.stdout.on('data', (data) => {
        senderOutput += data.toString();
        console.log(`CLI Sender: ${data}`);
      });

      cliSender.stderr.on('data', (data) => {
        console.error(`CLI Sender Error: ${data}`);
      });

      // Web receiver: Wait for the text message to appear
      await page.waitForSelector(`text=${testMessage}`, { timeout: 15000 });

      // Verify the message is displayed
      const messageBox = await page.locator('.bg-white.border-2').filter({ hasText: testMessage });
      await expect(messageBox).toBeVisible();

      // Verify copy button exists
      const copyButton = await messageBox.locator('button i.fa-copy');
      await expect(copyButton).toBeVisible();

      // Verify the message content
      const messageText = await messageBox.locator('div.whitespace-pre-wrap').textContent();
      expect(messageText.trim()).toBe(testMessage);

      // Wait for CLI sender to complete
      await new Promise((resolve) => {
        cliSender.on('close', (code) => {
          console.log(`CLI Sender exited with code ${code}`);
          resolve();
        });
      });

      console.log('CLI Sender output:', senderOutput);

    } finally {
      await page.close();
      await context.close();
    }
  });
});

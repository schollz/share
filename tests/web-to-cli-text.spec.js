// @ts-check
const { test, expect } = require('@playwright/test');
const path = require('path');
const { spawn } = require('child_process');

/**
 * Test: Web to CLI text message transfer
 *
 * This test verifies that a text message can be successfully sent from a web client
 * to the command-line tool through the relay server.
 */
test.describe('Web to CLI Text Messaging', () => {
  let relayServer;
  let serverPort;
  let serverUrl;

  test.beforeAll(async () => {
    // Start the relay server
    serverPort = 8083 + Math.floor(Math.random() * 1000); // Random port to avoid conflicts

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

  test('should send text message from web to CLI receiver', async ({ browser }) => {
    const testMessage = 'Hello from Web! This is a text message sent from the browser.';
    const context = await browser.newContext();
    const page = await context.newPage();

    try {
      // Generate a unique room name
      const roomName = `test-web-cli-text-${Date.now()}`;
      const wsUrl = `ws://localhost:${serverPort}`;

      // Create a temp directory for CLI receiver
      const tmpDir = path.join(__dirname, 'tmp-web-cli-text');
      const fs = require('fs');
      if (!fs.existsSync(tmpDir)) {
        fs.mkdirSync(tmpDir, { recursive: true });
      }

      // Start CLI receiver
      const cliReceiver = spawn('./e2ecp', ['receive', roomName, '--server', wsUrl, '-o', tmpDir, '-f'], {
        cwd: path.join(__dirname, '..'),
      });

      let receiverOutput = '';
      let textReceived = false;

      cliReceiver.stdout.on('data', (data) => {
        receiverOutput += data.toString();
        console.log(`CLI Receiver: ${data}`);
        // Check if the text message was received
        if (data.toString().includes('Received text message')) {
          textReceived = true;
        }
      });

      cliReceiver.stderr.on('data', (data) => {
        console.error(`CLI Receiver Error: ${data}`);
      });

      // Wait for CLI receiver to connect
      await new Promise(resolve => setTimeout(resolve, 3000));

      // Navigate web sender to the room
      await page.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait for both clients to be connected (text input should be enabled)
      await page.waitForSelector('input[placeholder="TYPE TEXT TO SEND..."]:not([disabled])', { timeout: 15000 });

      // Extra wait to ensure connection is stable
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Web sender: Type text message
      const textInput = await page.locator('input[placeholder="TYPE TEXT TO SEND..."]');
      await textInput.fill(testMessage);

      // Web sender: Click send button
      await page.click('button:has-text("SEND")');

      // Wait for CLI receiver to process the message
      await new Promise(resolve => setTimeout(resolve, 3000));

      // Verify the message was received by CLI
      expect(textReceived).toBe(true);
      expect(receiverOutput).toContain(testMessage);

      // Clean up CLI receiver
      cliReceiver.kill('SIGTERM');

      // Wait for CLI receiver to close
      await new Promise((resolve) => {
        cliReceiver.on('close', (code) => {
          console.log(`CLI Receiver exited with code ${code}`);
          resolve();
        });
      });

      console.log('CLI Receiver output:', receiverOutput);

      // Clean up temp directory
      if (fs.existsSync(tmpDir)) {
        fs.rmSync(tmpDir, { recursive: true, force: true });
      }

    } finally {
      await page.close();
      await context.close();
    }
  });
});

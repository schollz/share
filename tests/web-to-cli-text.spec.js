// @ts-check
const { test, expect } = require('@playwright/test');
const path = require('path');
const { spawn } = require('child_process');

/**
 * Test: Web to CLI text transfer
 * 
 * This test verifies that text can be successfully transferred from a web client
 * to the command-line tool through the relay server.
 */
test.describe('Web to CLI Text Transfer', () => {
  let relayServer;
  let serverPort;
  let serverUrl;

  test.beforeAll(async () => {
    // Start the relay server
    serverPort = 8081 + Math.floor(Math.random() * 1000); // Random port to avoid conflicts
    
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

  test('should transfer text from web client to CLI receiver', async ({ browser }) => {
    // Test text to send
    const testText = 'Hello from web to CLI! This is a Playwright test for web-to-CLI text transfer.';

    const context = await browser.newContext();
    const page = await context.newPage();

    try {
      // Generate a unique room name
      const roomName = `test-web-cli-text-${Date.now()}`;

      // Create a temporary directory for receiver output
      const tmpDir = path.join(__dirname, 'cli-output-text');

      // Start CLI receiver in background
      const wsUrl = `ws://localhost:${serverPort}`;
      const cliReceiver = spawn('./e2ecp', ['receive', roomName, '--server', wsUrl, '--output', tmpDir, '--force'], {
        cwd: path.join(__dirname, '..'),
      });

      let receiverReady = false;
      let receiverOutput = '';
      let receivedText = '';

      cliReceiver.stdout.on('data', (data) => {
        const output = data.toString();
        receiverOutput += output;
        console.log(`CLI Receiver: ${output}`);
        
        // Mark as ready when connected
        if (output.includes('You are:') || output.includes('Connected')) {
          receiverReady = true;
        }
        
        // Extract received text between markers
        if (output.includes('=== Received Text ===')) {
          const lines = output.split('\n');
          for (let i = 0; i < lines.length; i++) {
            if (lines[i].includes('=== Received Text ===') && i + 1 < lines.length) {
              receivedText = lines[i + 1].trim();
              break;
            }
          }
        }
      });

      cliReceiver.stderr.on('data', (data) => {
        console.error(`CLI Receiver Error: ${data}`);
      });

      // Wait for receiver to be ready
      await new Promise(resolve => {
        const checkReady = setInterval(() => {
          if (receiverReady) {
            clearInterval(checkReady);
            resolve();
          }
        }, 100);
        setTimeout(() => {
          clearInterval(checkReady);
          resolve();
        }, 15000);
      });

      // Extra wait to ensure WebSocket connection is established
      await new Promise(resolve => setTimeout(resolve, 2000));

      // Navigate web sender to the room
      await page.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait for the text input to be enabled (indicates peer connection)
      await page.waitForSelector('input[type="text"]:not([disabled])', { timeout: 15000 });

      // Extra wait to ensure connection is stable
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Sender: Type and send the text
      const textInput = await page.locator('input[type="text"][placeholder*="message"]');
      await textInput.fill(testText);

      // Click the Send button
      await page.click('button:has-text("SEND")');

      // Wait for CLI receiver to process the text (give it time to output)
      await new Promise(resolve => setTimeout(resolve, 3000));

      // Verify the text was received
      expect(receivedText).toBe(testText);

      // Clean up
      cliReceiver.kill('SIGTERM');
      await new Promise(resolve => setTimeout(resolve, 1000));

    } finally {
      await page.close();
      await context.close();
    }
  });
});

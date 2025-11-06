// @ts-check
const { test, expect } = require('@playwright/test');
const path = require('path');
const { spawn } = require('child_process');

/**
 * Test: Web to Web text transfer
 * 
 * This test verifies that two browser clients can successfully transfer text
 * through the relay server using end-to-end encryption.
 */
test.describe('Web to Web Text Transfer', () => {
  let relayServer;
  let serverPort;
  let serverUrl;

  test.beforeAll(async () => {
    // Start the relay server
    serverPort = 8080 + Math.floor(Math.random() * 1000); // Random port to avoid conflicts
    
    return new Promise((resolve, reject) => {
      relayServer = spawn('./e2ecp', ['serve', '--port', serverPort.toString(), '--db-path', ''], {
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

  test('should transfer text from one web client to another', async ({ browser }) => {
    // Test text to send
    const testText = 'Hello from Playwright! This is a test text message for web-to-web transfer.';

    // Create two browser contexts (sender and receiver) with clipboard permissions
    const senderContext = await browser.newContext({
      permissions: ['clipboard-read', 'clipboard-write']
    });
    const receiverContext = await browser.newContext({
      permissions: ['clipboard-read', 'clipboard-write']
    });

    const senderPage = await senderContext.newPage();
    const receiverPage = await receiverContext.newPage();

    try {
      // Generate a unique room name
      const roomName = `test-room-${Date.now()}`;

      // Navigate receiver first and wait for it to be ready
      await receiverPage.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });
      
      // Wait a bit for receiver's WebSocket connection to establish
      await new Promise(resolve => setTimeout(resolve, 2000));
      
      // Now navigate sender
      await senderPage.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait for WebSocket connections to be established (both peers need to be connected)
      // Look for the text input to be enabled on sender (indicates peer connection)
      await senderPage.waitForSelector('input[type="text"]:not([disabled])', { timeout: 15000 });
      
      // Extra wait to ensure connection is stable
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Sender: Type and send the text
      const textInput = await senderPage.locator('input[type="text"][placeholder*="message"]');
      await textInput.fill(testText);
      
      // Click the Send button
      await senderPage.click('button:has-text("SEND")');

      // Receiver: Wait for the text message modal to appear
      await receiverPage.waitForSelector('text=RECEIVED TEXT', { timeout: 30000 });
      
      // Get the text content from the modal
      const receivedText = await receiverPage.locator('.bg-gray-200 .font-bold.break-words').textContent();
      
      // Verify the received text matches what was sent
      expect(receivedText.trim()).toBe(testText);

      // Test copy button functionality
      await receiverPage.click('button[title="Copy to clipboard"]');

      // Verify clipboard content directly (more reliable than checking toast)
      const clipboardText = await receiverPage.evaluate(async () => {
        return await navigator.clipboard.readText();
      });
      expect(clipboardText).toBe(testText);

      // Close the modal
      await receiverPage.click('button:has-text("Close")');

    } finally {
      await senderPage.close();
      await receiverPage.close();
      await senderContext.close();
      await receiverContext.close();
    }
  });
});

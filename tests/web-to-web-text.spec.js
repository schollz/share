// @ts-check
const { test, expect } = require('@playwright/test');
const path = require('path');
const { spawn } = require('child_process');

/**
 * Test: Web to Web text message transfer
 *
 * This test verifies that two browser clients can successfully send and receive
 * text messages through the relay server using end-to-end encryption.
 */
test.describe('Web to Web Text Messaging', () => {
  let relayServer;
  let serverPort;
  let serverUrl;

  test.beforeAll(async () => {
    // Start the relay server
    serverPort = 8080 + Math.floor(Math.random() * 1000); // Random port to avoid conflicts

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

  test('should send text message from one web client to another', async ({ browser }) => {
    const testMessage = 'Hello from Playwright! This is a test text message.';

    // Create two browser contexts (sender and receiver)
    const senderContext = await browser.newContext();
    const receiverContext = await browser.newContext();

    const senderPage = await senderContext.newPage();
    const receiverPage = await receiverContext.newPage();

    try {
      // Generate a unique room name
      const roomName = `text-test-room-${Date.now()}`;

      // Navigate receiver first and wait for it to be ready
      await receiverPage.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait a bit for receiver's WebSocket connection to establish
      await new Promise(resolve => setTimeout(resolve, 2000));

      // Now navigate sender
      await senderPage.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait for both clients to be connected (text input should be enabled)
      await senderPage.waitForSelector('input[placeholder="TYPE TEXT TO SEND..."]:not([disabled])', { timeout: 15000 });
      await receiverPage.waitForSelector('input[placeholder="TYPE TEXT TO SEND..."]:not([disabled])', { timeout: 15000 });

      // Extra wait to ensure connection is stable
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Sender: Type text message
      const textInput = await senderPage.locator('input[placeholder="TYPE TEXT TO SEND..."]');
      await textInput.fill(testMessage);

      // Sender: Click send button
      await senderPage.click('button:has-text("SEND")');

      // Receiver: Wait for the text message to appear
      await receiverPage.waitForSelector(`text=${testMessage}`, { timeout: 10000 });

      // Verify the message is displayed with copy button
      const messageBox = await receiverPage.locator('.bg-white.border-2').filter({ hasText: testMessage });
      await expect(messageBox).toBeVisible();

      // Verify copy button exists
      const copyButton = await messageBox.locator('button i.fa-copy');
      await expect(copyButton).toBeVisible();

      // Verify the message content
      const messageText = await messageBox.locator('div.whitespace-pre-wrap').textContent();
      expect(messageText.trim()).toBe(testMessage);

      // Test copy functionality
      await messageBox.locator('button').click();

      // Wait a bit for clipboard operation
      await new Promise(resolve => setTimeout(resolve, 500));

    } finally {
      await senderPage.close();
      await receiverPage.close();
      await senderContext.close();
      await receiverContext.close();
    }
  });

  test('should send text message with Enter key', async ({ browser }) => {
    const testMessage = 'Testing Enter key to send!';

    // Create two browser contexts (sender and receiver)
    const senderContext = await browser.newContext();
    const receiverContext = await browser.newContext();

    const senderPage = await senderContext.newPage();
    const receiverPage = await receiverContext.newPage();

    try {
      // Generate a unique room name
      const roomName = `text-enter-test-${Date.now()}`;

      // Navigate receiver first and wait for it to be ready
      await receiverPage.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait a bit for receiver's WebSocket connection to establish
      await new Promise(resolve => setTimeout(resolve, 2000));

      // Now navigate sender
      await senderPage.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait for both clients to be connected
      await senderPage.waitForSelector('input[placeholder="TYPE TEXT TO SEND..."]:not([disabled])', { timeout: 15000 });

      // Extra wait to ensure connection is stable
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Sender: Type text message and press Enter
      const textInput = await senderPage.locator('input[placeholder="TYPE TEXT TO SEND..."]');
      await textInput.fill(testMessage);
      await textInput.press('Enter');

      // Receiver: Wait for the text message to appear
      await receiverPage.waitForSelector(`text=${testMessage}`, { timeout: 10000 });

      // Verify the message is displayed
      const messageBox = await receiverPage.locator('.bg-white.border-2').filter({ hasText: testMessage });
      await expect(messageBox).toBeVisible();

    } finally {
      await senderPage.close();
      await receiverPage.close();
      await senderContext.close();
      await receiverContext.close();
    }
  });
});

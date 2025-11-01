// @ts-check
const { test, expect } = require('@playwright/test');
const path = require('path');
const fs = require('fs');
const { spawn } = require('child_process');

/**
 * Test: Web to Web file transfer
 * 
 * This test verifies that two browser clients can successfully transfer a file
 * through the relay server using end-to-end encryption.
 */
test.describe('Web to Web Transfer', () => {
  let relayServer;
  let serverPort;
  let serverUrl;

  test.beforeAll(async () => {
    // Start the relay server
    serverPort = 8080 + Math.floor(Math.random() * 1000); // Random port to avoid conflicts
    
    return new Promise((resolve, reject) => {
      relayServer = spawn('./share', ['serve', '--port', serverPort.toString()], {
        cwd: path.join(__dirname, '..'),
      });

      let started = false;

      relayServer.stdout.on('data', (data) => {
        console.log(`Server: ${data}`);
        if (!started && data.toString().includes('Starting')) {
          started = true;
          serverUrl = `http://localhost:${serverPort}`;
          setTimeout(resolve, 1000); // Give it a moment to fully start
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
      }, 3000);
    });
  });

  test.afterAll(async () => {
    // Stop the relay server
    if (relayServer) {
      relayServer.kill();
    }
  });

  test('should transfer a file from one web client to another', async ({ browser }) => {
    // Create a test file
    const testFilePath = path.join(__dirname, 'test-web-to-web.txt');
    const testContent = 'Hello from Playwright! This is a test file for web-to-web transfer.';
    fs.writeFileSync(testFilePath, testContent);

    // Create two browser contexts (sender and receiver)
    const senderContext = await browser.newContext();
    const receiverContext = await browser.newContext();

    const senderPage = await senderContext.newPage();
    const receiverPage = await receiverContext.newPage();

    try {
      // Generate a unique room name
      const roomName = `test-room-${Date.now()}`;

      // Navigate both pages to the server with the room name
      await receiverPage.goto(`${serverUrl}/${roomName}`);
      await senderPage.goto(`${serverUrl}/${roomName}`);

      // Wait for pages to load
      await receiverPage.waitForLoadState('networkidle');
      await senderPage.waitForLoadState('networkidle');

      // Set up download handler for receiver
      const downloadPromise = receiverPage.waitForEvent('download');

      // Sender: Upload the file
      // Find the file input and set the file
      const fileInput = await senderPage.locator('input[type="file"]').first();
      await fileInput.setInputFiles(testFilePath);

      // Wait for the file to be processed and transfer to start
      await senderPage.waitForTimeout(1000);

      // Look for send button or automatic sending
      // The app might auto-send once a file is selected
      const sendButton = senderPage.locator('button:has-text("Send")');
      const sendButtonCount = await sendButton.count();
      if (sendButtonCount > 0) {
        await sendButton.click();
      }

      // Receiver: Wait for download to start and complete
      const download = await downloadPromise;
      
      // Save the downloaded file
      const downloadPath = path.join(__dirname, 'downloaded-web-to-web.txt');
      await download.saveAs(downloadPath);

      // Verify the downloaded file content
      const downloadedContent = fs.readFileSync(downloadPath, 'utf-8');
      expect(downloadedContent).toBe(testContent);

      // Clean up test files
      fs.unlinkSync(testFilePath);
      fs.unlinkSync(downloadPath);

    } finally {
      await senderPage.close();
      await receiverPage.close();
      await senderContext.close();
      await receiverContext.close();
    }
  });
});

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

      // Navigate receiver first and wait for it to be ready
      await receiverPage.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });
      
      // Wait a bit for receiver's WebSocket connection to establish
      await new Promise(resolve => setTimeout(resolve, 2000));
      
      // Now navigate sender
      await senderPage.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait for WebSocket connections to be established (both peers need to be connected)
      // Look for the "CLICK OR DROP FILES HERE" button to be enabled on sender (indicates peer connection)
      await senderPage.waitForSelector('button:has-text("CLICK OR DROP FILES HERE"):not([disabled])', { timeout: 15000 });
      
      // Extra wait to ensure connection is stable
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Set up download handler for receiver before sending
      const downloadPromise = receiverPage.waitForEvent('download', { timeout: 60000 });

      // Sender: Upload the file
      // Find the hidden file input and set the file
      const fileInput = await senderPage.locator('input[type="file"]');
      await fileInput.setInputFiles(testFilePath);

      // The app automatically sends the file once selected

      // Receiver: Wait for the download confirmation modal to appear
      await receiverPage.waitForSelector('text=DOWNLOAD FILE?', { timeout: 30000 });
      
      // Click the Download button in the confirmation modal
      await receiverPage.click('button:has-text("Download")');

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

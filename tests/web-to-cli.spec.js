// @ts-check
const { test, expect } = require('@playwright/test');
const path = require('path');
const fs = require('fs');
const { spawn } = require('child_process');

/**
 * Test: Web to CLI file transfer
 * 
 * This test verifies that a file can be successfully transferred from a web client
 * to the command-line tool through the relay server.
 */
test.describe('Web to CLI Transfer', () => {
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

  test('should transfer a file from web client to CLI receiver', async ({ browser }) => {
    // Create a test file
    const testFilePath = path.join(__dirname, 'test-web-to-cli.txt');
    const testContent = 'Hello from web to CLI! This is a Playwright test for web-to-CLI transfer.';
    fs.writeFileSync(testFilePath, testContent);

    // Create output directory for CLI receiver
    const outputDir = path.join(__dirname, 'cli-output');
    if (!fs.existsSync(outputDir)) {
      fs.mkdirSync(outputDir, { recursive: true });
    }

    const context = await browser.newContext();
    const page = await context.newPage();

    try {
      // Generate a unique room name
      const roomName = `test-web-cli-${Date.now()}`;

      // Start CLI receiver in background
      const wsUrl = `ws://localhost:${serverPort}`;
      const cliReceiver = spawn('./e2ecp', ['receive', roomName, '--server', wsUrl, '--output', outputDir, '--force'], {
        cwd: path.join(__dirname, '..'),
      });

      let receiverReady = false;
      let receiverOutput = '';

      cliReceiver.stdout.on('data', (data) => {
        const output = data.toString();
        receiverOutput += output;
        console.log(`CLI Receiver: ${output}`);
        if (output.includes('Waiting') || output.includes('Connected')) {
          receiverReady = true;
        }
      });

      cliReceiver.stderr.on('data', (data) => {
        console.error(`CLI Receiver Error: ${data}`);
      });

      // Wait longer for receiver to be ready and establish WebSocket connection
      await new Promise(resolve => setTimeout(resolve, 4000));

      // Navigate web sender to the room
      await page.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait for connection to be established (CLICK OR DROP FILES HERE button enabled)
      await page.waitForSelector('button:has-text("CLICK OR DROP FILES HERE"):not([disabled])', { timeout: 15000 });
      
      // Extra wait to ensure connection is stable
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Sender: Upload the file
      const fileInput = await page.locator('input[type="file"]');
      await fileInput.setInputFiles(testFilePath);

      // File is automatically sent once selected

      // Wait for transfer to complete with longer timeout
      await new Promise((resolve) => {
        let resolved = false;
        const checkInterval = setInterval(() => {
          if (receiverOutput.includes('Saved:') || receiverOutput.includes('received')) {
            if (!resolved) {
              resolved = true;
              clearInterval(checkInterval);
              resolve();
            }
          }
        }, 500);

        // Timeout after 30 seconds
        setTimeout(() => {
          if (!resolved) {
            resolved = true;
            clearInterval(checkInterval);
            resolve();
          }
        }, 30000);
      });

      // Give it extra time to finish writing
      await page.waitForTimeout(2000);

      // Kill the receiver process
      cliReceiver.kill('SIGTERM');
      
      // Wait for process to exit
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Verify the file was received by CLI
      const receivedFilePath = path.join(outputDir, 'test-web-to-cli.txt');
      
      // Check if file exists
      expect(fs.existsSync(receivedFilePath)).toBe(true);

      if (fs.existsSync(receivedFilePath)) {
        const receivedContent = fs.readFileSync(receivedFilePath, 'utf-8');
        expect(receivedContent).toBe(testContent);
      }

      // Clean up
      fs.unlinkSync(testFilePath);
      if (fs.existsSync(receivedFilePath)) {
        fs.unlinkSync(receivedFilePath);
      }
      if (fs.existsSync(outputDir)) {
        fs.rmSync(outputDir, { recursive: true });
      }

    } finally {
      await page.close();
      await context.close();
    }
  });

  test('should transfer a file from CLI sender to web receiver', async ({ browser }) => {
    // Create a test file
    const testFilePath = path.join(__dirname, 'test-cli-to-web.txt');
    const testContent = 'Hello from CLI to web! This is a Playwright test for CLI-to-web transfer.';
    fs.writeFileSync(testFilePath, testContent);

    const context = await browser.newContext();
    const page = await context.newPage();

    try {
      // Generate a unique room name
      const roomName = `test-cli-web-${Date.now()}`;

      // Navigate web receiver to the room first
      await page.goto(`${serverUrl}/${roomName}`, { waitUntil: 'networkidle', timeout: 30000 });

      // Wait for the page to be ready (CLICK OR DROP FILES HERE button visible means ready to receive)
      await page.waitForSelector('button:has-text("CLICK OR DROP FILES HERE"), div:has-text("WAITING FOR PEER")', { timeout: 15000 });
      
      // Extra wait to ensure WebSocket connection is established
      await new Promise(resolve => setTimeout(resolve, 2000));

      // Set up download handler for receiver with longer timeout
      const downloadPromise = page.waitForEvent('download', { timeout: 60000 });

      // Start CLI sender
      const wsUrl = `ws://localhost:${serverPort}`;
      const cliSender = spawn('./e2ecp', ['send', testFilePath, roomName, '--server', wsUrl], {
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

      // Wait for the download confirmation modal to appear
      await page.waitForSelector('text=DOWNLOAD FILE?', { timeout: 30000 });

      // Wait for the Download button to be visible and clickable
      await page.waitForSelector('button.bg-black:has-text("Download")', { state: 'visible', timeout: 5000 });
      await new Promise(resolve => setTimeout(resolve, 500));

      // Click the Download button in the confirmation modal
      await page.click('button.bg-black:has-text("Download")');

      // Wait for download to complete
      const download = await downloadPromise;
      
      // Save the downloaded file
      const downloadPath = path.join(__dirname, 'downloaded-cli-to-web.txt');
      await download.saveAs(downloadPath);

      // Verify the downloaded file content
      const downloadedContent = fs.readFileSync(downloadPath, 'utf-8');
      expect(downloadedContent).toBe(testContent);

      // Clean up
      cliSender.kill('SIGTERM');
      await new Promise(resolve => setTimeout(resolve, 1000));
      fs.unlinkSync(testFilePath);
      fs.unlinkSync(downloadPath);

    } finally {
      await page.close();
      await context.close();
    }
  });
});


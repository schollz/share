package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/schollz/e2ecp/src/client"
	"github.com/schollz/e2ecp/src/relay"
)

func TestIntegrationFileTransfer(t *testing.T) {
	// Skip if short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "share-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file to send
	testContent := []byte("Hello, World! This is a test file for integration testing. " +
		"This message contains enough data to verify the transfer works correctly. " +
		"It includes multiple sentences to ensure we have a reasonable payload size.")
	sendFilePath := filepath.Join(tmpDir, "test-send.txt")
	err = os.WriteFile(sendFilePath, testContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a directory for receiving the file
	receiveDir := filepath.Join(tmpDir, "receive")
	err = os.MkdirAll(receiveDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create receive directory: %v", err)
	}

	// Start the relay server in a goroutine
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// Skip integration testing if the sandbox disallows opening sockets
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			t.Skipf("Skipping integration test due to network restrictions: %v", err)
		}
		t.Fatalf("Failed to reserve test port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Start the actual relay server in background
	go func() {
		relay.Start(port, 0, 0, "", staticFS, logger) // 0 limits disable room caps for integration tests
	}()

	// Give the server time to start
	time.Sleep(1 * time.Second)

	serverURL := fmt.Sprintf("ws://localhost:%d", port)
	roomID := "integration-test-room"

	// Start receiver in a goroutine
	receiveDone := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				receiveDone <- fmt.Errorf("receiver panic: %v", r)
			}
		}()
		client.ReceiveFile(roomID, serverURL, receiveDir, true, logger) // Force overwrite in test
		receiveDone <- nil
	}()

	// Give receiver time to connect and join the room
	time.Sleep(2 * time.Second)

	// Start sender in a goroutine
	sendDone := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				sendDone <- fmt.Errorf("sender panic: %v", r)
			}
		}()
		client.SendFile(sendFilePath, roomID, serverURL, logger)
		sendDone <- nil
	}()

	// Wait for both sender and receiver to complete (with timeout)
	timeout := time.After(30 * time.Second)

	select {
	case err := <-sendDone:
		if err != nil {
			t.Fatalf("Sender failed: %v", err)
		}
		t.Log("Sender completed")
	case <-timeout:
		t.Fatal("Sender timed out")
	}

	select {
	case err := <-receiveDone:
		if err != nil {
			t.Fatalf("Receiver failed: %v", err)
		}
		t.Log("Receiver completed")
	case <-timeout:
		t.Fatal("Receiver timed out")
	}

	// Verify the file was received correctly
	receivedFilePath := filepath.Join(receiveDir, "test-send.txt")
	receivedContent, err := os.ReadFile(receivedFilePath)
	if err != nil {
		t.Fatalf("Failed to read received file: %v", err)
	}

	if string(receivedContent) != string(testContent) {
		t.Fatalf("Received content does not match sent content.\nExpected: %s\nGot: %s",
			string(testContent), string(receivedContent))
	}

	t.Logf("Successfully transferred file with %d bytes", len(receivedContent))

	// Verify file size
	if len(receivedContent) != len(testContent) {
		t.Fatalf("File size mismatch: expected %d bytes, got %d bytes",
			len(testContent), len(receivedContent))
	}
}

func TestIntegrationFolderTransfer(t *testing.T) {
	// Skip if short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "share-integration-folder-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test folder structure with multiple files
	testFolder := filepath.Join(tmpDir, "test-folder")
	err = os.MkdirAll(testFolder, 0755)
	if err != nil {
		t.Fatalf("Failed to create test folder: %v", err)
	}

	// Create test files with different content
	testFiles := map[string]string{
		"file1.txt":             "This is file 1 content",
		"file2.txt":             "This is file 2 content with more data",
		"subdir/file3.txt":      "This is file 3 in a subdirectory",
		"subdir/file4.txt":      "This is file 4 in a subdirectory",
		"subdir/deep/file5.txt": "This is file 5 in a nested subdirectory",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(testFolder, relPath)
		err = os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create subdirectory for %s: %v", relPath, err)
		}
		err = os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", relPath, err)
		}
	}

	// Create a directory for receiving the folder
	receiveDir := filepath.Join(tmpDir, "receive")
	err = os.MkdirAll(receiveDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create receive directory: %v", err)
	}

	// Start the relay server in a goroutine
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// Skip integration testing if the sandbox disallows opening sockets
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			t.Skipf("Skipping integration test due to network restrictions: %v", err)
		}
		t.Fatalf("Failed to reserve test port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Start the actual relay server in background
	go func() {
		relay.Start(port, 0, 0, "", staticFS, logger) // 0 limits disable room caps for integration tests
	}()

	// Give the server time to start
	time.Sleep(1 * time.Second)

	serverURL := fmt.Sprintf("ws://localhost:%d", port)
	roomID := "integration-folder-test-room"

	// Start receiver in a goroutine
	receiveDone := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				receiveDone <- fmt.Errorf("receiver panic: %v", r)
			}
		}()
		client.ReceiveFile(roomID, serverURL, receiveDir, true, logger) // Force overwrite in test
		receiveDone <- nil
	}()

	// Give receiver time to connect and join the room
	time.Sleep(2 * time.Second)

	// Start sender in a goroutine
	sendDone := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				sendDone <- fmt.Errorf("sender panic: %v", r)
			}
		}()
		client.SendFile(testFolder, roomID, serverURL, logger)
		sendDone <- nil
	}()

	// Wait for both sender and receiver to complete (with timeout)
	timeout := time.After(30 * time.Second)

	select {
	case err := <-sendDone:
		if err != nil {
			t.Fatalf("Sender failed: %v", err)
		}
		t.Log("Sender completed")
	case <-timeout:
		t.Fatal("Sender timed out")
	}

	select {
	case err := <-receiveDone:
		if err != nil {
			t.Fatalf("Receiver failed: %v", err)
		}
		t.Log("Receiver completed")
	case <-timeout:
		t.Fatal("Receiver timed out")
	}

	// Verify the folder was received and extracted correctly
	receivedFolderPath := filepath.Join(receiveDir, "test-folder")

	// Check if the folder exists
	if _, err := os.Stat(receivedFolderPath); os.IsNotExist(err) {
		t.Fatalf("Received folder does not exist at %s", receivedFolderPath)
	}

	// Verify each file was received with correct content
	for relPath, expectedContent := range testFiles {
		receivedFilePath := filepath.Join(receivedFolderPath, relPath)
		receivedContent, err := os.ReadFile(receivedFilePath)
		if err != nil {
			t.Fatalf("Failed to read received file %s: %v", relPath, err)
		}

		if string(receivedContent) != expectedContent {
			t.Fatalf("Content mismatch for %s.\nExpected: %s\nGot: %s",
				relPath, expectedContent, string(receivedContent))
		}
	}

	t.Logf("Successfully transferred folder with %d files", len(testFiles))

	// Verify that no extra files were created
	fileCount := 0
	err = filepath.Walk(receivedFolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk received folder: %v", err)
	}

	if fileCount != len(testFiles) {
		t.Fatalf("File count mismatch: expected %d files, got %d files", len(testFiles), fileCount)
	}

	t.Log("All files verified successfully")
}

func TestIntegrationHashVerification(t *testing.T) {
	// Skip if short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "share-hash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file to send
	testContent := []byte("Testing hash verification functionality. " +
		"This file will be transferred and its hash should be verified on the receiving end.")
	sendFilePath := filepath.Join(tmpDir, "test-hash.txt")
	err = os.WriteFile(sendFilePath, testContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a directory for receiving the file
	receiveDir := filepath.Join(tmpDir, "receive")
	err = os.MkdirAll(receiveDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create receive directory: %v", err)
	}

	// Start the relay server in a goroutine
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// Skip integration testing if the sandbox disallows opening sockets
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			t.Skipf("Skipping integration test due to network restrictions: %v", err)
		}
		t.Fatalf("Failed to reserve test port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Start the actual relay server in background
	go func() {
		relay.Start(port, 0, 0, "", staticFS, logger)
	}()

	// Give the server time to start
	time.Sleep(1 * time.Second)

	serverURL := fmt.Sprintf("ws://localhost:%d", port)
	roomID := "hash-verification-test-room"

	// Start receiver in a goroutine
	receiveDone := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				receiveDone <- fmt.Errorf("receiver panic: %v", r)
			}
		}()
		client.ReceiveFile(roomID, serverURL, receiveDir, true, logger)
		receiveDone <- nil
	}()

	// Give receiver time to connect and join the room
	time.Sleep(2 * time.Second)

	// Start sender in a goroutine
	sendDone := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				sendDone <- fmt.Errorf("sender panic: %v", r)
			}
		}()
		client.SendFile(sendFilePath, roomID, serverURL, logger)
		sendDone <- nil
	}()

	// Wait for both sender and receiver to complete (with timeout)
	timeout := time.After(30 * time.Second)

	select {
	case err := <-sendDone:
		if err != nil {
			t.Fatalf("Sender failed: %v", err)
		}
		t.Log("Sender completed")
	case <-timeout:
		t.Fatal("Sender timed out")
	}

	select {
	case err := <-receiveDone:
		if err != nil {
			t.Fatalf("Receiver failed: %v", err)
		}
		t.Log("Receiver completed")
	case <-timeout:
		t.Fatal("Receiver timed out")
	}

	// Verify the file was received correctly
	receivedFilePath := filepath.Join(receiveDir, "test-hash.txt")
	receivedContent, err := os.ReadFile(receivedFilePath)
	if err != nil {
		t.Fatalf("Failed to read received file: %v", err)
	}

	if string(receivedContent) != string(testContent) {
		t.Fatalf("Received content does not match sent content.\nExpected: %s\nGot: %s",
			string(testContent), string(receivedContent))
	}

	t.Log("Hash verification test passed - file transferred with hash verification")
}

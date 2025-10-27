package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/schollz/share/src/client"
	"github.com/schollz/share/src/relay"
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
	port := 13001 // Use a non-standard port for testing
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Start the actual relay server in background
	go func() {
		relay.Start(port, staticFS, logger)
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
		client.ReceiveFile(roomID, serverURL, receiveDir)
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
		client.SendFile(sendFilePath, roomID, serverURL)
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

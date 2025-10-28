package client

import (
	"crypto/ecdh"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/schollz/share/src/crypto"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// formatBytes formats bytes into human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// sanitizeFileName cleans a filename to prevent path traversal attacks
// by extracting only the base filename and removing any directory components
func sanitizeFileName(fileName string) string {
	// Use filepath.Base to extract only the filename, removing any directory components
	// This prevents path traversal attacks like "../../../etc/passwd"
	return filepath.Base(fileName)
}

// ReceiveFile receives a file from the specified room via the relay server
func ReceiveFile(roomID, serverURL, outputDir string) {
	clientID := uuid.New().String()

	privKey, err := crypto.GenerateECDHKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate key pair: %v", err)
	}

	u, _ := url.Parse(serverURL)
	u.Path = "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	joinMsg := map[string]interface{}{
		"type":     "join",
		"roomId":   roomID,
		"clientId": clientID,
	}
	sendProtobufMessage(conn, joinMsg)

	var sharedSecret []byte
	var fileName string
	var totalSize int64
	var receivedBytes int64
	var bar *progressbar.ProgressBar
	var outputFile *os.File

	sendPublicKey := func() {
		pubKeyBytes := privKey.PublicKey().Bytes()
		pubKeyMsg := map[string]interface{}{
			"type": "pubkey",
			"pub":  base64.StdEncoding.EncodeToString(pubKeyBytes),
		}
		sendProtobufMessage(conn, pubKeyMsg)
	}

	for {
		msg, err := receiveProtobufMessage(conn)
		if err != nil {
			log.Fatalf("Connection closed: %v", err)
		}

		switch msg.Type {
		case "error":
			if msg.Error != "" {
				log.Fatalf("Server error: %s", msg.Error)
			}
			return

		case "joined":
			sendPublicKey()

		case "peers":
			// When a new peer joins, re-send our public key
			if msg.Count == 2 {
				sendPublicKey()
			}

		case "pubkey":
			peerPubBytes, _ := base64.StdEncoding.DecodeString(msg.Pub)
			peerPubKey, err := ecdh.P256().NewPublicKey(peerPubBytes)
			if err != nil {
				log.Fatalf("Failed to parse peer public key: %v", err)
			}

			sharedSecret, err = crypto.DeriveSharedSecret(privKey, peerPubKey)
			if err != nil {
				log.Fatalf("Failed to derive shared secret: %v", err)
			}

		case "file_start":
			if sharedSecret == nil {
				continue
			}

			fileName = sanitizeFileName(msg.Name)
			totalSize = msg.TotalSize
			receivedBytes = 0

			// Create output file for streaming
			outputPath := filepath.Join(outputDir, fileName)
			outputFile, err = os.Create(outputPath)
			if err != nil {
				log.Fatalf("Failed to create output file: %v", err)
			}

			// Create progress bar for receiving
			bar = progressbar.NewOptions64(
				totalSize,
				progressbar.OptionSetDescription("Receiving"),
				progressbar.OptionSetWriter(os.Stderr),
				progressbar.OptionShowBytes(true),
				progressbar.OptionSetWidth(10),
				progressbar.OptionThrottle(65*time.Millisecond),
				progressbar.OptionShowCount(),
				progressbar.OptionOnCompletion(func() {
					fmt.Fprint(os.Stderr, "\n")
				}),
				progressbar.OptionSpinnerType(14),
				progressbar.OptionFullWidth(),
			)

		case "file_chunk":
			if bar == nil || outputFile == nil {
				continue
			}

			// Decrypt this chunk with its own IV
			chunkIV, _ := base64.StdEncoding.DecodeString(msg.IvB64)
			cipherChunk, _ := base64.StdEncoding.DecodeString(msg.ChunkData)

			plainChunk, err := crypto.DecryptAESGCM(sharedSecret, chunkIV, cipherChunk)
			if err != nil {
				log.Fatalf("Failed to decrypt chunk: %v", err)
			}

			// Write decrypted chunk directly to file
			n, err := outputFile.Write(plainChunk)
			if err != nil {
				log.Fatalf("Failed to write to file: %v", err)
			}

			receivedBytes += int64(n)
			bar.Add(n)

		case "file_end":
			if bar == nil || outputFile == nil {
				continue
			}

			bar.Finish()
			outputFile.Close()

			outputPath := filepath.Join(outputDir, fileName)
			fmt.Printf("Saved: %s (%s)\n", outputPath, formatBytes(totalSize))
			return

		case "file":
			// Backward compatibility: handle old-style single-message transfers
			if sharedSecret == nil {
				continue
			}

			bar := progressbar.NewOptions64(
				msg.Size,
				progressbar.OptionSetDescription("Receiving"),
				progressbar.OptionSetWriter(os.Stderr),
				progressbar.OptionShowBytes(true),
				progressbar.OptionSetWidth(10),
				progressbar.OptionThrottle(65*time.Millisecond),
				progressbar.OptionShowCount(),
				progressbar.OptionOnCompletion(func() {
					fmt.Fprint(os.Stderr, "\n")
				}),
				progressbar.OptionSpinnerType(14),
				progressbar.OptionFullWidth(),
			)

			iv, _ := base64.StdEncoding.DecodeString(msg.IvB64)
			ciphertext, _ := base64.StdEncoding.DecodeString(msg.DataB64)

			bar.Add64(msg.Size)
			bar.Finish()

			plaintext, err := crypto.DecryptAESGCM(sharedSecret, iv, ciphertext)
			if err != nil {
				log.Fatalf("Decryption failed: %v", err)
			}

			outputPath := filepath.Join(outputDir, sanitizeFileName(msg.Name))
			err = os.WriteFile(outputPath, plaintext, 0644)
			if err != nil {
				log.Fatalf("Failed to write file: %v", err)
			}

			fmt.Printf("Saved: %s (%s)\n", outputPath, formatBytes(int64(len(plaintext))))
			return
		}
	}
}

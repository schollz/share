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
	"github.com/schollz/share/src/relay"

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
	conn.WriteJSON(joinMsg)

	var sharedSecret []byte
	var fileChunks [][]byte
	var fileName string
	var fileIV []byte
	var totalSize int64
	var receivedBytes int64
	var bar *progressbar.ProgressBar

	sendPublicKey := func() {
		pubKeyBytes := privKey.PublicKey().Bytes()
		pubKeyMsg := map[string]interface{}{
			"type": "pubkey",
			"pub":  base64.StdEncoding.EncodeToString(pubKeyBytes),
		}
		conn.WriteJSON(pubKeyMsg)
	}

	for {
		var msg relay.OutgoingMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Fatalf("Connection closed: %v", err)
		}

		switch msg.Type {
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

			fileName = msg.Name
			totalSize = msg.TotalSize
			fileIV, _ = base64.StdEncoding.DecodeString(msg.IvB64)
			fileChunks = make([][]byte, 0)
			receivedBytes = 0

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
			if bar == nil {
				continue
			}

			chunkData, _ := base64.StdEncoding.DecodeString(msg.ChunkData)
			fileChunks = append(fileChunks, chunkData)
			receivedBytes += int64(len(chunkData))
			bar.Add(len(chunkData))

		case "file_end":
			if bar == nil {
				continue
			}

			bar.Finish()

			// Reassemble the ciphertext
			totalCipherLen := 0
			for _, chunk := range fileChunks {
				totalCipherLen += len(chunk)
			}

			ciphertext := make([]byte, 0, totalCipherLen)
			for _, chunk := range fileChunks {
				ciphertext = append(ciphertext, chunk...)
			}

			plaintext, err := crypto.DecryptAESGCM(sharedSecret, fileIV, ciphertext)
			if err != nil {
				log.Fatalf("Decryption failed: %v", err)
			}

			outputPath := filepath.Join(outputDir, fileName)
			err = os.WriteFile(outputPath, plaintext, 0644)
			if err != nil {
				log.Fatalf("Failed to write file: %v", err)
			}

			fmt.Printf("Saved: %s (%s)\n", outputPath, formatBytes(int64(len(plaintext))))
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

			outputPath := filepath.Join(outputDir, msg.Name)
			err = os.WriteFile(outputPath, plaintext, 0644)
			if err != nil {
				log.Fatalf("Failed to write file: %v", err)
			}

			fmt.Printf("Saved: %s (%s)\n", outputPath, formatBytes(int64(len(plaintext))))
			return
		}
	}
}

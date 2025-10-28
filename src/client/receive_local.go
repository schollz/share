package client

import (
	"bufio"
	"crypto/ecdh"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
	"github.com/schollz/share/src/crypto"
)

// ReceiveFileWithLocalRelay receives a file with local relay support
func ReceiveFileWithLocalRelay(roomID, serverURL, outputDir string, forceOverwrite bool) {
	clientID := uuid.New().String()

	privKey, err := crypto.GenerateECDHKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate key pair: %v", err)
	}

	// Setup connections (both local and internet)
	cm, err := SetupConnections(roomID, serverURL, true)
	if err != nil {
		log.Fatalf("Failed to setup connections: %v", err)
	}
	defer cm.Close()

	// Join the room
	joinMsg := map[string]interface{}{
		"type":     "join",
		"roomId":   roomID,
		"clientId": clientID,
	}
	if err := cm.SendMessage(joinMsg); err != nil {
		log.Fatalf("Failed to join room: %v", err)
	}

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
		cm.SendMessage(pubKeyMsg)
	}

	for {
		msg, err := cm.ReceiveMessage()
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

			// Check if file exists and prompt for overwrite if needed
			if !checkFileOverwrite(outputPath, forceOverwrite) {
				return
			}

			outputFile, err = os.Create(outputPath)
			if err != nil {
				log.Fatalf("Failed to create output file: %v", err)
			}

			connType := cm.GetPreferredConnectionType()
			connDesc := "Receiving"
			if connType == ConnectionTypeLocal {
				connDesc = "Receiving (local network)"
			}

			// Create progress bar for receiving
			bar = progressbar.NewOptions64(
				totalSize,
				progressbar.OptionSetDescription(connDesc),
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

			connType := cm.GetPreferredConnectionType()
			connTypeStr := ""
			if connType == ConnectionTypeLocal {
				connTypeStr = " via local network"
			}

			fmt.Printf("Saved: %s (%s)%s\n", outputPath, formatBytes(totalSize), connTypeStr)
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
			// Check if file exists and prompt for overwrite if needed
			if !checkFileOverwrite(outputPath, forceOverwrite) {
				return
			}

			err = os.WriteFile(outputPath, plaintext, 0644)
			if err != nil {
				log.Fatalf("Failed to write file: %v", err)
			}

			fmt.Printf("Saved: %s (%s)\n", outputPath, formatBytes(int64(len(plaintext))))
			return
		}
	}
}

// promptOverwriteLocal is a duplicate of promptOverwrite for this file
func promptOverwriteLocal(filePath string) bool {
	fmt.Printf("File '%s' already exists. Overwrite? (Y/n): ", filePath)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(response)
	return response == "Y"
}

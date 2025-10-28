package client

import (
	"crypto/ecdh"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
	"github.com/schollz/share/src/crypto"
	"github.com/schollz/share/src/qrcode"
)

// SendFileWithLocalRelay sends a file with local relay support
func SendFileWithLocalRelay(filePath, roomID, serverURL string) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Fatalf("Failed to stat file: %v", err)
	}
	fileSize := fileInfo.Size()

	fileName := filepath.Base(filePath)
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
	var peerMnemonic string

	sendPublicKey := func() {
		pubKeyBytes := privKey.PublicKey().Bytes()
		pubKeyMsg := map[string]interface{}{
			"type": "pubkey",
			"pub":  base64.StdEncoding.EncodeToString(pubKeyBytes),
		}
		cm.SendMessage(pubKeyMsg)
	}

	done := make(chan bool)
	go func() {
		for {
			msg, err := cm.ReceiveMessage()
			if err != nil {
				return
			}

			switch msg.Type {
			case "error":
				if msg.Error != "" {
					log.Fatalf("Server error: %s", msg.Error)
				}
				return

			case "joined":
				// Convert WebSocket URL to HTTP URL for display
				webURL := serverURL
				if len(webURL) >= 6 && webURL[:6] == "wss://" {
					webURL = "https://" + webURL[6:]
				} else if len(webURL) >= 5 && webURL[:5] == "ws://" {
					webURL = "http://" + webURL[5:]
				}
				// Remove /ws path if present
				parsedURL, _ := url.Parse(webURL)
				parsedURL.Path = ""
				fullURL := fmt.Sprintf("%s/%s", parsedURL.String(), roomID)

				connType := cm.GetPreferredConnectionType()
				connTypeStr := ""
				if connType == ConnectionTypeLocal {
					connTypeStr = " (local network)"
				}

				fmt.Printf("Sending file '%s' (%s)%s.\n",
					fileName, formatBytes(fileSize), connTypeStr)
				fmt.Printf("Receive via CLI with\n\n\tshare receive %s\n\nor online at\n\n\t%s\n\n",
					roomID, fullURL)

				// Generate compact QR code (strip protocol for shorter code)
				qrURL := fullURL
				if len(qrURL) >= 8 && qrURL[:8] == "https://" {
					qrURL = qrURL[8:]
				} else if len(qrURL) >= 7 && qrURL[:7] == "http://" {
					qrURL = qrURL[7:]
				}

				// Print QR code using custom half-block renderer with left padding
				if err := qrcode.PrintHalfBlock(os.Stdout, qrURL, 15); err != nil {
					log.Printf("Failed to generate QR code: %v", err)
				}
				fmt.Println()

				sendPublicKey()

			case "peers":
				// When a new peer joins, re-send our public key
				if msg.Count == 2 {
					sendPublicKey()
				}

			case "pubkey":
				peerMnemonic = msg.Mnemonic
				peerPubBytes, _ := base64.StdEncoding.DecodeString(msg.Pub)
				peerPubKey, err := ecdh.P256().NewPublicKey(peerPubBytes)
				if err != nil {
					log.Fatalf("Failed to parse peer public key: %v", err)
				}

				sharedSecret, err = crypto.DeriveSharedSecret(privKey, peerPubKey)
				if err != nil {
					log.Fatalf("Failed to derive shared secret: %v", err)
				}

				// Open file for streaming
				file, err := os.Open(filePath)
				if err != nil {
					log.Fatalf("Failed to open file: %v", err)
				}
				defer file.Close()

				// Send file_start message
				fileStartMsg := map[string]interface{}{
					"type":       "file_start",
					"name":       fileName,
					"total_size": fileSize,
				}
				cm.SendMessage(fileStartMsg)

				// Create progress bar
				bar := progressbar.NewOptions64(
					fileSize,
					progressbar.OptionSetDescription("Sending"),
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

				// Stream file in chunks, encrypting each chunk individually
				chunkSize := 256 * 1024
				buffer := make([]byte, chunkSize)
				chunkNum := 0

				for {
					n, err := file.Read(buffer)
					if n > 0 {
						plainChunk := buffer[:n]

						// Encrypt this chunk with its own IV
						iv, cipherChunk, err := crypto.EncryptAESGCM(sharedSecret, plainChunk)
						if err != nil {
							log.Fatalf("Failed to encrypt chunk: %v", err)
						}

						// Send chunk with its IV
						chunkMsg := map[string]interface{}{
							"type":       "file_chunk",
							"chunk_num":  chunkNum,
							"chunk_data": base64.StdEncoding.EncodeToString(cipherChunk),
							"iv_b64":     base64.StdEncoding.EncodeToString(iv),
						}
						cm.SendMessage(chunkMsg)
						bar.Add(n)
						chunkNum++

						// Small delay to allow network transmission
						time.Sleep(10 * time.Millisecond)
					}

					if err == io.EOF {
						break
					}
					if err != nil {
						log.Fatalf("Failed to read file: %v", err)
					}
				}

				// Send file_end message
				fileEndMsg := map[string]interface{}{
					"type": "file_end",
				}
				cm.SendMessage(fileEndMsg)

				connType := cm.GetPreferredConnectionType()
				connTypeStr := ""
				if connType == ConnectionTypeLocal {
					connTypeStr = " via local network"
				}

				fmt.Printf("Sent encrypted file '%s' to %s (%s)%s\n", fileName, peerMnemonic, formatBytes(fileSize), connTypeStr)

				time.Sleep(500 * time.Millisecond)
				done <- true
			}
		}
	}()

	<-done
}

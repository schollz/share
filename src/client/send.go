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

// SendFile sends a file to the specified room via the relay server
func SendFile(filePath, roomID, serverURL string) {
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
	var peerMnemonic string

	sendPublicKey := func() {
		pubKeyBytes := privKey.PublicKey().Bytes()
		pubKeyMsg := map[string]interface{}{
			"type": "pubkey",
			"pub":  base64.StdEncoding.EncodeToString(pubKeyBytes),
		}
		conn.WriteJSON(pubKeyMsg)
	}

	done := make(chan bool)
	go func() {
		for {
			var msg relay.OutgoingMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				return
			}

			switch msg.Type {
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
				fmt.Printf("Sending file '%s' (%d bytes).\n",
					fileName, fileSize)
				fmt.Printf("\nReceive file online at\n\n\t%s/%s\n\nor receive via CLI with\n\n\tshare receive %s\n\n",
					parsedURL.String(), roomID, roomID)
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

				// Send file_start message (no IV needed here anymore)
				fileStartMsg := map[string]interface{}{
					"type":       "file_start",
					"name":       fileName,
					"total_size": fileSize,
				}
				conn.WriteJSON(fileStartMsg)

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
						conn.WriteJSON(chunkMsg)
						bar.Add(n)
						chunkNum++

						// Small delay to allow network transmission
						time.Sleep(10 * time.Millisecond)
					}

					if err == os.ErrClosed || err != nil {
						break
					}
				}

				// Send file_end message
				fileEndMsg := map[string]interface{}{
					"type": "file_end",
				}
				conn.WriteJSON(fileEndMsg)

				fmt.Printf("Sent encrypted file '%s' to %s (%s)\n", fileName, peerMnemonic, formatBytes(fileSize))

				time.Sleep(500 * time.Millisecond)
				done <- true
			}
		}
	}()

	<-done
}

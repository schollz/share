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

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

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

	fmt.Printf("Connected to %s\n", serverURL)

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
		fmt.Println("Sent public key")
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
				fmt.Printf("Joined room '%s' as %s\n", roomID, msg.Mnemonic)
				sendPublicKey()

			case "peers":
				// When a new peer joins, re-send our public key
				if msg.Count == 2 {
					fmt.Println("Peer joined, sending public key...")
					sendPublicKey()
				}

			case "pubkey":
				peerMnemonic = msg.Mnemonic
				fmt.Printf("Received peer public key from %s\n", peerMnemonic)
				peerPubBytes, _ := base64.StdEncoding.DecodeString(msg.Pub)
				peerPubKey, err := ecdh.P256().NewPublicKey(peerPubBytes)
				if err != nil {
					log.Fatalf("Failed to parse peer public key: %v", err)
				}

				sharedSecret, err = crypto.DeriveSharedSecret(privKey, peerPubKey)
				if err != nil {
					log.Fatalf("Failed to derive shared secret: %v", err)
				}
				fmt.Println("Derived shared AES key (E2EE ready)")

				// Encrypt the entire file first
				iv, ciphertext, err := crypto.EncryptAESGCM(sharedSecret, data)
				if err != nil {
					log.Fatalf("Failed to encrypt file: %v", err)
				}

				// Send file_start message
				fileStartMsg := map[string]interface{}{
					"type":       "file_start",
					"name":       fileName,
					"total_size": fileSize,
					"iv_b64":     base64.StdEncoding.EncodeToString(iv),
				}
				conn.WriteJSON(fileStartMsg)

				// Create progress bar
				bar := progressbar.NewOptions64(
					int64(len(ciphertext)),
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

				// Send in chunks (256KB per chunk)
				chunkSize := 256 * 1024
				totalChunks := (len(ciphertext) + chunkSize - 1) / chunkSize

				for i := 0; i < totalChunks; i++ {
					start := i * chunkSize
					end := start + chunkSize
					if end > len(ciphertext) {
						end = len(ciphertext)
					}

					chunk := ciphertext[start:end]
					chunkMsg := map[string]interface{}{
						"type":       "file_chunk",
						"chunk_num":  i,
						"chunk_data": base64.StdEncoding.EncodeToString(chunk),
					}
					conn.WriteJSON(chunkMsg)
					bar.Add(len(chunk))

					// Small delay to allow network transmission
					time.Sleep(10 * time.Millisecond)
				}

				// Send file_end message
				fileEndMsg := map[string]interface{}{
					"type": "file_end",
				}
				conn.WriteJSON(fileEndMsg)

				fmt.Printf("Sent encrypted file '%s' to %s (%d bytes)\n", fileName, peerMnemonic, len(data))

				time.Sleep(500 * time.Millisecond)
				done <- true
			}
		}
	}()

	<-done
	fmt.Println("Transfer complete!")
}

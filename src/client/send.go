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

	"share/src/crypto"
	"share/src/relay"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// SendFile sends a file to the specified room via the relay server
func SendFile(filePath, roomID, serverURL string) {
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

	fmt.Printf("📡 Connected to %s\n", serverURL)

	joinMsg := map[string]interface{}{
		"type":     "join",
		"roomId":   roomID,
		"clientId": clientID,
	}
	conn.WriteJSON(joinMsg)

	var sharedSecret []byte
	var peerMnemonic string

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
				fmt.Printf("✅ Joined room '%s' as %s\n", roomID, msg.Mnemonic)
				pubKeyBytes := privKey.PublicKey().Bytes()
				pubKeyMsg := map[string]interface{}{
					"type": "pubkey",
					"pub":  base64.StdEncoding.EncodeToString(pubKeyBytes),
				}
				conn.WriteJSON(pubKeyMsg)
				fmt.Println("📡 Sent public key")

			case "pubkey":
				peerMnemonic = msg.Mnemonic
				fmt.Printf("📥 Received peer public key from %s\n", peerMnemonic)
				peerPubBytes, _ := base64.StdEncoding.DecodeString(msg.Pub)
				peerPubKey, err := ecdh.P256().NewPublicKey(peerPubBytes)
				if err != nil {
					log.Fatalf("Failed to parse peer public key: %v", err)
				}

				sharedSecret, err = crypto.DeriveSharedSecret(privKey, peerPubKey)
				if err != nil {
					log.Fatalf("Failed to derive shared secret: %v", err)
				}
				fmt.Println("🤝 Derived shared AES key (E2EE ready)")

				iv, ciphertext, err := crypto.EncryptAESGCM(sharedSecret, data)
				if err != nil {
					log.Fatalf("Failed to encrypt file: %v", err)
				}

				fileMsg := map[string]interface{}{
					"type":     "file",
					"name":     fileName,
					"iv_b64":   base64.StdEncoding.EncodeToString(iv),
					"data_b64": base64.StdEncoding.EncodeToString(ciphertext),
				}
				conn.WriteJSON(fileMsg)
				fmt.Printf("🚀 Sent encrypted file '%s' to %s (%d bytes)\n", fileName, peerMnemonic, len(data))

				time.Sleep(500 * time.Millisecond)
				done <- true
			}
		}
	}()

	<-done
	fmt.Println("✨ Transfer complete!")
}

package client

import (
	"crypto/ecdh"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/schollz/share/src/crypto"
	"github.com/schollz/share/src/relay"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

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

	fmt.Printf("📡 Connected to %s\n", serverURL)

	joinMsg := map[string]interface{}{
		"type":     "join",
		"roomId":   roomID,
		"clientId": clientID,
	}
	conn.WriteJSON(joinMsg)

	var sharedSecret []byte

	for {
		var msg relay.OutgoingMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Fatalf("Connection closed: %v", err)
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
			fmt.Println("⏳ Waiting for file...")

		case "pubkey":
			fmt.Printf("📥 Received peer public key from %s\n", msg.Mnemonic)
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

		case "file":
			if sharedSecret == nil {
				fmt.Println("❌ Can't decrypt yet (no shared key)")
				continue
			}

			fmt.Printf("📦 Incoming encrypted file: %s\n", msg.Name)

			iv, _ := base64.StdEncoding.DecodeString(msg.IvB64)
			ciphertext, _ := base64.StdEncoding.DecodeString(msg.DataB64)

			plaintext, err := crypto.DecryptAESGCM(sharedSecret, iv, ciphertext)
			if err != nil {
				log.Fatalf("❌ Decryption failed: %v", err)
			}

			outputPath := filepath.Join(outputDir, msg.Name)
			err = os.WriteFile(outputPath, plaintext, 0644)
			if err != nil {
				log.Fatalf("Failed to write file: %v", err)
			}

			fmt.Printf("✅ Decrypted and saved file: %s (%d bytes)\n", outputPath, len(plaintext))
			fmt.Println("✨ Transfer complete!")
			return
		}
	}
}

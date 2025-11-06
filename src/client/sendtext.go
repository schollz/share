package client

import (
	"crypto/ecdh"
	"encoding/base64"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/schollz/e2ecp/src/crypto"
	"github.com/schollz/e2ecp/src/qrcode"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// SendText sends a text message to the specified room via the relay server
func SendText(text, roomID, serverURL string, logger *slog.Logger) {
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

	// Mutex to protect websocket writes
	var connMutex sync.Mutex
	safeSend := func(msg map[string]interface{}) {
		connMutex.Lock()
		defer connMutex.Unlock()
		sendProtobufMessage(conn, msg)
	}

	joinMsg := map[string]interface{}{
		"type":     "join",
		"roomId":   roomID,
		"clientId": clientID,
	}
	safeSend(joinMsg)

	// Generate random room if not provided
	if roomID == "" {
		log.Fatal("Room ID is required")
	}

	// Show QR code
	qrURL, _ := url.Parse(serverURL)
	qrURL.Path = "/" + roomID
	if err := qrcode.PrintHalfBlock(os.Stdout, qrURL.String(), 15); err != nil {
		logger.Debug("Failed to print QR code", "error", err)
	}

	fmt.Printf("Share this link with the recipient:\n")
	fmt.Printf("  %s\n\n", qrURL.String())
	fmt.Printf("Waiting for peer to join room '%s'...\n", roomID)

	// Handle incoming messages
	var sharedSecret []byte
	var myMnemonic string
	var peerMnemonic string
	peerConnected := false
	textSent := false

	// Function to send our public key
	sendPublicKey := func() {
		pubBytes := privKey.PublicKey().Bytes()
		pubKeyMsg := map[string]interface{}{
			"type": "pubkey",
			"pub":  base64.StdEncoding.EncodeToString(pubBytes),
		}
		safeSend(pubKeyMsg)
	}

	for {
		msg, err := receiveProtobufMessage(conn)
		if err != nil {
			logger.Debug("Connection closed", "error", err)
			break
		}

		switch msg.Type {
		case "joined":
			myMnemonic = msg.Mnemonic
			logger.Debug("Joined room", "mnemonic", myMnemonic)
			// Announce our public key immediately when we join
			sendPublicKey()

		case "peers":
			if msg.Count >= 2 && !peerConnected {
				fmt.Println("\nPeer connected! Establishing secure channel...")
				peerConnected = true
				// Re-send public key when peer joins
				sendPublicKey()
			}

		case "pubkey":
			peerMnemonic = msg.Mnemonic
			logger.Debug("Received peer public key", "peer", peerMnemonic)

			// Derive shared secret
			peerPubKeyBytes, err := base64.StdEncoding.DecodeString(msg.Pub)
			if err != nil {
				log.Fatalf("Failed to decode peer public key: %v", err)
			}

			peerPubKey, err := ecdh.P256().NewPublicKey(peerPubKeyBytes)
			if err != nil {
				log.Fatalf("Failed to parse peer public key: %v", err)
			}

			sharedSecret, err = privKey.ECDH(peerPubKey)
			if err != nil {
				log.Fatalf("Failed to derive shared secret: %v", err)
			}

			// Send text message immediately after deriving shared secret
			if sharedSecret != nil && !textSent {
				textSent = true
				err = sendTextMessage(text, sharedSecret, safeSend, logger)
				if err != nil {
					log.Fatalf("Failed to send text: %v", err)
				}
				fmt.Printf("\n✓ Sent text message to %s\n", peerMnemonic)
			}

		case "text_received":
			// Receiver confirmed they received the text
			fmt.Println("✓ Text message delivered")
			return
		}
	}
}

func sendTextMessage(text string, sharedSecret []byte, safeSend func(map[string]interface{}), logger *slog.Logger) error {
	// Encrypt the text message
	textBytes := []byte(text)
	iv, encryptedText, err := crypto.EncryptAESGCM(sharedSecret, textBytes)
	if err != nil {
		return fmt.Errorf("failed to encrypt text: %v", err)
	}

	// Send text_message with encrypted text
	textMsg := map[string]interface{}{
		"type":               "text_message",
		"encrypted_metadata": base64.StdEncoding.EncodeToString(encryptedText),
		"metadata_iv":        base64.StdEncoding.EncodeToString(iv),
	}
	safeSend(textMsg)

	logger.Debug("Sent encrypted text message")
	return nil
}

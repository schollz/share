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

	"github.com/schollz/progressbar/v3"
	"github.com/schollz/share/src/crypto"
	"github.com/schollz/share/src/qrcode"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// SendFile sends a file to the specified room via the relay server
func SendFile(filePath, roomID, serverURL string) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Fatalf("Failed to stat file: %v", err)
	}

	// Check if path is a directory
	isFolder := fileInfo.IsDir()
	var actualFilePath string
	var originalFolderName string
	var tempZipPath string
	var fileSize int64

	if isFolder {
		// It's a folder - we need to zip it
		originalFolderName = filepath.Base(filePath)

		// Count files for user feedback
		fileCount, err := CountFilesInDirectory(filePath)
		if err != nil {
			log.Fatalf("Failed to count files in directory: %v", err)
		}

		fmt.Printf("Zipping folder '%s' (%d files)...\n", originalFolderName, fileCount)

		// Create temp zip file
		tempZipPath = filepath.Join(os.TempDir(), originalFolderName+".zip")
		err = CreateZipFromDirectory(filePath, tempZipPath)
		if err != nil {
			log.Fatalf("Failed to zip folder: %v", err)
		}
		defer os.Remove(tempZipPath) // Clean up temp file when done

		// Get zip file size
		zipInfo, err := os.Stat(tempZipPath)
		if err != nil {
			log.Fatalf("Failed to stat zip file: %v", err)
		}
		fileSize = zipInfo.Size()
		actualFilePath = tempZipPath

		fmt.Printf("Folder zipped successfully (%s)\n", formatBytes(fileSize))
	} else {
		// It's a regular file
		fileSize = fileInfo.Size()
		actualFilePath = filePath
		originalFolderName = ""
	}

	fileName := filepath.Base(actualFilePath)
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
	var peerMnemonic string

	sendPublicKey := func() {
		pubKeyBytes := privKey.PublicKey().Bytes()
		pubKeyMsg := map[string]interface{}{
			"type": "pubkey",
			"pub":  base64.StdEncoding.EncodeToString(pubKeyBytes),
		}
		sendProtobufMessage(conn, pubKeyMsg)
	}

	done := make(chan bool)
	go func() {
		defer func() {
			select {
			case done <- true:
			default:
			}
		}()
		for {
			msg, err := receiveProtobufMessage(conn)
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

				if isFolder {
					fmt.Printf("Sending folder '%s' (%s, zipped).\n",
						originalFolderName, formatBytes(fileSize))
				} else {
					fmt.Printf("Sending file '%s' (%s).\n",
						fileName, formatBytes(fileSize))
				}
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

				// Calculate file hash before sending
				hashFile, err := os.Open(actualFilePath)
				if err != nil {
					log.Fatalf("Failed to open file for hashing: %v", err)
				}
				fileHash, err := crypto.CalculateFileHash(hashFile)
				hashFile.Close()
				if err != nil {
					log.Fatalf("Failed to calculate file hash: %v", err)
				}

				// Open file for streaming
				file, err := os.Open(actualFilePath)
				if err != nil {
					log.Fatalf("Failed to open file: %v", err)
				}
				defer file.Close()

				// Create encrypted metadata
				metadata := FileMetadata{
					Name:      fileName,
					TotalSize: fileSize,
					Hash:      fileHash,
				}
				if isFolder {
					metadata.IsFolder = true
					metadata.OriginalFolderName = originalFolderName
				}

				// Marshal and encrypt metadata
				metadataJSON, err := MarshalMetadata(metadata)
				if err != nil {
					log.Fatalf("Failed to marshal metadata: %v", err)
				}

				metadataIV, encryptedMetadata, err := crypto.EncryptAESGCM(sharedSecret, metadataJSON)
				if err != nil {
					log.Fatalf("Failed to encrypt metadata: %v", err)
				}

				// Send file_start message with encrypted metadata only
				fileStartMsg := map[string]interface{}{
					"type":               "file_start",
					"encrypted_metadata": base64.StdEncoding.EncodeToString(encryptedMetadata),
					"metadata_iv":        base64.StdEncoding.EncodeToString(metadataIV),
				}
				sendProtobufMessage(conn, fileStartMsg)

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
						sendProtobufMessage(conn, chunkMsg)
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
				sendProtobufMessage(conn, fileEndMsg)

				if isFolder {
					fmt.Printf("Sent encrypted folder '%s' to %s (%s)\n", originalFolderName, peerMnemonic, formatBytes(fileSize))
				} else {
					fmt.Printf("Sent encrypted file '%s' to %s (%s)\n", fileName, peerMnemonic, formatBytes(fileSize))
				}

				time.Sleep(500 * time.Millisecond)
				done <- true
			}
		}
	}()

	<-done
}

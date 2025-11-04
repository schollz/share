package client

import (
	"crypto/ecdh"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/schollz/e2ecp/src/crypto"
	"github.com/schollz/e2ecp/src/qrcode"
	"github.com/schollz/progressbar/v3"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// SendFile sends a file to the specified room via the relay server
func SendFile(filePath, roomID, serverURL string, logger *slog.Logger) {
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

	var sharedSecret []byte
	var peerMnemonic string
	var transferStarted bool
	var transferMutex sync.Mutex

	// Local relay connection tracking
	var useLocalRelay bool
	var localConn *websocket.Conn
	var localConnMutex sync.Mutex
	var localSafeSend func(msg map[string]interface{})

	sendPublicKey := func() {
		pubKeyBytes := privKey.PublicKey().Bytes()
		pubKeyMsg := map[string]interface{}{
			"type": "pubkey",
			"pub":  base64.StdEncoding.EncodeToString(pubKeyBytes),
		}
		safeSend(pubKeyMsg)
	}

	done := make(chan bool)
	ackChan := make(chan int, 100) // Channel for receiving chunk ACKs

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

			case "peer_disconnected":
				disconnectedPeerName := msg.Mnemonic
				if disconnectedPeerName == "" {
					disconnectedPeerName = "Peer"
				}
				fmt.Printf("\n%s disconnected. Exiting to prevent new connections.\n", disconnectedPeerName)
				return

			case "transfer_cancelled":
				receiverName := msg.Mnemonic
				if receiverName == "" {
					receiverName = "Receiver"
				}
				fmt.Printf("\n%s cancelled the transfer.\n", receiverName)
				return

			case "transfer_received":
				// Receiver confirmed they successfully received the file
				receiverName := msg.Mnemonic
				if receiverName == "" {
					receiverName = "Receiver"
				}
				fmt.Printf("%s confirmed receipt of the file.\n", receiverName)

			case "chunk_ack":
				// Receiver acknowledged receiving a chunk
				ackChan <- msg.ChunkNum

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
				fmt.Printf("Receive via CLI with\n\n\te2ecp receive %s\n\nor online at\n\n\t%s\n\n",
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

			case "local_relay_info":
				// Receiver sent us encrypted local relay information - decrypt and try to connect
				if msg.EncryptedMetadata == "" || msg.MetadataIV == "" {
					logger.Debug("Received local_relay_info without encrypted metadata")
					continue
				}

				// Decrypt local relay info
				relayInfoIV, _ := base64.StdEncoding.DecodeString(msg.MetadataIV)
				encryptedRelayInfo, _ := base64.StdEncoding.DecodeString(msg.EncryptedMetadata)

				relayInfoJSON, err := crypto.DecryptAESGCM(sharedSecret, relayInfoIV, encryptedRelayInfo)
				if err != nil {
					logger.Debug("Failed to decrypt local relay info", "error", err)
					continue
				}

				relayInfo, err := UnmarshalLocalRelayInfo(relayInfoJSON)
				if err != nil {
					logger.Debug("Failed to unmarshal local relay info", "error", err)
					continue
				}

				if len(relayInfo.IPs) == 0 || relayInfo.Port == 0 {
					logger.Debug("Invalid local relay info", "num_ips", len(relayInfo.IPs), "port", relayInfo.Port)
					continue
				}

				logger.Debug("Received encrypted local relay info", "num_ips", len(relayInfo.IPs), "port", relayInfo.Port)

				// Try to connect to local relay using each IP address
				for _, ip := range relayInfo.IPs {
					localURL := fmt.Sprintf("ws://%s:%d/ws", ip, relayInfo.Port)
					lconn, _, err := websocket.DefaultDialer.Dial(localURL, nil)
					if err != nil {
						logger.Debug("Failed to connect to local relay", "url", localURL, "error", err)
						continue
					}

					// Successfully connected to local relay
					localConn = lconn
					localSafeSend = func(msg map[string]interface{}) {
						localConnMutex.Lock()
						defer localConnMutex.Unlock()
						sendProtobufMessage(localConn, msg)
					}

					// Join the same room on local relay
					localJoinMsg := map[string]interface{}{
						"type":     "join",
						"roomId":   roomID,
						"clientId": clientID,
					}
					localSafeSend(localJoinMsg)

					// Wait a bit for join to complete
					time.Sleep(100 * time.Millisecond)

					useLocalRelay = true
					logger.Debug("Connected to local relay for file transfer", "url", localURL)
					fmt.Printf("Connected to local relay at %s (faster transfer)\n", localURL)

					// Start reading from local relay connection
					go func() {
						for {
							lmsg, err := receiveProtobufMessage(localConn)
							if err != nil {
								logger.Debug("Local relay connection closed", "error", err)
								return
							}

							// Handle messages from local relay
							switch lmsg.Type {
							case "chunk_ack":
								ackChan <- lmsg.ChunkNum
							case "transfer_cancelled":
								receiverName := lmsg.Mnemonic
								if receiverName == "" {
									receiverName = "Receiver"
								}
								fmt.Printf("\n%s cancelled the transfer.\n", receiverName)
								done <- true
								return
							}
						}
					}()

					break
				}

				if !useLocalRelay {
					logger.Debug("Failed to connect to any local relay IP, will use global relay")
				}

			case "pubkey":
				// Check if transfer already started to prevent duplicates
				transferMutex.Lock()
				if transferStarted {
					transferMutex.Unlock()
					continue
				}
				transferStarted = true
				transferMutex.Unlock()

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

				// Start file transfer in a separate goroutine so message loop continues
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Fatalf("Panic during file transfer: %v", r)
						}
					}()

					// Give local relay connection time to establish if it's being set up
					time.Sleep(200 * time.Millisecond)

					// Use local relay for file transfer if available, otherwise use global relay
					transferSend := safeSend
					if useLocalRelay && localSafeSend != nil {
						transferSend = localSafeSend
						logger.Debug("Using local relay for file transfer")
					} else {
						logger.Debug("Using global relay for file transfer")
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
					transferSend(fileStartMsg)

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

					// Track chunks for retransmission
					type ChunkInfo struct {
						data     []byte
						iv       []byte
						num      int
						sentTime time.Time
						retries  int
					}
					pendingChunks := make(map[int]*ChunkInfo)
					var pendingMutex sync.Mutex
					maxRetries := 3
					ackTimeout := 5 * time.Second
					lastActivityTime := time.Now()
					transferTimeout := 30 * time.Second

					// Goroutine to handle ACKs and retransmissions
					stopRetransmitter := make(chan bool)
					go func() {
						ticker := time.NewTicker(500 * time.Millisecond)
						defer ticker.Stop()

						for {
							select {
							case <-stopRetransmitter:
								return
							case ackNum := <-ackChan:
								// Remove acknowledged chunk
								pendingMutex.Lock()
								delete(pendingChunks, ackNum)
								lastActivityTime = time.Now()
								pendingMutex.Unlock()
							case <-ticker.C:
								// Check for chunks that need retransmission
								pendingMutex.Lock()
								now := time.Now()

								// Check for transfer timeout
								if now.Sub(lastActivityTime) > transferTimeout {
									pendingMutex.Unlock()
									log.Fatalf("Transfer timeout: no activity for %v", transferTimeout)
								}

								for _, chunk := range pendingChunks {
									if now.Sub(chunk.sentTime) > ackTimeout {
										if chunk.retries >= maxRetries {
											pendingMutex.Unlock()
											log.Fatalf("Failed to send chunk %d after %d retries", chunk.num, maxRetries)
										}

										// Resend chunk
										chunkMsg := map[string]interface{}{
											"type":       "file_chunk",
											"chunk_num":  chunk.num,
											"chunk_data": base64.StdEncoding.EncodeToString(chunk.data),
											"iv_b64":     base64.StdEncoding.EncodeToString(chunk.iv),
										}
										transferSend(chunkMsg)
										chunk.sentTime = now
										chunk.retries++
										lastActivityTime = now
									}
								}
								pendingMutex.Unlock()
							}
						}
					}()

					for {
						n, err := file.Read(buffer)
						if n > 0 {
							plainChunk := buffer[:n]

							// Encrypt this chunk with its own IV
							iv, cipherChunk, err := crypto.EncryptAESGCM(sharedSecret, plainChunk)
							if err != nil {
								log.Fatalf("Failed to encrypt chunk: %v", err)
							}

							// Store chunk for potential retransmission
							pendingMutex.Lock()
							pendingChunks[chunkNum] = &ChunkInfo{
								data:     cipherChunk,
								iv:       iv,
								num:      chunkNum,
								sentTime: time.Now(),
								retries:  0,
							}
							lastActivityTime = time.Now()
							pendingMutex.Unlock()

							// Send chunk with its IV
							chunkMsg := map[string]interface{}{
								"type":       "file_chunk",
								"chunk_num":  chunkNum,
								"chunk_data": base64.StdEncoding.EncodeToString(cipherChunk),
								"iv_b64":     base64.StdEncoding.EncodeToString(iv),
							}
							transferSend(chunkMsg)
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

					// Wait for all chunks to be acknowledged
					waitStart := time.Now()
					for {
						pendingMutex.Lock()
						pendingCount := len(pendingChunks)
						pendingMutex.Unlock()

						if pendingCount == 0 {
							break
						}

						if time.Since(waitStart) > 30*time.Second {
							stopRetransmitter <- true
							log.Fatalf("Timeout waiting for chunk acknowledgments")
						}

						time.Sleep(100 * time.Millisecond)
					}

					stopRetransmitter <- true

					// Send file_end message
					fileEndMsg := map[string]interface{}{
						"type": "file_end",
					}
					transferSend(fileEndMsg)

					if isFolder {
						fmt.Printf("Sent encrypted folder '%s' to %s (%s)\n", originalFolderName, peerMnemonic, formatBytes(fileSize))
					} else {
						fmt.Printf("Sent encrypted file '%s' to %s (%s)\n", fileName, peerMnemonic, formatBytes(fileSize))
					}

					time.Sleep(500 * time.Millisecond)
					done <- true
				}()
			}
		}
	}()

	<-done
}

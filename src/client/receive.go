package client

import (
	"bufio"
	"crypto/ecdh"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/schollz/share/src/crypto"

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

// promptOverwrite prompts the user to confirm overwriting an existing file
// Returns true if the user confirms with capital 'Y', false otherwise
func promptOverwrite(filePath string) bool {
	fmt.Printf("File '%s' already exists. Overwrite? (Y/n): ", filePath)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(response)
	return response == "Y"
}

// checkFileOverwrite checks if a file exists and prompts for overwrite if not forcing
// Returns true to proceed, false to cancel
func checkFileOverwrite(outputPath string, forceOverwrite bool) bool {
	if !forceOverwrite {
		if _, err := os.Stat(outputPath); err == nil {
			if !promptOverwrite(outputPath) {
				fmt.Println("File transfer cancelled.")
				return false
			}
		}
	}
	return true
}

// sanitizeFileName cleans a filename to prevent path traversal attacks
// by extracting only the base filename and removing any directory components
func sanitizeFileName(fileName string) string {
	// Use filepath.Base to extract only the filename, removing any directory components
	// This prevents path traversal attacks like "../../../etc/passwd"
	return filepath.Base(fileName)
}

// ReceiveFile receives a file from the specified room via the relay server
func ReceiveFile(roomID, serverURL, outputDir string, forceOverwrite bool) {
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
	var fileName string
	var totalSize int64
	var receivedBytes int64
	var bar *progressbar.ProgressBar
	var outputFile *os.File
	var isFolder bool
	var isMultipleFiles bool
	var originalFolderName string
	var tempZipPath string
	var expectedHash string

	sendPublicKey := func() {
		pubKeyBytes := privKey.PublicKey().Bytes()
		pubKeyMsg := map[string]interface{}{
			"type": "pubkey",
			"pub":  base64.StdEncoding.EncodeToString(pubKeyBytes),
		}
		sendProtobufMessage(conn, pubKeyMsg)
	}

	for {
		msg, err := receiveProtobufMessage(conn)
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

			// Decrypt metadata
			if msg.EncryptedMetadata == "" || msg.MetadataIV == "" {
				log.Fatal("Missing encrypted metadata")
			}

			metadataIV, _ := base64.StdEncoding.DecodeString(msg.MetadataIV)
			encryptedMeta, _ := base64.StdEncoding.DecodeString(msg.EncryptedMetadata)

			metadataJSON, err := crypto.DecryptAESGCM(sharedSecret, metadataIV, encryptedMeta)
			if err != nil {
				log.Fatalf("Failed to decrypt metadata: %v", err)
			}

			metadata, err := UnmarshalMetadata(metadataJSON)
			if err != nil {
				log.Fatalf("Failed to unmarshal metadata: %v", err)
			}

			// Use decrypted metadata
			fileName = sanitizeFileName(metadata.Name)
			totalSize = metadata.TotalSize
			isFolder = metadata.IsFolder
			isMultipleFiles = metadata.IsMultipleFiles
			originalFolderName = metadata.OriginalFolderName
			expectedHash = metadata.Hash

			receivedBytes = 0

			var outputPath string
			if isFolder {
				// Save to temp zip file first
				tempZipPath = filepath.Join(outputDir, fileName)
				outputPath = tempZipPath
				fmt.Printf("Receiving folder '%s' (%s, zipped)\n", originalFolderName, formatBytes(totalSize))
			} else {
				outputPath = filepath.Join(outputDir, fileName)
			}

			// Check if file exists and prompt for overwrite if needed
			if !checkFileOverwrite(outputPath, forceOverwrite) {
				return
			}

			outputFile, err = os.Create(outputPath)
			if err != nil {
				log.Fatalf("Failed to create output file: %v", err)
			}

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

			// Verify file hash if provided
			if expectedHash != "" {
				// Calculate hash of received file
				outputPath := filepath.Join(outputDir, fileName)

				receivedFile, err := os.Open(outputPath)
				if err != nil {
					log.Fatalf("Failed to open received file for verification: %v", err)
				}
				actualHash, err := crypto.CalculateFileHash(receivedFile)
				receivedFile.Close()
				if err != nil {
					log.Fatalf("Failed to calculate received file hash: %v", err)
				}

				// Compare hashes
				if actualHash != expectedHash {
					fmt.Printf("\n⚠️  WARNING: File hash mismatch!\n")
					fmt.Printf("   Expected: %s\n", expectedHash)
					fmt.Printf("   Received: %s\n", actualHash)
					fmt.Printf("   The file may be corrupted or tampered with.\n\n")
					// Continue with extraction anyway, but user is warned
				} else {
					// Display truncated hash only if it's long enough
					if len(expectedHash) >= 16 {
						fmt.Printf("✓ File integrity verified (hash: %s...%s)\n", expectedHash[:8], expectedHash[len(expectedHash)-8:])
					} else {
						fmt.Printf("✓ File integrity verified (hash: %s)\n", expectedHash)
					}
				}
			}

			// Check if this is a zip file (folder, multiple files, or ends with .zip)
			isZipFile := isFolder || isMultipleFiles || strings.HasSuffix(strings.ToLower(fileName), ".zip")

			if isZipFile {
				// Extract the zip file
				if isFolder {
					fmt.Println("Extracting folder...")
				} else if isMultipleFiles {
					fmt.Println("Extracting files...")
				} else {
					fmt.Println("Extracting zip file...")
				}

				zipPath := filepath.Join(outputDir, fileName)

				// Determine where to extract
				var extractDir string
				if isMultipleFiles {
					// Extract directly to outputDir for multiple files
					extractDir = outputDir
				} else {
					// Determine extraction directory name
					var extractDirName string
					if isFolder && originalFolderName != "" {
						extractDirName = sanitizeFileName(originalFolderName)
					} else {
						// Remove .zip extension from filename
						extractDirName = strings.TrimSuffix(fileName, ".zip")
						extractDirName = sanitizeFileName(extractDirName)
					}
					extractDir = filepath.Join(outputDir, extractDirName)

					// Check if directory exists
					if _, err := os.Stat(extractDir); err == nil {
						if !forceOverwrite {
							fmt.Printf("Directory '%s' already exists. Overwrite? (Y/n): ", extractDir)
							reader := bufio.NewReader(os.Stdin)
							response, err := reader.ReadString('\n')
							if err != nil || strings.TrimSpace(response) != "Y" {
								fmt.Println("Extraction cancelled.")
								fmt.Printf("Zip file saved as: %s\n", zipPath)
								return
							}
						}
						// Remove existing directory
						os.RemoveAll(extractDir)
					}
				}

				// Extract the zip and get list of extracted files
				// For multiple files, strip the root folder from the zip
				var extractedFiles []string
				var err error
				if isMultipleFiles {
					extractedFiles, err = ExtractZipToDirectoryWithOptions(zipPath, outputDir, true)
				} else {
					extractedFiles, err = ExtractZipToDirectory(zipPath, outputDir)
				}
				if err != nil {
					log.Fatalf("Failed to extract zip: %v", err)
				}

				// Delete zip file after successful extraction
				os.Remove(zipPath)

				// Show appropriate message based on type
				if isFolder {
					// For folders, just show simple message
					fileCount, _ := CountFilesInDirectory(extractDir)
					fmt.Printf("Folder received: %s (%d files, %s)\n", extractDir, fileCount, formatBytes(totalSize))
				} else if isMultipleFiles {
					// For multiple files, list the extracted files
					if len(extractedFiles) > 0 {
						fmt.Printf("\nExtracted %d file(s):\n", len(extractedFiles))
						for _, file := range extractedFiles {
							// Show relative path from output directory
							relPath, err := filepath.Rel(outputDir, file)
							if err != nil || relPath == "" {
								// Fallback to just the basename if relative path fails
								relPath = filepath.Base(file)
							}
							fmt.Printf("  - %s\n", relPath)
						}
					} else {
						fmt.Printf("Extracted %d files (%s)\n", len(extractedFiles), formatBytes(totalSize))
					}
				} else {
					// For other .zip files, list extracted files with subdirectory
					if len(extractedFiles) > 0 {
						fmt.Printf("\nExtracted %d file(s) to %s:\n", len(extractedFiles), extractDir)
						for _, file := range extractedFiles {
							// Show relative path from extract directory
							relPath, _ := filepath.Rel(extractDir, file)
							fmt.Printf("  - %s\n", relPath)
						}
					} else {
						fileCount, _ := CountFilesInDirectory(extractDir)
						fmt.Printf("Extracted: %s (%d files, %s)\n", extractDir, fileCount, formatBytes(totalSize))
					}
				}
			} else {
				outputPath := filepath.Join(outputDir, fileName)
				fmt.Printf("Saved: %s (%s)\n", outputPath, formatBytes(totalSize))
			}
			return

		}
	}
}

package client

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/schollz/e2ecp/src/crypto"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

// getConfigDir returns the appropriate config directory for the OS
func getConfigDir() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		configDir = os.Getenv("APPDATA")
		if configDir == "" {
			configDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
		configDir = filepath.Join(configDir, "e2ecp")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, "Library", "Application Support", "e2ecp")
	default: // Linux and other Unix-like systems
		configDir = os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configDir = filepath.Join(home, ".config")
		}
		configDir = filepath.Join(configDir, "e2ecp")
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", err
	}

	return configDir, nil
}

// getTokenPath returns the path to the token file
func getTokenPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "token"), nil
}

// saveToken saves the authentication token to disk
func saveToken(token string) error {
	tokenPath, err := getTokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(tokenPath, []byte(token), 0600)
}

// loadToken loads the authentication token from disk
func loadToken() (string, error) {
	tokenPath, err := getTokenPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("not authenticated. Run 'e2ecp auth' first")
		}
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// AuthenticateDevice initiates the device authentication flow
func AuthenticateDevice(server string, logger *slog.Logger) {
	// Parse server URL
	serverURL, err := url.Parse(server)
	if err != nil || serverURL.Scheme == "" {
		serverURL, _ = url.Parse("https://" + server)
	}

	// Initiate device auth
	initURL := fmt.Sprintf("%s://%s/api/auth/device/init", serverURL.Scheme, serverURL.Host)
	resp, err := http.Post(initURL, "application/json", nil)
	if err != nil {
		logger.Error("Failed to initiate device auth", "error", err)
		fmt.Println("Error: Failed to connect to server")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Device auth initiation failed", "status", resp.StatusCode)
		fmt.Println("Error: Failed to initiate authentication")
		return
	}

	var initResp struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
		ExpiresIn  int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		logger.Error("Failed to decode init response", "error", err)
		fmt.Println("Error: Failed to parse server response")
		return
	}

	// Display instructions to user
	authURL := fmt.Sprintf("%s://%s/device-auth", serverURL.Scheme, serverURL.Host)
	fmt.Println("\n╔════════════════════════════════════════════════════╗")
	fmt.Println("║          E2ECP DEVICE AUTHENTICATION               ║")
	fmt.Println("╠════════════════════════════════════════════════════╣")
	// Build lines with proper padding - content should be exactly 52 chars
	line1 := fmt.Sprintf("  1. Visit: %s", authURL)
	fmt.Printf("║%-52s║\n", line1)
	line2 := fmt.Sprintf("  2. Enter code: %s", initResp.UserCode)
	fmt.Printf("║%-52s║\n", line2)
	fmt.Println("║  3. Waiting for approval...                        ║")
	fmt.Println("╚════════════════════════════════════════════════════╝\n")

	// Poll for approval
	pollURL := fmt.Sprintf("%s://%s/api/auth/device/poll", serverURL.Scheme, serverURL.Host)
	pollInterval := 5 * time.Second
	timeout := time.Duration(initResp.ExpiresIn) * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Poll the server
		pollReq := map[string]string{"device_code": initResp.DeviceCode}
		pollBody, _ := json.Marshal(pollReq)
		pollResp, err := http.Post(pollURL, "application/json", bytes.NewReader(pollBody))
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}

		var pollResult struct {
			Status string `json:"status"`
			Token  string `json:"token"`
		}

		body, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()

		if err := json.Unmarshal(body, &pollResult); err != nil {
			time.Sleep(pollInterval)
			continue
		}

		if pollResult.Status == "approved" {
			// Save token
			if err := saveToken(pollResult.Token); err != nil {
				logger.Error("Failed to save token", "error", err)
				fmt.Println("Error: Failed to save authentication token")
				return
			}

			fmt.Println("✓ Authentication successful!")
			fmt.Println("✓ Token saved to config directory")
			fmt.Println("\nYou can now use 'e2ecp upload <file>' to upload files to your account.")
			return
		}

		// Still pending, keep polling
		fmt.Print(".")
		time.Sleep(pollInterval)
	}

	fmt.Println("\n\n✗ Authentication timed out. Please try again.")
}

// UploadFile uploads a file to the authenticated user's account
func UploadFile(filePath, server string, logger *slog.Logger) {
	// Load token
	token, err := loadToken()
	if err != nil {
		fmt.Println("Error:", err.Error())
		return
	}

	// Parse server URL
	serverURL, err := url.Parse(server)
	if err != nil || serverURL.Scheme == "" {
		serverURL, _ = url.Parse("https://" + server)
	}

	// Get user info to retrieve encryption salt
	verifyURL := fmt.Sprintf("%s://%s/api/auth/verify", serverURL.Scheme, serverURL.Host)
	req, _ := http.NewRequest("GET", verifyURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to verify token", "error", err)
		fmt.Println("Error: Failed to authenticate. Please run 'e2ecp auth' again.")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error: Authentication failed. Please run 'e2ecp auth' again.")
		return
	}

	var userInfo struct {
		EncryptionSalt string `json:"encryption_salt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		logger.Error("Failed to decode user info", "error", err)
		fmt.Println("Error: Failed to get user information")
		return
	}

	// Ask for password to derive encryption key
	fmt.Print("Enter your password to encrypt the file: ")
	passwordBytes, err := readPassword()
	if err != nil {
		fmt.Println("\nError: Failed to read password")
		return
	}
	password := string(passwordBytes)
	fmt.Println() // New line after password input

	// Derive master key using PBKDF2 (matching web app)
	masterKey, err := deriveMasterKey(password, userInfo.EncryptionSalt)
	if err != nil {
		logger.Error("Failed to derive master key", "error", err)
		fmt.Println("Error: Failed to derive encryption key")
		return
	}

	// Verify password by trying to decrypt an existing filename
	fmt.Print("Verifying password... ")
	if !verifyPassword(masterKey, token, serverURL, logger) {
		fmt.Println("\n✗ Incorrect password. The password you entered does not match your account password.")
		fmt.Println("Please try again with the correct password.")
		return
	}
	fmt.Println("✓")

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		logger.Error("File not found", "path", filePath, "error", err)
		fmt.Println("Error: File not found")
		return
	}

	fmt.Printf("Uploading: %s (%d bytes)\n", filepath.Base(filePath), fileInfo.Size())

	// Read file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error("Failed to read file", "error", err)
		fmt.Println("Error: Failed to read file")
		return
	}

	// Generate a random 32-byte file key
	fileKey := make([]byte, 32)
	if _, err := rand.Read(fileKey); err != nil {
		logger.Error("Failed to generate file key", "error", err)
		fmt.Println("Error: Failed to generate encryption key")
		return
	}

	// Encrypt file with AES-GCM
	iv, ciphertext, err := crypto.EncryptAESGCM(fileKey, fileData)
	if err != nil {
		logger.Error("Failed to encrypt file", "error", err)
		fmt.Println("Error: Failed to encrypt file")
		return
	}

	// Combine IV and ciphertext (IV first, then ciphertext)
	encryptedData := append(iv, ciphertext...)

	// Encrypt the file key with the master key
	fileKeyIV, fileKeyCipher, err := crypto.EncryptAESGCM(masterKey, fileKey)
	if err != nil {
		logger.Error("Failed to encrypt file key", "error", err)
		fmt.Println("Error: Failed to encrypt file key")
		return
	}
	// Use base64 encoding (matching web app)
	encryptedKey := base64.StdEncoding.EncodeToString(append(fileKeyIV, fileKeyCipher...))

	// Encrypt filename with the master key (not file key!)
	filename := filepath.Base(filePath)
	filenameIV, filenameCipher, err := crypto.EncryptAESGCM(masterKey, []byte(filename))
	if err != nil {
		logger.Error("Failed to encrypt filename", "error", err)
		fmt.Println("Error: Failed to encrypt filename")
		return
	}
	// Use base64 encoding (matching web app)
	encryptedFilename := base64.StdEncoding.EncodeToString(append(filenameIV, filenameCipher...))

	// Create multipart form
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add file
	part, err := writer.CreateFormFile("file", "encrypted.bin")
	if err != nil {
		logger.Error("Failed to create form file", "error", err)
		fmt.Println("Error: Failed to prepare upload")
		return
	}
	part.Write(encryptedData)

	// Add metadata
	writer.WriteField("encrypted_key", encryptedKey)
	writer.WriteField("encrypted_filename", encryptedFilename)
	writer.Close()

	// Upload
	uploadURL := fmt.Sprintf("%s://%s/api/files/upload", serverURL.Scheme, serverURL.Host)
	uploadReq, err := http.NewRequest("POST", uploadURL, &requestBody)
	if err != nil {
		logger.Error("Failed to create request", "error", err)
		fmt.Println("Error: Failed to create upload request")
		return
	}

	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+token)

	uploadClient := &http.Client{Timeout: 30 * time.Second}
	uploadResp, err := uploadClient.Do(uploadReq)
	if err != nil {
		logger.Error("Upload failed", "error", err)
		fmt.Println("Error: Upload failed")
		return
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(uploadResp.Body)
		logger.Error("Upload failed", "status", uploadResp.StatusCode, "body", string(body))
		fmt.Printf("Error: Upload failed (HTTP %d)\n", uploadResp.StatusCode)
		return
	}

	fmt.Println("✓ File uploaded successfully!")
}

// deriveMasterKey derives a 256-bit encryption key from password and salt using PBKDF2
// This matches the Web Crypto API implementation in the frontend
func deriveMasterKey(password, saltHex string) ([]byte, error) {
	// Decode hex salt
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return nil, fmt.Errorf("invalid salt: %w", err)
	}

	// Use PBKDF2 with 100,000 iterations, SHA-256, and 32-byte output
	// This matches the frontend: PBKDF2 with 100000 iterations
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
	return key, nil
}

// readPassword reads a password from stdin without echoing
func readPassword() ([]byte, error) {
	// Get the file descriptor for stdin
	fd := int(syscall.Stdin)

	// Read password without echoing
	password, err := term.ReadPassword(fd)
	if err != nil {
		return nil, err
	}

	return password, nil
}

// verifyPassword verifies the password by attempting to decrypt an existing encrypted filename
func verifyPassword(masterKey []byte, token string, serverURL *url.URL, logger *slog.Logger) bool {
	// Fetch user's files to get an encrypted filename
	listURL := fmt.Sprintf("%s://%s/api/files/list", serverURL.Scheme, serverURL.Host)
	req, _ := http.NewRequest("GET", listURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("Failed to fetch files for password verification", "error", err)
		// If we can't fetch files, assume password is correct (user might have no files yet)
		return true
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("Failed to fetch files for password verification", "status", resp.StatusCode)
		// If we can't fetch files, assume password is correct
		return true
	}

	var filesResp struct {
		Files []struct {
			EncryptedFilename string `json:"encrypted_filename"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&filesResp); err != nil {
		logger.Warn("Failed to decode files response", "error", err)
		return true
	}

	// If user has no files yet, we can't verify the password, so assume it's correct
	if len(filesResp.Files) == 0 {
		return true
	}

	// Try to decrypt the first file's encrypted filename
	encryptedFilename := filesResp.Files[0].EncryptedFilename
	if encryptedFilename == "" {
		return true
	}

	// Decode base64
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedFilename)
	if err != nil {
		logger.Warn("Failed to decode encrypted filename", "error", err)
		return true
	}

	// The encrypted data contains IV (first 12 bytes) + ciphertext
	if len(encryptedData) < 12 {
		logger.Warn("Encrypted filename too short", "length", len(encryptedData))
		return true
	}

	iv := encryptedData[:12]
	ciphertext := encryptedData[12:]

	// Try to decrypt
	_, err = crypto.DecryptAESGCM(masterKey, iv, ciphertext)
	if err != nil {
		// Decryption failed - password is incorrect
		logger.Debug("Password verification failed - incorrect password")
		return false
	}

	// Decryption succeeded - password is correct
	return true
}

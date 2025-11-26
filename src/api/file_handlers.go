package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/schollz/e2ecp/src/db"
)

type FileHandlers struct {
	queries           *db.Queries
	logger            *slog.Logger
	freeStorageLimit  int64
	subscriberStorage int64
}

func NewFileHandlers(database *sql.DB, logger *slog.Logger) *FileHandlers {
	freeLimit := parseStorageEnv("FREE_STORAGE_BYTES", 1*1024*1024*1024)               // 1GB default
	subscriberLimit := parseStorageEnv("SUBSCRIBER_STORAGE_BYTES", 100*1024*1024*1024) // 100GB default

	return &FileHandlers{
		queries:           db.New(database),
		logger:            logger,
		freeStorageLimit:  freeLimit,
		subscriberStorage: subscriberLimit,
	}
}

type FileInfo struct {
	ID                int64  `json:"id"`
	EncryptedFilename string `json:"encrypted_filename"`
	FileSize          int64  `json:"file_size"`
	EncryptedKey      string `json:"encrypted_key"`
	ShareToken        string `json:"share_token,omitempty"`
	DownloadCount     int64  `json:"download_count"`
	CreatedAt         string `json:"created_at"`
}

type FilesListResponse struct {
	Files           []FileInfo `json:"files"`
	TotalStorage    int64      `json:"total_storage"`
	StorageLimit    int64      `json:"storage_limit"`
	IsSubscriber    bool       `json:"is_subscriber"`
	FreeLimit       int64      `json:"free_limit"`
	SubscriberLimit int64      `json:"subscriber_limit"`
}

type ConfigResponse struct {
	StorageProfileEnabled bool  `json:"storage_profile_enabled"`
	FreeStorageLimit      int64 `json:"free_storage_limit"`
}

type ShareLinkResponse struct {
	ShareToken string `json:"share_token"`
	ShareURL   string `json:"share_url"`
	FileKey    string `json:"file_key"`
}

// generateShareToken generates a random token for file sharing
func generateShareToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Upload handles file upload
func (h *FileHandlers) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := GetUserID(r)
	if userID == 0 {
		h.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max in memory
		h.writeError(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get encrypted file key from form
	encryptedKey := r.FormValue("encrypted_key")
	if encryptedKey == "" {
		h.writeError(w, "Encrypted key is required", http.StatusBadRequest)
		return
	}

	// Get encrypted filename from form
	encryptedFilename := r.FormValue("encrypted_filename")
	if encryptedFilename == "" {
		h.writeError(w, "Encrypted filename is required", http.StatusBadRequest)
		return
	}

	// Get user and determine storage limit
	user, err := h.queries.GetUserByID(context.Background(), userID)
	if err != nil {
		h.logger.Error("Failed to get user", "error", err)
		h.writeError(w, "Failed to get user", http.StatusInternalServerError)
		return
	}
	storageLimit := h.storageLimitForUser(user)

	// Check current storage usage
	totalStorage, err := h.queries.GetTotalStorageByUserID(context.Background(), userID)
	if err != nil {
		h.logger.Error("Failed to get storage", "error", err)
		h.writeError(w, "Failed to check storage", http.StatusInternalServerError)
		return
	}

	if totalStorage+header.Size > storageLimit {
		remaining := storageLimit - totalStorage
		h.writeError(w, fmt.Sprintf("Storage limit exceeded. You have %d bytes remaining", remaining), http.StatusForbidden)
		return
	}

	// Read file data into memory
	fileData, err := io.ReadAll(file)
	if err != nil {
		h.logger.Error("Failed to read file data", "error", err)
		h.writeError(w, "Failed to read file", http.StatusInternalServerError)
		return
	}
	h.logger.Info("Read encrypted file data", "user_id", userID, "size_bytes", len(fileData))

	// Save file metadata and data to database
	fileRecord, err := h.queries.CreateFile(context.Background(), db.CreateFileParams{
		UserID:            userID,
		EncryptedFilename: encryptedFilename,
		FileSize:          header.Size,
		EncryptedKey:      encryptedKey,
		ShareToken: sql.NullString{
			String: "",
			Valid:  false,
		},
		FileData: fileData,
	})
	if err != nil {
		h.logger.Error("Failed to save file to database", "error", err)
		h.writeError(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	h.logger.Info("File uploaded to database", "user_id", userID, "size", header.Size, "file_id", fileRecord.ID)
	h.writeJSON(w, FileInfo{
		ID:                fileRecord.ID,
		EncryptedFilename: fileRecord.EncryptedFilename,
		FileSize:          fileRecord.FileSize,
		EncryptedKey:      fileRecord.EncryptedKey,
		DownloadCount:     fileRecord.DownloadCount,
		CreatedAt:         fileRecord.CreatedAt.Format("2006-01-02 15:04:05"),
	}, http.StatusCreated)
}

// List handles listing user files
func (h *FileHandlers) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := GetUserID(r)
	if userID == 0 {
		h.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.queries.GetUserByID(context.Background(), userID)
	if err != nil {
		h.logger.Error("Failed to get user", "error", err)
		h.writeError(w, "Failed to get files", http.StatusInternalServerError)
		return
	}

	files, err := h.queries.GetFilesByUserID(context.Background(), userID)
	if err != nil {
		h.logger.Error("Failed to get files", "error", err)
		h.writeError(w, "Failed to get files", http.StatusInternalServerError)
		return
	}

	totalStorage, err := h.queries.GetTotalStorageByUserID(context.Background(), userID)
	if err != nil {
		h.logger.Error("Failed to get storage", "error", err)
		h.writeError(w, "Failed to get storage", http.StatusInternalServerError)
		return
	}

	fileList := make([]FileInfo, len(files))
	for i, f := range files {
		fileList[i] = FileInfo{
			ID:                f.ID,
			EncryptedFilename: f.EncryptedFilename,
			FileSize:          f.FileSize,
			EncryptedKey:      f.EncryptedKey,
			ShareToken:        f.ShareToken.String,
			DownloadCount:     f.DownloadCount,
			CreatedAt:         f.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	storageLimit := h.storageLimitForUser(user)

	h.writeJSON(w, FilesListResponse{
		Files:           fileList,
		TotalStorage:    totalStorage,
		StorageLimit:    storageLimit,
		IsSubscriber:    user.Subscriber == 1,
		FreeLimit:       h.freeStorageLimit,
		SubscriberLimit: h.subscriberStorage,
	}, http.StatusOK)
}

// Download handles file download by ID (requires authentication)
func (h *FileHandlers) Download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := GetUserID(r)
	if userID == 0 {
		h.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fileIDStr := strings.TrimPrefix(r.URL.Path, "/api/files/download/")
	fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
	if err != nil {
		h.writeError(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	file, err := h.queries.GetFileByID(context.Background(), db.GetFileByIDParams{
		ID:     fileID,
		UserID: userID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			h.writeError(w, "File not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to get file", "error", err)
		h.writeError(w, "Failed to get file", http.StatusInternalServerError)
		return
	}

	h.serveFileData(w, file.FileData)
}

// DownloadByToken handles file download by share token (no authentication required)
func (h *FileHandlers) DownloadByToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := strings.TrimPrefix(r.URL.Path, "/api/files/share/")

	file, err := h.queries.GetFileByShareToken(context.Background(), sql.NullString{
		String: token,
		Valid:  true,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			h.writeError(w, "File not found or share link invalid", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to get file by token", "error", err)
		h.writeError(w, "Failed to get file", http.StatusInternalServerError)
		return
	}

	// Increment download counter for share links
	if _, err := h.queries.IncrementDownloadCountByToken(context.Background(), sql.NullString{
		String: token,
		Valid:  true,
	}); err != nil {
		h.logger.Warn("Failed to increment download count", "error", err, "token", token)
	}

	h.serveFileData(w, file.FileData)
}

// GenerateShareLink generates a shareable link for a file
func (h *FileHandlers) GenerateShareLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := GetUserID(r)
	if userID == 0 {
		h.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fileIDStr := strings.TrimPrefix(r.URL.Path, "/api/files/share/")
	fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
	if err != nil {
		h.writeError(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	// Verify file ownership
	_, err = h.queries.GetFileByID(context.Background(), db.GetFileByIDParams{
		ID:     fileID,
		UserID: userID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			h.writeError(w, "File not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to get file", "error", err)
		h.writeError(w, "Failed to get file", http.StatusInternalServerError)
		return
	}

	// Generate share token
	shareToken, err := generateShareToken()
	if err != nil {
		h.logger.Error("Failed to generate share token", "error", err)
		h.writeError(w, "Failed to generate share link", http.StatusInternalServerError)
		return
	}

	// Update file with share token
	file, err := h.queries.UpdateFileShareToken(context.Background(), db.UpdateFileShareTokenParams{
		ShareToken: sql.NullString{
			String: shareToken,
			Valid:  true,
		},
		ID:     fileID,
		UserID: userID,
	})
	if err != nil {
		h.logger.Error("Failed to update share token", "error", err)
		h.writeError(w, "Failed to generate share link", http.StatusInternalServerError)
		return
	}

	shareURL := fmt.Sprintf("/api/files/share/%s", shareToken)
	h.writeJSON(w, ShareLinkResponse{
		ShareToken: file.ShareToken.String,
		ShareURL:   shareURL,
		FileKey:    file.EncryptedKey, // Return encrypted key for client to decrypt
	}, http.StatusOK)
}

// Rekey updates encrypted metadata (filename and file key) after a password change
func (h *FileHandlers) Rekey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := GetUserID(r)
	if userID == 0 {
		h.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Files []struct {
			ID                int64  `json:"id"`
			EncryptedFilename string `json:"encrypted_filename"`
			EncryptedKey      string `json:"encrypted_key"`
		} `json:"files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(body.Files) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	for _, f := range body.Files {
		if f.ID == 0 || f.EncryptedFilename == "" || f.EncryptedKey == "" {
			h.writeError(w, "Each file must include id, encrypted_filename, and encrypted_key", http.StatusBadRequest)
			return
		}

		if err := h.queries.UpdateFileEncryption(context.Background(), db.UpdateFileEncryptionParams{
			EncryptedFilename: f.EncryptedFilename,
			EncryptedKey:      f.EncryptedKey,
			ID:                f.ID,
			UserID:            userID,
		}); err != nil {
			if err == sql.ErrNoRows {
				h.writeError(w, "File not found", http.StatusNotFound)
				return
			}
			h.logger.Error("Failed to re-encrypt file metadata", "error", err, "user_id", userID, "file_id", f.ID)
			h.writeError(w, "Failed to update files", http.StatusInternalServerError)
			return
		}
	}

	h.logger.Info("Re-encrypted files after password change", "user_id", userID, "count", len(body.Files))
	w.WriteHeader(http.StatusNoContent)
}

// Delete handles file deletion
func (h *FileHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := GetUserID(r)
	if userID == 0 {
		h.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	fileIDStr := strings.TrimPrefix(r.URL.Path, "/api/files/")
	fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
	if err != nil {
		h.writeError(w, "Invalid file ID", http.StatusBadRequest)
		return
	}

	// Delete from database (file data is stored in the database)
	if err := h.queries.DeleteFile(context.Background(), db.DeleteFileParams{
		ID:     fileID,
		UserID: userID,
	}); err != nil {
		h.logger.Error("Failed to delete file from database", "error", err)
		h.writeError(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	h.logger.Info("File deleted from database", "user_id", userID, "file_id", fileID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *FileHandlers) serveFileData(w http.ResponseWriter, fileData []byte) {
	if len(fileData) == 0 {
		h.writeError(w, "File data not found", http.StatusNotFound)
		return
	}

	// Don't expose filename - client already has the encrypted filename
	w.Header().Set("Content-Disposition", "attachment")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(fileData)))

	if _, err := w.Write(fileData); err != nil {
		h.logger.Error("Failed to write file data", "error", err)
	}
}

func (h *FileHandlers) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *FileHandlers) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// WriteConfig responds with public configuration values
func (h *FileHandlers) WriteConfig(w http.ResponseWriter) {
	h.writeJSON(w, ConfigResponse{
		StorageProfileEnabled: true,
		FreeStorageLimit:      h.freeStorageLimit,
	}, http.StatusOK)
}

func parseStorageEnv(key string, defaultVal int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseInt(val, 10, 64)
	if err != nil || parsed <= 0 {
		return defaultVal
	}
	return parsed
}

func (h *FileHandlers) storageLimitForUser(user db.User) int64 {
	if user.Subscriber == 1 {
		return h.subscriberStorage
	}
	return h.freeStorageLimit
}

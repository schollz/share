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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/schollz/e2ecp/src/db"
)

const (
	MaxStorageBytes = 2 * 1024 * 1024 * 1024 // 2GB
	UploadDir       = "./uploads"
)

type FileHandlers struct {
	queries *db.Queries
	logger  *slog.Logger
}

func NewFileHandlers(database *sql.DB, logger *slog.Logger) *FileHandlers {
	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		logger.Error("Failed to create upload directory", "error", err)
	}

	return &FileHandlers{
		queries: db.New(database),
		logger:  logger,
	}
}

type FileInfo struct {
	ID         int64  `json:"id"`
	Filename   string `json:"filename"`
	FileSize   int64  `json:"file_size"`
	ShareToken string `json:"share_token,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type FilesListResponse struct {
	Files        []FileInfo `json:"files"`
	TotalStorage int64      `json:"total_storage"`
	StorageLimit int64      `json:"storage_limit"`
}

type ShareLinkResponse struct {
	ShareToken string `json:"share_token"`
	ShareURL   string `json:"share_url"`
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

	// Check current storage usage
	totalStorageRaw, err := h.queries.GetTotalStorageByUserID(context.Background(), userID)
	if err != nil {
		h.logger.Error("Failed to get storage", "error", err)
		h.writeError(w, "Failed to check storage", http.StatusInternalServerError)
		return
	}

	totalStorage, ok := totalStorageRaw.(int64)
	if !ok {
		h.logger.Error("Invalid storage type", "type", fmt.Sprintf("%T", totalStorageRaw))
		h.writeError(w, "Failed to check storage", http.StatusInternalServerError)
		return
	}

	if totalStorage+header.Size > MaxStorageBytes {
		remaining := MaxStorageBytes - totalStorage
		h.writeError(w, fmt.Sprintf("Storage limit exceeded. You have %d bytes remaining", remaining), http.StatusForbidden)
		return
	}

	// Create user directory
	userDir := filepath.Join(UploadDir, fmt.Sprintf("user_%d", userID))
	if err := os.MkdirAll(userDir, 0755); err != nil {
		h.logger.Error("Failed to create user directory", "error", err)
		h.writeError(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Generate unique filename
	timestamp := strconv.FormatInt(header.Size, 10)
	safeFilename := filepath.Base(header.Filename)
	filePath := filepath.Join(userDir, fmt.Sprintf("%s_%s", timestamp, safeFilename))

	// Save file to disk
	dst, err := os.Create(filePath)
	if err != nil {
		h.logger.Error("Failed to create file", "error", err)
		h.writeError(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		h.logger.Error("Failed to write file", "error", err)
		os.Remove(filePath)
		h.writeError(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// Save file metadata to database
	fileRecord, err := h.queries.CreateFile(context.Background(), db.CreateFileParams{
		UserID:   userID,
		Filename: header.Filename,
		FilePath: filePath,
		FileSize: header.Size,
		ShareToken: sql.NullString{
			String: "",
			Valid:  false,
		},
	})
	if err != nil {
		h.logger.Error("Failed to save file metadata", "error", err)
		os.Remove(filePath)
		h.writeError(w, "Failed to save file metadata", http.StatusInternalServerError)
		return
	}

	h.logger.Info("File uploaded", "user_id", userID, "filename", header.Filename, "size", header.Size)
	h.writeJSON(w, FileInfo{
		ID:        fileRecord.ID,
		Filename:  fileRecord.Filename,
		FileSize:  fileRecord.FileSize,
		CreatedAt: fileRecord.CreatedAt.Format("2006-01-02 15:04:05"),
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

	files, err := h.queries.GetFilesByUserID(context.Background(), userID)
	if err != nil {
		h.logger.Error("Failed to get files", "error", err)
		h.writeError(w, "Failed to get files", http.StatusInternalServerError)
		return
	}

	totalStorageRaw, err := h.queries.GetTotalStorageByUserID(context.Background(), userID)
	if err != nil {
		h.logger.Error("Failed to get storage", "error", err)
		h.writeError(w, "Failed to get storage", http.StatusInternalServerError)
		return
	}

	totalStorage, ok := totalStorageRaw.(int64)
	if !ok {
		h.logger.Error("Invalid storage type", "type", fmt.Sprintf("%T", totalStorageRaw))
		h.writeError(w, "Failed to get storage", http.StatusInternalServerError)
		return
	}

	fileList := make([]FileInfo, len(files))
	for i, f := range files {
		fileList[i] = FileInfo{
			ID:         f.ID,
			Filename:   f.Filename,
			FileSize:   f.FileSize,
			ShareToken: f.ShareToken.String,
			CreatedAt:  f.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	h.writeJSON(w, FilesListResponse{
		Files:        fileList,
		TotalStorage: totalStorage,
		StorageLimit: MaxStorageBytes,
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

	h.serveFile(w, r, file.FilePath, file.Filename)
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

	h.serveFile(w, r, file.FilePath, file.Filename)
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
	}, http.StatusOK)
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

	// Get file info first
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

	// Delete from database
	if err := h.queries.DeleteFile(context.Background(), db.DeleteFileParams{
		ID:     fileID,
		UserID: userID,
	}); err != nil {
		h.logger.Error("Failed to delete file from database", "error", err)
		h.writeError(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	// Delete from filesystem
	if err := os.Remove(file.FilePath); err != nil {
		h.logger.Warn("Failed to delete file from disk", "error", err, "path", file.FilePath)
	}

	h.logger.Info("File deleted", "user_id", userID, "file_id", fileID, "filename", file.Filename)
	w.WriteHeader(http.StatusNoContent)
}

func (h *FileHandlers) serveFile(w http.ResponseWriter, r *http.Request, filePath, filename string) {
	file, err := os.Open(filePath)
	if err != nil {
		h.logger.Error("Failed to open file", "error", err)
		h.writeError(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		h.logger.Error("Failed to stat file", "error", err)
		h.writeError(w, "Failed to get file info", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))

	http.ServeContent(w, r, filename, stat.ModTime(), file)
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

package relay

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/schollz/e2ecp/src/auth"
)

// AuthRequest represents login/register request body
type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents login/register response
type AuthResponse struct {
	Token string             `json:"token"`
	User  *auth.UserResponse `json:"user"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// Helper function to get base URL from request
func getBaseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		// Check for X-Forwarded-Proto header (common with reverse proxies)
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host
}

// Helper function to extract JWT token from Authorization header
func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return parts[1]
}

// registerHandler handles user registration
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Email and password are required"})
		return
	}

	if len(req.Password) < 6 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Password must be at least 6 characters"})
		return
	}

	authService := auth.GetAuthService()
	if authService == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Auth service not initialized"})
		return
	}

	user, token, err := authService.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		if err == auth.ErrEmailExists {
			writeJSON(w, http.StatusConflict, ErrorResponse{Error: "Email already exists"})
			return
		}
		logger.Error("Registration failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Registration failed"})
		return
	}

	writeJSON(w, http.StatusCreated, AuthResponse{Token: token, User: user})
}

// loginHandler handles user login
func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Email and password are required"})
		return
	}

	authService := auth.GetAuthService()
	if authService == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Auth service not initialized"})
		return
	}

	user, token, err := authService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid email or password"})
			return
		}
		logger.Error("Login failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Login failed"})
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{Token: token, User: user})
}

// profileHandler returns the current user's profile
func profileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Authorization required"})
		return
	}

	authService := auth.GetAuthService()
	if authService == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Auth service not initialized"})
		return
	}

	claims, err := authService.ValidateToken(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid or expired token"})
		return
	}

	user, err := authService.GetUserProfile(r.Context(), claims.UserID)
	if err != nil {
		logger.Error("Failed to get profile", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to get profile"})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// filesHandler handles listing and uploading files
func filesHandler(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Authorization required"})
		return
	}

	authService := auth.GetAuthService()
	if authService == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Auth service not initialized"})
		return
	}

	claims, err := authService.ValidateToken(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid or expired token"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		// List files
		baseURL := getBaseURL(r)
		files, err := authService.ListFiles(r.Context(), claims.UserID, baseURL)
		if err != nil {
			logger.Error("Failed to list files", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to list files"})
			return
		}
		writeJSON(w, http.StatusOK, files)

	case http.MethodPost:
		// Upload file
		// Limit to 100MB per upload
		r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024)
		
		err := r.ParseMultipartForm(100 * 1024 * 1024)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to parse form"})
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "File is required"})
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to read file"})
			return
		}

		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		fileResp, err := authService.UploadFile(r.Context(), claims.UserID, header.Filename, contentType, data)
		if err != nil {
			if err == auth.ErrStorageLimitExceeded {
				writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Storage limit exceeded (2GB)"})
				return
			}
			logger.Error("Failed to upload file", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to upload file"})
			return
		}

		writeJSON(w, http.StatusCreated, fileResp)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// fileHandler handles individual file operations (download, delete, share)
func fileHandler(w http.ResponseWriter, r *http.Request) {
	token := extractToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Authorization required"})
		return
	}

	authService := auth.GetAuthService()
	if authService == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Auth service not initialized"})
		return
	}

	claims, err := authService.ValidateToken(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid or expired token"})
		return
	}

	// Extract file ID from URL path: /api/files/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/files/")
	fileID := strings.Split(path, "/")[0]
	
	if fileID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "File ID required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Download file
		file, err := authService.DownloadFile(r.Context(), fileID, claims.UserID)
		if err != nil {
			if err == auth.ErrFileNotFound {
				writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "File not found"})
				return
			}
			if err == auth.ErrUnauthorized {
				writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "Access denied"})
				return
			}
			logger.Error("Failed to download file", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to download file"})
			return
		}

		w.Header().Set("Content-Type", file.ContentType.String)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Filename+"\"")
		w.Write(file.Data)

	case http.MethodDelete:
		// Delete file
		err := authService.DeleteFile(r.Context(), fileID, claims.UserID)
		if err != nil {
			if err == auth.ErrFileNotFound {
				writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "File not found"})
				return
			}
			logger.Error("Failed to delete file", "error", err)
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete file"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"message": "File deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// shareHandler generates a share link for a file
func shareHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Authorization required"})
		return
	}

	authService := auth.GetAuthService()
	if authService == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Auth service not initialized"})
		return
	}

	claims, err := authService.ValidateToken(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid or expired token"})
		return
	}

	// Extract file ID from URL path: /api/files/{id}/share
	path := strings.TrimPrefix(r.URL.Path, "/api/files/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "share" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid request"})
		return
	}
	fileID := parts[0]

	baseURL := getBaseURL(r)
	shareURL, err := authService.GenerateShareLink(r.Context(), fileID, claims.UserID, baseURL)
	if err != nil {
		if err == auth.ErrFileNotFound {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "File not found"})
			return
		}
		if err == auth.ErrUnauthorized {
			writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "Access denied"})
			return
		}
		logger.Error("Failed to generate share link", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate share link"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"share_url": shareURL})
}

// publicShareHandler handles public file downloads via share token
func publicShareHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authService := auth.GetAuthService()
	if authService == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Auth service not initialized"})
		return
	}

	// Extract share token from URL path: /api/share/{token}
	shareToken := strings.TrimPrefix(r.URL.Path, "/api/share/")
	if shareToken == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Share token required"})
		return
	}

	file, err := authService.DownloadSharedFile(r.Context(), shareToken)
	if err != nil {
		if err == auth.ErrFileNotFound {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "File not found or share link expired"})
			return
		}
		logger.Error("Failed to download shared file", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to download file"})
		return
	}

	w.Header().Set("Content-Type", file.ContentType.String)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Filename+"\"")
	w.Write(file.Data)
}

// verifyHandler verifies if a JWT token is valid
func verifyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Authorization required"})
		return
	}

	authService := auth.GetAuthService()
	if authService == nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Auth service not initialized"})
		return
	}

	claims, err := authService.ValidateToken(token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid or expired token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":   true,
		"user_id": claims.UserID,
		"email":   claims.Email,
	})
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// RegisterAuthHandlers registers the authentication and file management handlers
func RegisterAuthHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/register", registerHandler)
	mux.HandleFunc("/api/auth/login", loginHandler)
	mux.HandleFunc("/api/auth/verify", verifyHandler)
	mux.HandleFunc("/api/profile", profileHandler)
	mux.HandleFunc("/api/files", filesHandler)
	mux.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		// Route to appropriate handler based on path
		path := strings.TrimPrefix(r.URL.Path, "/api/files/")
		if strings.Contains(path, "/share") {
			shareHandler(w, r)
		} else {
			fileHandler(w, r)
		}
	})
	mux.HandleFunc("/api/share/", publicShareHandler)
}

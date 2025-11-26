package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/schollz/e2ecp/src/auth"
)

type AuthHandlers struct {
	authService *auth.Service
	logger      *slog.Logger
}

func NewAuthHandlers(authService *auth.Service, logger *slog.Logger) *AuthHandlers {
	return &AuthHandlers{
		authService: authService,
		logger:      logger,
	}
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type User struct {
	ID             int64  `json:"id"`
	Email          string `json:"email"`
	EncryptionSalt string `json:"encryption_salt"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Register handles user registration
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		h.writeError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 6 {
		h.writeError(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	user, token, err := h.authService.Register(req.Email, req.Password)
	if err != nil {
		if err == auth.ErrUserExists {
			h.writeError(w, "User already exists", http.StatusConflict)
			return
		}
		h.logger.Error("Registration failed", "error", err)
		h.writeError(w, "Registration failed", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, AuthResponse{
		Token: token,
		User: User{
			ID:             user.ID,
			Email:          user.Email,
			EncryptionSalt: user.EncryptionSalt,
		},
	}, http.StatusCreated)
}

// Login handles user login
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		h.writeError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	user, token, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			h.writeError(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		h.logger.Error("Login failed", "error", err)
		h.writeError(w, "Login failed", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, AuthResponse{
		Token: token,
		User: User{
			ID:             user.ID,
			Email:          user.Email,
			EncryptionSalt: user.EncryptionSalt,
		},
	}, http.StatusOK)
}

// Verify handles JWT token verification
func (h *AuthHandlers) Verify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := GetUserID(r)
	email := GetUserEmail(r)

	h.writeJSON(w, User{
		ID:    userID,
		Email: email,
	}, http.StatusOK)
}

func (h *AuthHandlers) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *AuthHandlers) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

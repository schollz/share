package api

import (
	"encoding/json"
	"errors"
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
	Email         string `json:"email"`
	Password      string `json:"password"`
	CaptchaAnswer int    `json:"captcha_answer"`
	CaptchaToken  string `json:"captcha_token"`
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
	Subscriber     bool   `json:"subscriber"`
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

	// Accept RFC-5322-friendly emails (including +) by relying on downstream validation

	if req.Email == "" || req.Password == "" {
		h.writeError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 6 {
		h.writeError(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	if req.CaptchaToken == "" {
		h.writeError(w, "Captcha is required", http.StatusBadRequest)
		return
	}

	if err := h.authService.VerifyCaptcha(req.CaptchaToken, req.CaptchaAnswer); err != nil {
		if errors.Is(err, auth.ErrInvalidCaptcha) {
			h.writeError(w, "Captcha is invalid or expired", http.StatusBadRequest)
			return
		}
		if errors.Is(err, auth.ErrCaptchaMismatch) {
			h.writeError(w, "Captcha answer incorrect", http.StatusBadRequest)
			return
		}
		h.logger.Error("Captcha validation failed", "error", err)
		h.writeError(w, "Captcha validation failed", http.StatusInternalServerError)
		return
	}

	_, _, err := h.authService.Register(req.Email, req.Password)
	if err != nil {
		if err == auth.ErrUserExists {
			h.writeError(w, "User already exists", http.StatusConflict)
			return
		}
		h.logger.Error("Registration failed", "error", err)
		h.writeError(w, "Registration failed", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, map[string]string{
		"message": "Verification email sent. Please check your inbox.",
	}, http.StatusAccepted)
}

// Captcha returns a simple math challenge and token for registration
func (h *AuthHandlers) Captcha(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	prompt, token, err := h.authService.GenerateCaptcha()
	if err != nil {
		h.logger.Error("Failed to generate captcha", "error", err)
		h.writeError(w, "Failed to generate captcha", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, map[string]string{
		"prompt": prompt,
		"token":  token,
	}, http.StatusOK)
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
		if err == auth.ErrEmailNotVerified {
			h.writeError(w, "Email not verified. Please check your email.", http.StatusForbidden)
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
			Subscriber:     user.Subscriber == 1,
		},
	}, http.StatusOK)
}

// ChangePassword handles password updates for authenticated users
func (h *AuthHandlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
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
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if body.CurrentPassword == "" || body.NewPassword == "" {
		h.writeError(w, "Current password and new password are required", http.StatusBadRequest)
		return
	}
	if len(body.NewPassword) < 6 {
		h.writeError(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	if err := h.authService.ChangePassword(userID, body.CurrentPassword, body.NewPassword); err != nil {
		if err == auth.ErrInvalidCredentials {
			h.writeError(w, "Current password is incorrect", http.StatusUnauthorized)
			return
		}
		h.logger.Error("Change password failed", "error", err)
		h.writeError(w, "Failed to change password", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteAccount deletes a user and all associated data (requires password)
func (h *AuthHandlers) DeleteAccount(w http.ResponseWriter, r *http.Request) {
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
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if body.Password == "" {
		h.writeError(w, "Password is required", http.StatusBadRequest)
		return
	}

	if err := h.authService.DeleteAccount(userID, body.Password); err != nil {
		if err == auth.ErrInvalidCredentials {
			h.writeError(w, "Password is incorrect", http.StatusUnauthorized)
			return
		}
		h.logger.Error("Delete account failed", "error", err)
		h.writeError(w, "Failed to delete account", http.StatusInternalServerError)
		return
	}

	// File data is stored in database and will be deleted via CASCADE
	w.WriteHeader(http.StatusNoContent)
}

// VerifyEmailToken verifies an email via token and signs user in
func (h *AuthHandlers) VerifyEmailToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	user, jwtToken, err := h.authService.VerifyEmail(token)
	if err != nil {
		if err == auth.ErrInvalidToken {
			h.writeError(w, "Invalid or expired token", http.StatusBadRequest)
			return
		}
		h.logger.Error("Email verification failed", "error", err)
		h.writeError(w, "Verification failed", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, AuthResponse{
		Token: jwtToken,
		User: User{
			ID:             user.ID,
			Email:          user.Email,
			EncryptionSalt: user.EncryptionSalt,
			Subscriber:     user.Subscriber == 1,
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
	if userID == 0 {
		h.writeError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		h.logger.Error("Failed to fetch user", "error", err)
		h.writeError(w, "Failed to verify user", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, User{
		ID:             user.ID,
		Email:          user.Email,
		EncryptionSalt: user.EncryptionSalt,
		Subscriber:     user.Subscriber == 1,
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

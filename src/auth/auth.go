package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/schollz/e2ecp/src/db"
	"golang.org/x/crypto/bcrypt"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrInvalidCaptcha     = errors.New("invalid captcha token")
	ErrCaptchaMismatch    = errors.New("captcha answer incorrect")
)

// Service handles authentication operations
type Service struct {
	queries   *db.Queries
	jwtSecret []byte
	logger    *slog.Logger
}

// NewService creates a new authentication service
func NewService(database *sql.DB, jwtSecret string, logger *slog.Logger) *Service {
	return &Service{
		queries:   db.New(database),
		jwtSecret: []byte(jwtSecret),
		logger:    logger,
	}
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// VerifyPassword checks if a password matches the hash
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// GenerateJWT generates a JWT token for a user
func (s *Service) GenerateJWT(userID int64, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateJWT validates a JWT token and returns the claims
func (s *Service) ValidateJWT(tokenString string) (int64, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return 0, "", ErrInvalidToken
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := int64(claims["user_id"].(float64))
		email := claims["email"].(string)
		return userID, email, nil
	}

	return 0, "", ErrInvalidToken
}

// GenerateCaptcha creates a simple math captcha challenge with a signed token
func (s *Service) GenerateCaptcha() (string, string, error) {
	first, err := randomInt(2, 9)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate captcha: %w", err)
	}
	second, err := randomInt(1, 9)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate captcha: %w", err)
	}

	payload := fmt.Sprintf("%d:%d", first, second)
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(payload))
	token := fmt.Sprintf("%s:%s", payload, hex.EncodeToString(mac.Sum(nil)))

	return fmt.Sprintf("What is %d + %d?", first, second), token, nil
}

// VerifyCaptcha validates the token signature and supplied answer
func (s *Service) VerifyCaptcha(token string, answer int) error {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return ErrInvalidCaptcha
	}

	payload := strings.Join(parts[:2], ":")
	expectedSig, err := hex.DecodeString(parts[2])
	if err != nil {
		return ErrInvalidCaptcha
	}

	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(payload))
	if !hmac.Equal(expectedSig, mac.Sum(nil)) {
		return ErrInvalidCaptcha
	}

	first, err := strconv.Atoi(parts[0])
	if err != nil {
		return ErrInvalidCaptcha
	}
	second, err := strconv.Atoi(parts[1])
	if err != nil {
		return ErrInvalidCaptcha
	}

	if first+second != answer {
		return ErrCaptchaMismatch
	}

	return nil
}

// generateSalt generates a random salt for encryption key derivation
func generateSalt() (string, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return hex.EncodeToString(salt), nil
}

func randomInt(min, max int) (int, error) {
	if min > max {
		return 0, fmt.Errorf("invalid range %d-%d", min, max)
	}

	diff := max - min + 1
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(diff)))
	if err != nil {
		return 0, err
	}

	return int(nBig.Int64()) + min, nil
}

// Register creates a new user account
func (s *Service) Register(email, password string) (*db.User, string, error) {
	// Hash the password
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return nil, "", err
	}

	// Generate encryption salt for client-side file encryption
	encryptionSalt, err := generateSalt()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate encryption salt: %w", err)
	}

	// Generate verification token
	verificationToken, err := generateSalt()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate verification token: %w", err)
	}

	// Create the user
	user, err := s.queries.CreateUser(context.Background(), db.CreateUserParams{
		Email:          email,
		PasswordHash:   hashedPassword,
		EncryptionSalt: encryptionSalt,
		Subscriber:     0,
		Verified:       0,
		VerificationToken: sql.NullString{
			String: verificationToken,
			Valid:  true,
		},
	})
	if err != nil {
		var sqliteErr interface{ Code() int }
		if errors.As(err, &sqliteErr) {
			switch sqliteErr.Code() {
			case sqlite3.SQLITE_CONSTRAINT, sqlite3.SQLITE_CONSTRAINT_UNIQUE:
				return nil, "", ErrUserExists
			}
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
			return nil, "", ErrUserExists
		}
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	// Send verification email
	if err := s.sendVerificationEmail(user.Email, verificationToken); err != nil {
		s.logger.Warn("Failed to send verification email", "error", err, "email", email)
	}

	s.logger.Info("User registered", "email", email, "user_id", user.ID, "needs_verification", true)
	return &user, "", nil
}

// Login authenticates a user and returns a JWT token
func (s *Service) Login(email, password string) (*db.User, string, error) {
	// Get user by email
	user, err := s.queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}

	// Verify password
	if err := VerifyPassword(user.PasswordHash, password); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	if user.Verified == 0 {
		return nil, "", ErrEmailNotVerified
	}

	// Generate JWT token
	token, err := s.GenerateJWT(user.ID, user.Email)
	if err != nil {
		return nil, "", err
	}

	s.logger.Info("User logged in", "email", email, "user_id", user.ID)
	return &user, token, nil
}

// GetUserByID retrieves a user by their ID
func (s *Service) GetUserByID(userID int64) (*db.User, error) {
	user, err := s.queries.GetUserByID(context.Background(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// ChangePassword updates the user's password after verifying the current password
func (s *Service) ChangePassword(userID int64, currentPassword, newPassword string) error {
	user, err := s.queries.GetUserByID(context.Background(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("user not found")
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Verify current password
	if err := VerifyPassword(user.PasswordHash, currentPassword); err != nil {
		return ErrInvalidCredentials
	}

	// Hash new password
	hashed, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update in DB
	if err := s.queries.UpdateUserPassword(context.Background(), db.UpdateUserPasswordParams{
		PasswordHash: hashed,
		ID:           userID,
	}); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	s.logger.Info("Password changed", "user_id", userID)
	return nil
}

// DeleteAccount removes a user and their data after verifying password
func (s *Service) DeleteAccount(userID int64, currentPassword string) error {
	user, err := s.queries.GetUserByID(context.Background(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("user not found")
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Verify current password
	if err := VerifyPassword(user.PasswordHash, currentPassword); err != nil {
		return ErrInvalidCredentials
	}

	// Delete user (cascades to files)
	if err := s.queries.DeleteUserByID(context.Background(), userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	s.logger.Info("User deleted", "user_id", userID, "email", user.Email)
	return nil
}

// VerifyEmail verifies a user by verification token and issues a JWT
func (s *Service) VerifyEmail(token string) (*db.User, string, error) {
	if token == "" {
		return nil, "", ErrInvalidToken
	}

	// Get user by token
	user, err := s.queries.GetUserByVerificationToken(context.Background(), sql.NullString{
		String: token,
		Valid:  true,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", ErrInvalidToken
		}
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}

	// If already verified, continue
	if user.Verified == 0 {
		// Mark verified
		user, err = s.queries.VerifyUserByToken(context.Background(), sql.NullString{
			String: token,
			Valid:  true,
		})
		if err != nil {
			return nil, "", fmt.Errorf("failed to verify user: %w", err)
		}
	}

	jwtToken, err := s.GenerateJWT(user.ID, user.Email)
	if err != nil {
		return nil, "", err
	}

	return &user, jwtToken, nil
}

func (s *Service) sendVerificationEmail(email, token string) error {
	apiKey := os.Getenv("MAILJET_API_KEY")
	apiSecret := os.Getenv("MAILJET_API_SECRET")
	if apiKey == "" || apiSecret == "" {
		return fmt.Errorf("mailjet credentials not configured")
	}

	appBase := os.Getenv("APP_BASE_URL")
	if appBase == "" {
		appBase = "https://e2ecp.com"
	}
	appBase = strings.TrimRight(appBase, "/")
	verifyLink := fmt.Sprintf("%s/verify-email?token=%s", appBase, token)

	payload := map[string]interface{}{
		"Messages": []map[string]interface{}{
			{
				"From": map[string]string{
					"Email": "no-reply@e2ecp.com",
					"Name":  "E2ECP",
				},
				"To": []map[string]string{
					{
						"Email": email,
					},
				},
				"Subject":  "Verify your email",
				"TextPart": fmt.Sprintf("Verify your email: %s", verifyLink),
				"HTMLPart": fmt.Sprintf("<p>Click to verify your email:</p><p><a href=\"%s\">Verify Email</a></p>", verifyLink),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.mailjet.com/v3.1/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.SetBasicAuth(apiKey, apiSecret)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("mailjet send failed: %s", resp.Status)
	}
	return nil
}

package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/schollz/e2ecp/src/db"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid or expired token")
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

// Register creates a new user account
func (s *Service) Register(email, password string) (*db.User, string, error) {
	// Hash the password
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return nil, "", err
	}

	// Create the user
	user, err := s.queries.CreateUser(context.Background(), db.CreateUserParams{
		Email:        email,
		PasswordHash: hashedPassword,
	})
	if err != nil {
		if err.Error() == "UNIQUE constraint failed: users.email" {
			return nil, "", ErrUserExists
		}
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	// Generate JWT token
	token, err := s.GenerateJWT(user.ID, user.Email)
	if err != nil {
		return nil, "", err
	}

	s.logger.Info("User registered", "email", email, "user_id", user.ID)
	return &user, token, nil
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

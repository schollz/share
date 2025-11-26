package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	_ "modernc.org/sqlite"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
	ErrStorageLimitExceeded = errors.New("storage limit exceeded")
	ErrFileNotFound       = errors.New("file not found")
	ErrUnauthorized       = errors.New("unauthorized")
)

// AuthService handles user authentication and file management
type AuthService struct {
	db      *sql.DB
	queries *Queries
	logger  *slog.Logger
	jwtKey  []byte
	mutex   sync.Mutex
}

// Claims represents JWT claims
type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// UserResponse represents user data returned to client
type UserResponse struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	StorageUsed  int64     `json:"storage_used"`
	StorageLimit int64     `json:"storage_limit"`
	CreatedAt    time.Time `json:"created_at"`
}

// FileResponse represents file data returned to client
type FileResponse struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	ShareToken  *string   `json:"share_token,omitempty"`
	ShareURL    string    `json:"share_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

var (
	authService     *AuthService
	authServiceOnce sync.Once
)

// InitAuthService initializes the auth service with the given database path
func InitAuthService(dbPath string, jwtSecret string, log *slog.Logger) error {
	var err error
	authServiceOnce.Do(func() {
		authService = &AuthService{
			logger: log,
			jwtKey: []byte(jwtSecret),
		}

		authService.db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			err = fmt.Errorf("failed to open auth database: %w", err)
			return
		}

		// Create tables
		if err = authService.createTables(); err != nil {
			err = fmt.Errorf("failed to create auth tables: %w", err)
			return
		}

		authService.queries = New(authService.db)
		log.Info("Auth service initialized", "database", dbPath)
	})

	return err
}

// GetAuthService returns the singleton auth service instance
func GetAuthService() *AuthService {
	return authService
}

func (s *AuthService) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		storage_used INTEGER DEFAULT 0,
		storage_limit INTEGER DEFAULT 2147483648,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		filename TEXT NOT NULL,
		size INTEGER NOT NULL,
		content_type TEXT DEFAULT 'application/octet-stream',
		data BLOB NOT NULL,
		share_token TEXT UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
	CREATE INDEX IF NOT EXISTS idx_files_share_token ON files(share_token);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, email, password string) (*UserResponse, string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if email already exists
	_, err := s.queries.GetUserByEmail(ctx, email)
	if err == nil {
		return nil, "", ErrEmailExists
	}
	if err != sql.ErrNoRows {
		return nil, "", fmt.Errorf("failed to check email: %w", err)
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user, err := s.queries.CreateUser(ctx, CreateUserParams{
		Email:        email,
		PasswordHash: string(hash),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	// Generate JWT token
	token, err := s.generateToken(user.ID, email)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return &UserResponse{
		ID:           user.ID,
		Email:        email,
		StorageUsed:  user.StorageUsed.Int64,
		StorageLimit: user.StorageLimit.Int64,
		CreatedAt:    user.CreatedAt.Time,
	}, token, nil
}

// Login authenticates a user and returns a JWT token
func (s *AuthService) Login(ctx context.Context, email, password string) (*UserResponse, string, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", fmt.Errorf("failed to get user: %w", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := s.generateToken(user.ID, email)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return &UserResponse{
		ID:           user.ID,
		Email:        user.Email,
		StorageUsed:  user.StorageUsed.Int64,
		StorageLimit: user.StorageLimit.Int64,
		CreatedAt:    user.CreatedAt.Time,
	}, token, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, ErrUnauthorized
	}

	return claims, nil
}

// GetUserProfile returns the user profile for a given user ID
func (s *AuthService) GetUserProfile(ctx context.Context, userID int64) (*UserResponse, error) {
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &UserResponse{
		ID:           user.ID,
		Email:        user.Email,
		StorageUsed:  user.StorageUsed.Int64,
		StorageLimit: user.StorageLimit.Int64,
		CreatedAt:    user.CreatedAt.Time,
	}, nil
}

// UploadFile uploads a file for a user
func (s *AuthService) UploadFile(ctx context.Context, userID int64, filename, contentType string, data []byte) (*FileResponse, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Get user to check storage limit
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	fileSize := int64(len(data))
	newStorageUsed := user.StorageUsed.Int64 + fileSize

	if newStorageUsed > user.StorageLimit.Int64 {
		return nil, ErrStorageLimitExceeded
	}

	// Generate file ID
	fileID := generateUUID()

	// Create file
	file, err := s.queries.CreateFile(ctx, CreateFileParams{
		ID:          fileID,
		UserID:      userID,
		Filename:    filename,
		Size:        fileSize,
		ContentType: sql.NullString{String: contentType, Valid: contentType != ""},
		Data:        data,
		ShareToken:  sql.NullString{Valid: false},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	// Update user storage used
	err = s.queries.UpdateUserStorageUsed(ctx, UpdateUserStorageUsedParams{
		StorageUsed: sql.NullInt64{Int64: newStorageUsed, Valid: true},
		ID:          userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update storage used: %w", err)
	}

	return &FileResponse{
		ID:          file.ID,
		Filename:    file.Filename,
		Size:        file.Size,
		ContentType: file.ContentType.String,
		CreatedAt:   file.CreatedAt.Time,
	}, nil
}

// ListFiles returns all files for a user
func (s *AuthService) ListFiles(ctx context.Context, userID int64, baseURL string) ([]FileResponse, error) {
	files, err := s.queries.ListFilesByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	result := make([]FileResponse, len(files))
	for i, f := range files {
		result[i] = FileResponse{
			ID:          f.ID,
			Filename:    f.Filename,
			Size:        f.Size,
			ContentType: f.ContentType.String,
			CreatedAt:   f.CreatedAt.Time,
		}
		if f.ShareToken.Valid {
			result[i].ShareToken = &f.ShareToken.String
			result[i].ShareURL = fmt.Sprintf("%s/api/share/%s", baseURL, f.ShareToken.String)
		}
	}

	return result, nil
}

// DownloadFile downloads a file by ID (owned by user)
func (s *AuthService) DownloadFile(ctx context.Context, fileID string, userID int64) (*File, error) {
	file, err := s.queries.GetFileByID(ctx, fileID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	if file.UserID != userID {
		return nil, ErrUnauthorized
	}

	return &file, nil
}

// DownloadSharedFile downloads a file by share token (public access)
func (s *AuthService) DownloadSharedFile(ctx context.Context, shareToken string) (*File, error) {
	file, err := s.queries.GetFileByShareToken(ctx, sql.NullString{String: shareToken, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return &file, nil
}

// GenerateShareLink generates a share token for a file
func (s *AuthService) GenerateShareLink(ctx context.Context, fileID string, userID int64, baseURL string) (string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Verify file belongs to user
	file, err := s.queries.GetFileByID(ctx, fileID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrFileNotFound
		}
		return "", fmt.Errorf("failed to get file: %w", err)
	}

	if file.UserID != userID {
		return "", ErrUnauthorized
	}

	// Generate share token if not exists
	shareToken := file.ShareToken.String
	if !file.ShareToken.Valid || shareToken == "" {
		shareToken = generateShareToken()
		err = s.queries.UpdateFileShareToken(ctx, UpdateFileShareTokenParams{
			ShareToken: sql.NullString{String: shareToken, Valid: true},
			ID:         fileID,
			UserID:     userID,
		})
		if err != nil {
			return "", fmt.Errorf("failed to update share token: %w", err)
		}
	}

	return fmt.Sprintf("%s/api/share/%s", baseURL, shareToken), nil
}

// DeleteFile deletes a file and updates storage used
func (s *AuthService) DeleteFile(ctx context.Context, fileID string, userID int64) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Get file size first
	fileSize, err := s.queries.GetFileSizeByID(ctx, GetFileSizeByIDParams{
		ID:     fileID,
		UserID: userID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to get file size: %w", err)
	}

	// Delete file
	err = s.queries.DeleteFile(ctx, DeleteFileParams{
		ID:     fileID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Update user storage used
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	newStorageUsed := user.StorageUsed.Int64 - fileSize
	if newStorageUsed < 0 {
		newStorageUsed = 0
	}

	err = s.queries.UpdateUserStorageUsed(ctx, UpdateUserStorageUsedParams{
		StorageUsed: sql.NullInt64{Int64: newStorageUsed, Valid: true},
		ID:          userID,
	})
	if err != nil {
		return fmt.Errorf("failed to update storage used: %w", err)
	}

	return nil
}

// generateToken creates a JWT token for a user
func (s *AuthService) generateToken(userID int64, email string) (string, error) {
	expirationTime := time.Now().Add(7 * 24 * time.Hour) // 7 days

	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "e2ecp",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtKey)
}

// Close closes the database connection
func (s *AuthService) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Helper functions
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func generateShareToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

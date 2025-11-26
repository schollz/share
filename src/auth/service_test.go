package auth

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
)

func TestAuthService(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "auth_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Reset the singleton for testing
	authService = nil
	authServiceOnce = sync.Once{}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Initialize auth service
	err = InitAuthService(tmpFile.Name(), "test-secret-key", logger)
	if err != nil {
		t.Fatalf("Failed to initialize auth service: %v", err)
	}

	ctx := context.Background()
	service := GetAuthService()

	// Test registration
	t.Run("Register", func(t *testing.T) {
		user, token, err := service.Register(ctx, "test@example.com", "password123")
		if err != nil {
			t.Fatalf("Registration failed: %v", err)
		}

		if user.Email != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got '%s'", user.Email)
		}

		if token == "" {
			t.Error("Expected token to be non-empty")
		}

		if user.StorageLimit != 2147483648 { // 2GB
			t.Errorf("Expected storage limit 2147483648, got %d", user.StorageLimit)
		}
	})

	// Test duplicate registration
	t.Run("DuplicateRegistration", func(t *testing.T) {
		_, _, err := service.Register(ctx, "test@example.com", "password456")
		if err != ErrEmailExists {
			t.Errorf("Expected ErrEmailExists, got: %v", err)
		}
	})

	// Test login
	t.Run("Login", func(t *testing.T) {
		user, token, err := service.Login(ctx, "test@example.com", "password123")
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		if user.Email != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got '%s'", user.Email)
		}

		if token == "" {
			t.Error("Expected token to be non-empty")
		}
	})

	// Test invalid login
	t.Run("InvalidLogin", func(t *testing.T) {
		_, _, err := service.Login(ctx, "test@example.com", "wrongpassword")
		if err != ErrInvalidCredentials {
			t.Errorf("Expected ErrInvalidCredentials, got: %v", err)
		}
	})

	// Test token validation
	t.Run("TokenValidation", func(t *testing.T) {
		_, token, err := service.Login(ctx, "test@example.com", "password123")
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		claims, err := service.ValidateToken(token)
		if err != nil {
			t.Fatalf("Token validation failed: %v", err)
		}

		if claims.Email != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got '%s'", claims.Email)
		}
	})

	// Test file upload and storage tracking
	t.Run("FileUpload", func(t *testing.T) {
		user, token, _ := service.Login(ctx, "test@example.com", "password123")

		claims, _ := service.ValidateToken(token)
		userID := claims.UserID

		testData := []byte("Hello, World!")
		fileResp, err := service.UploadFile(ctx, userID, "test.txt", "text/plain", testData)
		if err != nil {
			t.Fatalf("File upload failed: %v", err)
		}

		if fileResp.Filename != "test.txt" {
			t.Errorf("Expected filename 'test.txt', got '%s'", fileResp.Filename)
		}

		// Check storage was updated
		updatedUser, _ := service.GetUserProfile(ctx, userID)
		if updatedUser.StorageUsed != int64(len(testData)) {
			t.Errorf("Expected storage used %d, got %d", len(testData), updatedUser.StorageUsed)
		}

		// Verify user.StorageUsed was updated (use consistent state)
		_ = user // silence unused variable
	})

	// Test file listing
	t.Run("ListFiles", func(t *testing.T) {
		_, token, _ := service.Login(ctx, "test@example.com", "password123")
		claims, _ := service.ValidateToken(token)

		files, err := service.ListFiles(ctx, claims.UserID, "http://localhost:3001")
		if err != nil {
			t.Fatalf("List files failed: %v", err)
		}

		if len(files) != 1 {
			t.Errorf("Expected 1 file, got %d", len(files))
		}
	})

	// Test share link generation
	t.Run("ShareLink", func(t *testing.T) {
		_, token, _ := service.Login(ctx, "test@example.com", "password123")
		claims, _ := service.ValidateToken(token)

		files, _ := service.ListFiles(ctx, claims.UserID, "http://localhost:3001")
		if len(files) == 0 {
			t.Fatal("No files to share")
		}

		shareURL, err := service.GenerateShareLink(ctx, files[0].ID, claims.UserID, "http://localhost:3001")
		if err != nil {
			t.Fatalf("Generate share link failed: %v", err)
		}

		if shareURL == "" {
			t.Error("Expected share URL to be non-empty")
		}
	})

	// Test file download via share link
	t.Run("DownloadSharedFile", func(t *testing.T) {
		_, token, _ := service.Login(ctx, "test@example.com", "password123")
		claims, _ := service.ValidateToken(token)

		files, _ := service.ListFiles(ctx, claims.UserID, "http://localhost:3001")
		if len(files) == 0 {
			t.Fatal("No files to download")
		}

		// Get updated file info with share token
		files, _ = service.ListFiles(ctx, claims.UserID, "http://localhost:3001")
		if files[0].ShareToken == nil {
			t.Fatal("File has no share token")
		}

		file, err := service.DownloadSharedFile(ctx, *files[0].ShareToken)
		if err != nil {
			t.Fatalf("Download shared file failed: %v", err)
		}

		if string(file.Data) != "Hello, World!" {
			t.Errorf("Expected file content 'Hello, World!', got '%s'", string(file.Data))
		}
	})

	// Test file deletion
	t.Run("DeleteFile", func(t *testing.T) {
		_, token, _ := service.Login(ctx, "test@example.com", "password123")
		claims, _ := service.ValidateToken(token)

		files, _ := service.ListFiles(ctx, claims.UserID, "http://localhost:3001")
		if len(files) == 0 {
			t.Fatal("No files to delete")
		}

		err := service.DeleteFile(ctx, files[0].ID, claims.UserID)
		if err != nil {
			t.Fatalf("Delete file failed: %v", err)
		}

		// Check file was deleted
		files, _ = service.ListFiles(ctx, claims.UserID, "http://localhost:3001")
		if len(files) != 0 {
			t.Errorf("Expected 0 files, got %d", len(files))
		}

		// Check storage was updated
		user, _ := service.GetUserProfile(ctx, claims.UserID)
		if user.StorageUsed != 0 {
			t.Errorf("Expected storage used 0, got %d", user.StorageUsed)
		}
	})

	// Clean up
	service.Close()
}

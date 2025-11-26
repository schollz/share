package relay

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/schollz/e2ecp/src/api"
	"github.com/schollz/e2ecp/src/auth"
)

// SetupAPIRoutes initializes all API routes for authentication and file management
func SetupAPIRoutes(mux *http.ServeMux, database *sql.DB, log *slog.Logger, enabled bool) {
	if !enabled {
		log.Info("Storage/profile endpoints disabled (set ALLOW_STORAGE_PROFILE=yes to enable)")
		return
	}

	// Get JWT secret from environment or use default
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-secret-change-in-production"
		log.Warn("Using default JWT secret. Set JWT_SECRET environment variable in production.")
	}

	// Initialize services and handlers
	authService := auth.NewService(database, jwtSecret, log)
	authHandlers := api.NewAuthHandlers(authService, log)
	fileHandlers := api.NewFileHandlers(database, log)
	authMiddleware := api.AuthMiddleware(authService)

	// Auth routes (no authentication required)
	mux.HandleFunc("/api/auth/register", authHandlers.Register)
	mux.HandleFunc("/api/auth/login", authHandlers.Login)
	mux.HandleFunc("/api/auth/verify-email", authHandlers.VerifyEmailToken)
	mux.HandleFunc("/api/auth/captcha", authHandlers.Captcha)

	// Auth verify route (authentication required)
	mux.Handle("/api/auth/verify", authMiddleware(http.HandlerFunc(authHandlers.Verify)))
	mux.Handle("/api/auth/change-password", authMiddleware(http.HandlerFunc(authHandlers.ChangePassword)))
	mux.Handle("/api/auth/delete-account", authMiddleware(http.HandlerFunc(authHandlers.DeleteAccount)))

	// File routes (authentication required)
	mux.Handle("/api/files/upload", authMiddleware(http.HandlerFunc(fileHandlers.Upload)))
	mux.Handle("/api/files/list", authMiddleware(http.HandlerFunc(fileHandlers.List)))

	// File download route (authentication required)
	mux.Handle("/api/files/download/", authMiddleware(http.HandlerFunc(fileHandlers.Download)))

	// File delete route (authentication required)
	mux.Handle("/api/files/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/files/") {
			fileHandlers.Delete(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))

	// Share link generation (authentication required)
	mux.Handle("/api/files/share/generate/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Rewrite path for handler
			r.URL.Path = strings.Replace(r.URL.Path, "/generate", "", 1)
			fileHandlers.GenerateShareLink(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// Public share link download (no authentication required)
	mux.HandleFunc("/api/files/share/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && !strings.Contains(r.URL.Path, "/generate/") {
			fileHandlers.DownloadByToken(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	log.Info("API routes initialized", "endpoints", []string{
		"/api/auth/register",
		"/api/auth/login",
		"/api/auth/verify-email",
		"/api/auth/captcha",
		"/api/auth/verify",
		"/api/files/upload",
		"/api/files/list",
		"/api/files/download/{id}",
		"/api/files/{id} (DELETE)",
		"/api/files/share/generate/{id}",
		"/api/files/share/{token}",
	})
}

func allowStorageProfile(log *slog.Logger) bool {
	val, ok := os.LookupEnv("ALLOW_STORAGE_PROFILE")
	if !ok {
		log.Info("ALLOW_STORAGE_PROFILE not set; storage/profile endpoints disabled by default")
		return false
	}

	if strings.EqualFold(val, "yes") {
		return true
	}

	log.Info("Storage/profile endpoints disabled (set ALLOW_STORAGE_PROFILE=yes to enable)", "value", val)
	return false
}

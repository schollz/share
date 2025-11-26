package relay

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SessionLog represents a relay session in the database
type SessionLog struct {
	SessionID      string
	IPFrom         string
	IPTo           string
	BandwidthBytes int64
	SessionStart   time.Time
	SessionEnd     *time.Time
}

// Database handles SQLite operations for relay session logging
type Database struct {
	db     *sql.DB
	logger *slog.Logger
	mutex  sync.Mutex
}

var (
	database     *Database
	databaseOnce sync.Once
)

// InitDatabase initializes the SQLite database for session logging
func InitDatabase(dbPath string, log *slog.Logger) error {
	var err error
	databaseOnce.Do(func() {
		database = &Database{
			logger: log,
		}

		database.db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			err = fmt.Errorf("failed to open database: %w", err)
			return
		}

		// Create the logs table if it doesn't exist
		createTableSQL := `
		CREATE TABLE IF NOT EXISTS logs (
			session_id TEXT PRIMARY KEY,
			ip_from TEXT NOT NULL,
			ip_to TEXT,
			bandwidth_bytes INTEGER DEFAULT 0,
			session_start DATETIME NOT NULL,
			session_end DATETIME
		);

		CREATE INDEX IF NOT EXISTS idx_session_start ON logs(session_start);
		CREATE INDEX IF NOT EXISTS idx_ip_from ON logs(ip_from);

		-- Users table for authentication
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		-- Files table for user uploaded files
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			filename TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			share_token TEXT UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);

		-- Indexes for better query performance
		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
		CREATE INDEX IF NOT EXISTS idx_files_share_token ON files(share_token);
		`

		_, err = database.db.Exec(createTableSQL)
		if err != nil {
			err = fmt.Errorf("failed to create tables: %w", err)
			return
		}

		log.Info("Database initialized", "path", dbPath)
	})

	return err
}

// GetDatabase returns the singleton database instance
func GetDatabase() *Database {
	return database
}

// StartSession creates a new session log entry
func (db *Database) StartSession(sessionID, ipFrom, ipTo string) error {
	if db == nil || db.db == nil {
		return fmt.Errorf("database not initialized")
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	query := `
		INSERT INTO logs (session_id, ip_from, ip_to, bandwidth_bytes, session_start)
		VALUES (?, ?, ?, 0, ?)
	`

	_, err := db.db.Exec(query, sessionID, ipFrom, ipTo, time.Now())
	if err != nil {
		db.logger.Error("Failed to start session", "error", err, "session_id", sessionID)
		return fmt.Errorf("failed to start session: %w", err)
	}

	db.logger.Debug("Session started", "session_id", sessionID, "ip_from", ipFrom, "ip_to", ipTo)
	return nil
}

// UpdateBandwidth updates the bandwidth for a session
func (db *Database) UpdateBandwidth(sessionID string, bytes int64) error {
	if db == nil || db.db == nil {
		return fmt.Errorf("database not initialized")
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	query := `
		UPDATE logs
		SET bandwidth_bytes = bandwidth_bytes + ?
		WHERE session_id = ?
	`

	_, err := db.db.Exec(query, bytes, sessionID)
	if err != nil {
		db.logger.Error("Failed to update bandwidth", "error", err, "session_id", sessionID)
		return fmt.Errorf("failed to update bandwidth: %w", err)
	}

	return nil
}

// EndSession marks a session as ended
func (db *Database) EndSession(sessionID string) error {
	if db == nil || db.db == nil {
		return fmt.Errorf("database not initialized")
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	query := `
		UPDATE logs
		SET session_end = ?
		WHERE session_id = ?
	`

	_, err := db.db.Exec(query, time.Now(), sessionID)
	if err != nil {
		db.logger.Error("Failed to end session", "error", err, "session_id", sessionID)
		return fmt.Errorf("failed to end session: %w", err)
	}

	db.logger.Debug("Session ended", "session_id", sessionID)
	return nil
}

// GetSessionStats retrieves session statistics
func (db *Database) GetSessionStats(sessionID string) (*SessionLog, error) {
	if db == nil || db.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	query := `
		SELECT session_id, ip_from, ip_to, bandwidth_bytes, session_start, session_end
		FROM logs
		WHERE session_id = ?
	`

	var log SessionLog
	var sessionEnd sql.NullTime

	err := db.db.QueryRow(query, sessionID).Scan(
		&log.SessionID,
		&log.IPFrom,
		&log.IPTo,
		&log.BandwidthBytes,
		&log.SessionStart,
		&sessionEnd,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get session stats: %w", err)
	}

	if sessionEnd.Valid {
		log.SessionEnd = &sessionEnd.Time
	}

	return &log, nil
}

// Close closes the database connection
func (db *Database) Close() error {
	if db == nil || db.db == nil {
		return nil
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	return db.db.Close()
}

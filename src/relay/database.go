package relay

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	migrationfs "github.com/schollz/e2ecp/migrations"
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

// Database handles PostgreSQL operations for relay session logging
type Database struct {
	db     *sql.DB
	logger *slog.Logger
	mutex  sync.Mutex
}

var (
	database     *Database
	databaseOnce sync.Once
)

// InitDatabase initializes the database for session logging
// Only PostgreSQL is supported.
func InitDatabase(databaseURL string, log *slog.Logger) error {
	var err error
	databaseOnce.Do(func() {
		if databaseURL == "" {
			err = fmt.Errorf("database URL is required")
			return
		}

		database = &Database{
			logger: log,
		}

		log.Info("Using PostgreSQL database", "dsn", maskPassword(databaseURL))

		database.db, err = sql.Open("postgres", databaseURL)
		if err != nil {
			err = fmt.Errorf("failed to open database: %w", err)
			return
		}

		// Test the connection
		if err = database.db.Ping(); err != nil {
			err = fmt.Errorf("failed to connect to database: %w", err)
			return
		}

		if err := runMigrations(database.db, log); err != nil {
			err = fmt.Errorf("failed to run migrations: %w", err)
			return
		}

		log.Info("Database initialized successfully", "driver", "postgres")
	})

	return err
}

// maskPassword masks the password in a database URL for logging
func maskPassword(dsn string) string {
	// Simple masking for postgres URLs
	// Format: postgresql://user:password@host:port/database
	if len(dsn) > 20 {
		return dsn[:20] + "***"
	}
	return "***"
}

func runMigrations(db *sql.DB, log *slog.Logger) error {
	// Prepare embedded migrations from the postgres subdirectory
	srcDriver, err := iofs.New(migrationfs.FS, "postgres")
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Prepare database driver based on the database type
	var dbDriver migratedb.Driver
	dbDriver, err = postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to init migrate postgres driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", srcDriver, "postgres", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// If schema_migrations table is missing but core tables exist (legacy),
	// mark baseline at version 1 so we can move forward with migrations.
	hasSchemaMigrations, _ := tableExists(db, "schema_migrations")
	if !hasSchemaMigrations {
		tablesPresent := existingSchemaPresent(db)
		if tablesPresent {
			log.Warn("Detected legacy database without migration history; baselining at version 1")
			if forceErr := m.Force(1); forceErr != nil {
				return fmt.Errorf("failed to baseline migrations: %w", forceErr)
			}
		}
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Info("Migrations completed successfully", "driver", "postgres")
	return nil
}

func tableExists(db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(1)
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = $1;
	`, name).Scan(&count)

	return count > 0, err
}

func existingSchemaPresent(db *sql.DB) bool {
	// Check for any of our core tables
	for _, tbl := range []string{"logs", "users", "files"} {
		exists, err := tableExists(db, tbl)
		if err == nil && exists {
			return true
		}
	}
	return false
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
		VALUES ($1, $2, $3, 0, $4)
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
		SET bandwidth_bytes = bandwidth_bytes + $1
		WHERE session_id = $2
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
		SET session_end = $1
		WHERE session_id = $2
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
		WHERE session_id = $1
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

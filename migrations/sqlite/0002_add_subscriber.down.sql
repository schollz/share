-- SQLite doesn't support dropping columns directly; recreate the table without subscriber
PRAGMA foreign_keys=off;

CREATE TABLE users_tmp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    encryption_salt TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users_tmp (id, email, password_hash, encryption_salt, created_at, updated_at)
SELECT id, email, password_hash, encryption_salt, created_at, updated_at FROM users;

DROP TABLE users;
ALTER TABLE users_tmp RENAME TO users;

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

PRAGMA foreign_keys=on;

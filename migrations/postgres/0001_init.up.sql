-- Initial schema for PostgreSQL
CREATE TABLE IF NOT EXISTS logs (
    session_id TEXT PRIMARY KEY,
    ip_from TEXT NOT NULL,
    ip_to TEXT,
    bandwidth_bytes BIGINT DEFAULT 0,
    session_start TIMESTAMP NOT NULL,
    session_end TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_session_start ON logs(session_start);
CREATE INDEX IF NOT EXISTS idx_ip_from ON logs(ip_from);

-- Users table for authentication
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    encryption_salt TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Files table for user uploaded files
CREATE TABLE IF NOT EXISTS files (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    encrypted_filename TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    encrypted_key TEXT NOT NULL,
    share_token TEXT UNIQUE,
    download_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_files_share_token ON files(share_token);

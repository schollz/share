-- Users table for authentication
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    storage_used INTEGER DEFAULT 0,           -- Bytes used
    storage_limit INTEGER DEFAULT 2147483648, -- 2GB = 2147483648 bytes
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for email lookups
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Files table for user uploaded files
CREATE TABLE IF NOT EXISTS files (
    id TEXT PRIMARY KEY,                      -- UUID
    user_id INTEGER NOT NULL,
    filename TEXT NOT NULL,
    size INTEGER NOT NULL,                    -- File size in bytes
    content_type TEXT DEFAULT 'application/octet-stream',
    data BLOB NOT NULL,                       -- Actual file content
    share_token TEXT UNIQUE,                  -- Token for shareable link (nullable)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Index for user file lookups
CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
-- Index for share token lookups
CREATE INDEX IF NOT EXISTS idx_files_share_token ON files(share_token);

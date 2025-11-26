-- SQLite doesn't support DROP COLUMN directly in older versions
-- Recreate table without file_path column
PRAGMA foreign_keys=off;

CREATE TABLE files_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    encrypted_filename TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    encrypted_key TEXT NOT NULL,
    share_token TEXT UNIQUE,
    download_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    file_data BLOB,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

INSERT INTO files_new (id, user_id, encrypted_filename, file_size, encrypted_key, share_token, download_count, created_at, updated_at, file_data)
SELECT id, user_id, encrypted_filename, file_size, encrypted_key, share_token, download_count, created_at, updated_at, file_data FROM files;

DROP TABLE files;
ALTER TABLE files_new RENAME TO files;

CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_files_share_token ON files(share_token);

PRAGMA foreign_keys=on;

-- SQLite doesn't support DROP COLUMN in all versions, so we recreate the table
PRAGMA foreign_keys=off;

CREATE TABLE files_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    encrypted_filename TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    encrypted_key TEXT NOT NULL,
    share_token TEXT UNIQUE,
    download_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

INSERT INTO files_new (id, user_id, encrypted_filename, file_path, file_size, encrypted_key, share_token, download_count, created_at, updated_at)
SELECT id, user_id, encrypted_filename, file_path, file_size, encrypted_key, share_token, download_count, created_at, updated_at FROM files;

DROP TABLE files;
ALTER TABLE files_new RENAME TO files;

CREATE INDEX IF NOT EXISTS idx_files_user_id ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_files_share_token ON files(share_token);

PRAGMA foreign_keys=on;

-- Device authentication sessions for CLI
CREATE TABLE IF NOT EXISTS device_auth_sessions (
    id BIGSERIAL PRIMARY KEY,
    device_code TEXT NOT NULL UNIQUE,
    user_code TEXT NOT NULL UNIQUE,
    user_id BIGINT,
    approved BOOLEAN NOT NULL DEFAULT FALSE,
    token TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_device_auth_device_code ON device_auth_sessions(device_code);
CREATE INDEX IF NOT EXISTS idx_device_auth_user_code ON device_auth_sessions(user_code);
CREATE INDEX IF NOT EXISTS idx_device_auth_expires_at ON device_auth_sessions(expires_at);

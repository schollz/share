-- Rollback device authentication
DROP INDEX IF EXISTS idx_device_auth_expires_at;
DROP INDEX IF EXISTS idx_device_auth_user_code;
DROP INDEX IF EXISTS idx_device_auth_device_code;
DROP TABLE IF EXISTS device_auth_sessions;

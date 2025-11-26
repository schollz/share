DROP INDEX IF EXISTS idx_users_verification_token;
ALTER TABLE users DROP COLUMN IF EXISTS verification_token;
ALTER TABLE users DROP COLUMN IF EXISTS verified;

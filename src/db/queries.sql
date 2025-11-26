-- name: CreateUser :one
INSERT INTO users (email, password_hash, encryption_salt, subscriber, verified, verification_token)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, email, password_hash, encryption_salt, created_at, updated_at, subscriber, verified, verification_token;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, encryption_salt, created_at, updated_at, subscriber, verified, verification_token FROM users
WHERE email = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT id, email, password_hash, encryption_salt, created_at, updated_at, subscriber, verified, verification_token FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByVerificationToken :one
SELECT id, email, password_hash, encryption_salt, created_at, updated_at, subscriber, verified, verification_token FROM users
WHERE verification_token = $1
LIMIT 1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2;

-- name: VerifyUserByToken :one
UPDATE users
SET verified = 1, verification_token = NULL, updated_at = CURRENT_TIMESTAMP
WHERE verification_token = $1
RETURNING *;

-- name: DeleteUserByID :exec
DELETE FROM users
WHERE id = $1;

-- name: CreateFile :one
INSERT INTO files (user_id, encrypted_filename, file_size, encrypted_key, share_token, download_count, file_data)
VALUES ($1, $2, $3, $4, $5, 0, $6)
RETURNING *;

-- name: GetFilesByUserID :many
SELECT * FROM files
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetFileByID :one
SELECT * FROM files
WHERE id = $1 AND user_id = $2
LIMIT 1;

-- name: GetFileByShareToken :one
SELECT * FROM files
WHERE share_token = $1
LIMIT 1;

-- name: IncrementDownloadCountByToken :one
UPDATE files
SET download_count = download_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE share_token = $1
RETURNING *;

-- name: GetTotalStorageByUserID :one
SELECT COALESCE(SUM(file_size), 0)::bigint as total_storage
FROM files
WHERE user_id = $1;

-- name: DeleteFile :exec
DELETE FROM files
WHERE id = $1 AND user_id = $2;

-- name: DeleteFileByID :exec
DELETE FROM files
WHERE id = $1;

-- name: UpdateFileShareToken :one
UPDATE files
SET share_token = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2 AND user_id = $3
RETURNING *;

-- name: UpdateFileEncryption :exec
UPDATE files
SET encrypted_filename = $1, encrypted_key = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $3 AND user_id = $4;

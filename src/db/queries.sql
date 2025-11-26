-- name: CreateUser :one
INSERT INTO users (email, password_hash, encryption_salt, subscriber, verified, verification_token)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, email, password_hash, encryption_salt, created_at, updated_at, subscriber, verified, verification_token;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, encryption_salt, created_at, updated_at, subscriber, verified, verification_token FROM users
WHERE email = ?
LIMIT 1;

-- name: GetUserByID :one
SELECT id, email, password_hash, encryption_salt, created_at, updated_at, subscriber, verified, verification_token FROM users
WHERE id = ?
LIMIT 1;

-- name: GetUserByVerificationToken :one
SELECT id, email, password_hash, encryption_salt, created_at, updated_at, subscriber, verified, verification_token FROM users
WHERE verification_token = ?
LIMIT 1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: VerifyUserByToken :one
UPDATE users
SET verified = 1, verification_token = NULL, updated_at = CURRENT_TIMESTAMP
WHERE verification_token = ?
RETURNING *;

-- name: DeleteUserByID :exec
DELETE FROM users
WHERE id = ?;

-- name: CreateFile :one
INSERT INTO files (user_id, encrypted_filename, file_path, file_size, encrypted_key, share_token, download_count)
VALUES (?, ?, ?, ?, ?, ?, 0)
RETURNING *;

-- name: GetFilesByUserID :many
SELECT * FROM files
WHERE user_id = ?
ORDER BY created_at DESC;

-- name: GetFileByID :one
SELECT * FROM files
WHERE id = ? AND user_id = ?
LIMIT 1;

-- name: GetFileByShareToken :one
SELECT * FROM files
WHERE share_token = ?
LIMIT 1;

-- name: IncrementDownloadCountByToken :one
UPDATE files
SET download_count = download_count + 1, updated_at = CURRENT_TIMESTAMP
WHERE share_token = ?
RETURNING *;

-- name: GetTotalStorageByUserID :one
SELECT COALESCE(SUM(file_size), 0) as total_storage
FROM files
WHERE user_id = ?;

-- name: DeleteFile :exec
DELETE FROM files
WHERE id = ? AND user_id = ?;

-- name: DeleteFileByID :exec
DELETE FROM files
WHERE id = ?;

-- name: UpdateFileShareToken :one
UPDATE files
SET share_token = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND user_id = ?
RETURNING *;

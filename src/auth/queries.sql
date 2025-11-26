-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES (?, ?)
RETURNING id, email, storage_used, storage_limit, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, storage_used, storage_limit, created_at, updated_at
FROM users
WHERE email = ?;

-- name: GetUserByID :one
SELECT id, email, password_hash, storage_used, storage_limit, created_at, updated_at
FROM users
WHERE id = ?;

-- name: UpdateUserStorageUsed :exec
UPDATE users
SET storage_used = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: CreateFile :one
INSERT INTO files (id, user_id, filename, size, content_type, data, share_token)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, user_id, filename, size, content_type, share_token, created_at;

-- name: GetFileByID :one
SELECT id, user_id, filename, size, content_type, data, share_token, created_at
FROM files
WHERE id = ?;

-- name: GetFileByShareToken :one
SELECT id, user_id, filename, size, content_type, data, share_token, created_at
FROM files
WHERE share_token = ?;

-- name: ListFilesByUserID :many
SELECT id, user_id, filename, size, content_type, share_token, created_at
FROM files
WHERE user_id = ?
ORDER BY created_at DESC;

-- name: UpdateFileShareToken :exec
UPDATE files
SET share_token = ?
WHERE id = ? AND user_id = ?;

-- name: DeleteFile :exec
DELETE FROM files
WHERE id = ? AND user_id = ?;

-- name: GetFileSizeByID :one
SELECT size
FROM files
WHERE id = ? AND user_id = ?;

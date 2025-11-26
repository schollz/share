-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES (?, ?)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = ?
LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = ?
LIMIT 1;

-- name: CreateFile :one
INSERT INTO files (user_id, filename, file_path, file_size, share_token)
VALUES (?, ?, ?, ?, ?)
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

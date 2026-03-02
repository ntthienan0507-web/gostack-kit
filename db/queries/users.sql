-- name: ListUsers :many
-- List users with optional search and role filter, paginated.
SELECT * FROM users
WHERE deleted_at IS NULL
  AND (
    CASE WHEN @search::text = '' THEN true
         ELSE full_name ILIKE '%' || @search || '%'
              OR email ILIKE '%' || @search || '%'
              OR username ILIKE '%' || @search || '%'
    END
  )
  AND (CASE WHEN @role::text = '' THEN true ELSE role = @role END)
ORDER BY created_at DESC
LIMIT @page_size::int OFFSET @page_offset::int;

-- name: CountUsers :one
-- Count users matching the same filters as ListUsers.
SELECT COUNT(*) FROM users
WHERE deleted_at IS NULL
  AND (
    CASE WHEN @search::text = '' THEN true
         ELSE full_name ILIKE '%' || @search || '%'
              OR email ILIKE '%' || @search || '%'
              OR username ILIKE '%' || @search || '%'
    END
  )
  AND (CASE WHEN @role::text = '' THEN true ELSE role = @role END);

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = @id AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = @email AND deleted_at IS NULL;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = @username AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO users (username, email, full_name, password_hash, role)
VALUES (@username, @email, @full_name, @password_hash, @role)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET
  full_name  = COALESCE(@full_name, full_name),
  role       = COALESCE(@role, role),
  is_active  = COALESCE(@is_active, is_active),
  updated_at = NOW()
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteUser :exec
UPDATE users
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = @id AND deleted_at IS NULL;

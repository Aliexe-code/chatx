-- name: CreateUser :one
INSERT INTO users (username, email, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CreateRoom :one
INSERT INTO rooms (name, private, password_hash, creator_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRoomByID :one
SELECT * FROM rooms
WHERE id = $1;

-- name: GetRoomByName :one
SELECT * FROM rooms
WHERE name = $1;

-- name: ListRooms :many
SELECT * FROM rooms
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListRoomsByCreator :many
SELECT * FROM rooms
WHERE creator_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateRoom :one
UPDATE rooms
SET name = $2, private = $3, password_hash = $4
WHERE id = $1
RETURNING *;

-- name: DeleteRoom :exec
DELETE FROM rooms
WHERE id = $1;

-- name: CreateMessage :one
INSERT INTO messages (room_id, user_id, content)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetMessageByID :one
SELECT * FROM messages
WHERE id = $1;

-- name: ListMessagesByRoom :many
SELECT m.*, u.username, r.name as room_name
FROM messages m
JOIN users u ON m.user_id = u.id
JOIN rooms r ON m.room_id = r.id
WHERE m.room_id = $1
ORDER BY m.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListRecentMessagesByRoom :many
SELECT m.*, u.username, r.name as room_name
FROM messages m
JOIN users u ON m.user_id = u.id
JOIN rooms r ON m.room_id = r.id
WHERE m.room_id = $1
ORDER BY m.created_at DESC
LIMIT $2;

-- name: AddRoomMember :one
INSERT INTO room_members (room_id, user_id)
VALUES ($1, $2)
ON CONFLICT (room_id, user_id) DO UPDATE SET joined_at = EXCLUDED.joined_at
RETURNING *;

-- name: RemoveRoomMember :exec
DELETE FROM room_members
WHERE room_id = $1 AND user_id = $2;

-- name: GetRoomMembers :many
SELECT u.*, rm.joined_at
FROM room_members rm
JOIN users u ON rm.user_id = u.id
WHERE rm.room_id = $1
ORDER BY rm.joined_at ASC;

-- name: IsRoomMember :one
SELECT EXISTS(
    SELECT 1 FROM room_members
    WHERE room_id = $1 AND user_id = $2
);

-- name: GetRoomMemberCount :one
SELECT COUNT(*) as count
FROM room_members
WHERE room_id = $1;

-- name: DeleteMessagesByRoom :exec
DELETE FROM messages
WHERE room_id = $1;

-- User profile management queries

-- name: UpdateUserUsername :one
UPDATE users
SET username = $2
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :one
UPDATE users
SET password_hash = $2
WHERE id = $1
RETURNING *;

-- name: UpdateUserLastLogin :one
UPDATE users
SET last_login = $2
WHERE id = $1
RETURNING *;
-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, user_id, name, url)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;
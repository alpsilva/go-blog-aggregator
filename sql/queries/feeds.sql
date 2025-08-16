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

-- name: GetFeedByUrl :one
SELECT *
FROM feeds
WHERE feeds.url = $1;

-- name: GetFeeds :many
SELECT *
FROM feeds;

-- name: MarkFeedFetched :exec
UPDATE feeds
SET
updated_at = NOW(),
last_fetched_at = NOW()
WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT *
FROM feeds
ORDER BY last_fetched_at ASC NULLS FIRST
LIMIT 1;

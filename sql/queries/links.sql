-- name: ListLinks :many
SELECT id, original_url, short_name, created_at
FROM links
ORDER BY id;

-- name: ListLinksByRange :many
SELECT id, original_url, short_name, created_at
FROM links
ORDER BY id
LIMIT $1 OFFSET $2;

-- name: CountLinks :one
SELECT COUNT(*)
FROM links;

-- name: CreateLink :one
INSERT INTO links (original_url, short_name)
VALUES ($1, $2)
RETURNING id, original_url, short_name, created_at;

-- name: GetLink :one
SELECT id, original_url, short_name, created_at
FROM links
WHERE id = $1;

-- name: GetLinkByShortName :one
SELECT id, original_url, short_name, created_at
FROM links
WHERE short_name = $1;

-- name: UpdateLink :one
UPDATE links
SET original_url = $2,
    short_name = COALESCE(NULLIF(sqlc.arg(short_name)::text, ''), short_name)
WHERE id = $1
RETURNING id, original_url, short_name, created_at;

-- name: DeleteLink :one
DELETE FROM links
WHERE id = $1
RETURNING id;

-- name: CreateAuditLog :one
INSERT INTO audit_logs (actor_user_id, action, entity, metadata, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAuditLogsByUser :many
SELECT * FROM audit_logs
WHERE actor_user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

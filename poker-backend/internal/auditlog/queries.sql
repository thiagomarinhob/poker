-- name: InsertAuditLog :one
INSERT INTO audit_log (actor_user_id, action, resource_type, resource_id, payload, ip_address)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING
  id, actor_user_id, action, resource_type, resource_id, payload, ip_address, created_at;

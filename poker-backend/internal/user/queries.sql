-- name: CreateUser :one
INSERT INTO users (email, password_hash, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1
LIMIT 1;

-- name: ListUsersForAdmin :many
SELECT id, email, role, chips_balance, created_at, updated_at
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: SearchUsersForAdmin :many
SELECT id, email, role, chips_balance, created_at, updated_at
FROM users
WHERE email ILIKE $1
ORDER BY email ASC
LIMIT $2 OFFSET $3;

-- name: ApplyChipsDelta :one
-- Sempre: uma linha em transactions + o novo chips_balance, na mesma transação
-- (delta negativo = saída de saldo, positivo = entrada)
WITH u AS (
  SELECT users.id, users.chips_balance
  FROM users
  WHERE users.id = $1
  FOR UPDATE
),
t AS (
  INSERT INTO transactions (user_id, amount, balance_after, type, hand_id, metadata, created_at)
  SELECT
    u.id,
    $2,
    u.chips_balance + $2,
    $3,
    sqlc.narg('hand_id')::uuid,
    COALESCE(sqlc.narg('metadata')::jsonb, '{}'::jsonb),
    now()
  FROM u
  WHERE u.chips_balance + $2 >= 0
  RETURNING transactions.id, transactions.balance_after
)
UPDATE users
SET
  chips_balance = t.balance_after,
  updated_at    = now()
FROM t
WHERE users.id = (SELECT u.id FROM u)
RETURNING
  users.id,
  users.email,
  users.password_hash,
  users.role,
  users.chips_balance,
  users.created_at,
  users.updated_at;

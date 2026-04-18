-- name: InsertHand :one
INSERT INTO hands (id, room_id, hand_number, dealer_seat, small_blind, big_blind, status, pot_total)
VALUES ($1, $2, $3, $4, $5, $6, 'in_progress', $7)
RETURNING
  id, room_id, hand_number, dealer_seat, small_blind, big_blind, status, pot_total, created_at, completed_at;

-- name: CompleteHand :one
UPDATE hands
SET
  status         = 'complete',
  pot_total      = $2,
  completed_at   = now()
WHERE id = $1
  AND status = 'in_progress'
RETURNING
  id, room_id, hand_number, dealer_seat, small_blind, big_blind, status, pot_total, created_at, completed_at;

-- name: InsertHandAction :one
INSERT INTO hand_actions (hand_id, action_seq, table_seat, hand_player_index, user_id, action_type, amount, street, is_timeout, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING
  id, hand_id, action_seq, table_seat, hand_player_index, user_id, action_type, amount, street, is_timeout, metadata, created_at;

-- name: ListHandsForAdmin :many
SELECT
  id, room_id, hand_number, dealer_seat, small_blind, big_blind, status, pot_total, created_at, completed_at
FROM hands
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetHandByID :one
SELECT
  id, room_id, hand_number, dealer_seat, small_blind, big_blind, status, pot_total, created_at, completed_at
FROM hands
WHERE id = $1
LIMIT 1;

-- name: ListHandActionsByHand :many
SELECT
  id, hand_id, action_seq, table_seat, hand_player_index, user_id, action_type, amount, street, is_timeout, metadata, created_at
FROM hand_actions
WHERE hand_id = $1
ORDER BY action_seq ASC;

-- name: CountHandsBetween :one
SELECT count(*)::bigint AS cnt
FROM hands
WHERE created_at >= $1 AND created_at < $2;

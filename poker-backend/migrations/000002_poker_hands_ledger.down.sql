DROP TRIGGER IF EXISTS transactions_forbid_mutation ON transactions;
DROP TRIGGER IF EXISTS hand_actions_forbid_mutation ON hand_actions;
DROP FUNCTION IF EXISTS forbid_mutation;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS hand_actions;
DROP TABLE IF EXISTS hands;

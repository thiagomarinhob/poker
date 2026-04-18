DROP TRIGGER IF EXISTS audit_log_forbid_mutation ON audit_log;
DROP FUNCTION IF EXISTS forbid_audit_log_mutation;
DROP TABLE IF EXISTS audit_log;

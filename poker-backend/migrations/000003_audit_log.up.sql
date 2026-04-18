CREATE TABLE audit_log (
    id              BIGSERIAL     PRIMARY KEY,
    actor_user_id   UUID          NOT NULL
                                   REFERENCES users (id) ON DELETE RESTRICT,
    action          TEXT          NOT NULL,
    resource_type   TEXT,
    resource_id     TEXT,
    payload         JSONB         NOT NULL DEFAULT '{}'::jsonb,
    ip_address      TEXT,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX audit_log_actor_created_idx ON audit_log (actor_user_id, created_at DESC);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC);

CREATE OR REPLACE FUNCTION forbid_audit_log_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_log é append-only';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_forbid_mutation
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW
    EXECUTE PROCEDURE forbid_audit_log_mutation();

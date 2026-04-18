-- Ledger de mesa: ações (append-only), extrato (append-only), mão com status.
-- transactions e hand_actions não devem sofrer UPDATE/DELETE; hands pode finalizar
-- o status (única mutação permitida além do insert inicial).

CREATE TABLE hands (
    id            UUID         PRIMARY KEY,
    room_id       TEXT         NOT NULL,
    hand_number   INT          NOT NULL,
    dealer_seat   INT          NOT NULL,
    small_blind   BIGINT       NOT NULL,
    big_blind     BIGINT       NOT NULL,
    status        TEXT         NOT NULL
                              CHECK (status IN ('in_progress', 'complete')),
    pot_total     BIGINT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ,
    CONSTRAINT hands_room_number_unique UNIQUE (room_id, hand_number)
);

CREATE INDEX hands_room_id_idx ON hands (room_id);

CREATE TABLE hand_actions (
    id                BIGSERIAL   PRIMARY KEY,
    hand_id           UUID        NOT NULL
                                  REFERENCES hands (id) ON DELETE CASCADE,
    action_seq        INT         NOT NULL,
    table_seat        INT,
    hand_player_index INT,
    user_id           UUID        REFERENCES users (id) ON DELETE SET NULL,
    action_type       TEXT        NOT NULL,
    amount            BIGINT,
    street            TEXT        NOT NULL,
    is_timeout        BOOLEAN     NOT NULL DEFAULT false,
    metadata          JSONB,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT hand_actions_seq_unique UNIQUE (hand_id, action_seq)
);

CREATE INDEX hand_actions_hand_id_idx ON hand_actions (hand_id);
CREATE INDEX hand_actions_user_id_idx ON hand_actions (user_id) WHERE user_id IS NOT NULL;

-- Extrato: append-only, sempre junto a users.chips_balance na mesma transação SQL
-- (ApplyChipsDelta em internal/user).
CREATE TABLE transactions (
    id             BIGSERIAL   PRIMARY KEY,
    user_id        UUID        NOT NULL
                               REFERENCES users (id) ON DELETE RESTRICT,
    amount         BIGINT      NOT NULL,
    balance_after  BIGINT      NOT NULL,
    type           TEXT        NOT NULL,
    hand_id        UUID        REFERENCES hands (id) ON DELETE SET NULL,
    metadata       JSONB,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX transactions_user_id_idx ON transactions (user_id, created_at DESC);
CREATE INDEX transactions_hand_id_idx ON transactions (hand_id) WHERE hand_id IS NOT NULL;

-- Impede reescrita/eliminação de linhas de audit na mesa
CREATE OR REPLACE FUNCTION forbid_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'tabela % é append-only', TG_TABLE_NAME;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER hand_actions_forbid_mutation
    BEFORE UPDATE OR DELETE ON hand_actions
    FOR EACH ROW
    EXECUTE PROCEDURE forbid_mutation();

CREATE TRIGGER transactions_forbid_mutation
    BEFORE UPDATE OR DELETE ON transactions
    FOR EACH ROW
    EXECUTE PROCEDURE forbid_mutation();

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(255)   NOT NULL,
    action      VARCHAR(50)    NOT NULL,     -- create, update, delete, view, export, login, logout
    resource    VARCHAR(100)   NOT NULL,     -- e.g. "user", "order"
    resource_id VARCHAR(255)   NOT NULL,
    changes     JSONB,                       -- diff: what changed
    metadata    JSONB,                       -- extra context
    ip          VARCHAR(45),
    user_agent  VARCHAR(500),
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_user_id    ON audit_log(user_id);
CREATE INDEX idx_audit_log_action     ON audit_log(action);
CREATE INDEX idx_audit_log_resource   ON audit_log(resource, resource_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_log;
-- +goose StatementEnd

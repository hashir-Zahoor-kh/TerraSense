CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id  UUID NOT NULL REFERENCES infra_requests(id),
    action      TEXT NOT NULL,
    actor       TEXT NOT NULL DEFAULT 'system',
    details     JSONB,
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_request_id ON audit_logs(request_id);

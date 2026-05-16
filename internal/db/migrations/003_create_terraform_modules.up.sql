CREATE TABLE terraform_modules (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL UNIQUE,
    description    TEXT,
    hcl_template   TEXT NOT NULL,
    resource_types JSONB,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TYPE request_status AS ENUM (
    'pending',
    'approved',
    'rejected',
    'applied',
    'failed'
);

CREATE TABLE infra_requests (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    natural_language_req  TEXT NOT NULL,
    generated_hcl         TEXT,
    terraform_plan_output TEXT,
    checkov_score         INT,
    checkov_warnings      JSONB,
    correction_attempts   INT NOT NULL DEFAULT 0,
    status                request_status NOT NULL DEFAULT 'pending',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

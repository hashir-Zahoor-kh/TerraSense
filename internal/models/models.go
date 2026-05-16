package models

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// RequestStatus mirrors the request_status Postgres enum.
// Using a typed string keeps the compiler aware of valid values.
type RequestStatus string

const (
	StatusPending  RequestStatus = "pending"
	StatusApproved RequestStatus = "approved"
	StatusRejected RequestStatus = "rejected"
	StatusApplied  RequestStatus = "applied"
	StatusFailed   RequestStatus = "failed"
)

// CheckovWarning is one failed check surfaced by Checkov.
type CheckovWarning struct {
	CheckID   string `json:"check_id"`
	CheckType string `json:"check_type"`
	Resource  string `json:"resource"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
}

// InfraRequest is the central model — one row per infrastructure provisioning request.
// It carries the full lifecycle: from the raw user intent through HCL generation,
// security scoring, and human approval.
type InfraRequest struct {
	ID                  pgtype.UUID      `db:"id"`
	NaturalLanguageReq  string           `db:"natural_language_req"`
	GeneratedHCL        *string          `db:"generated_hcl"`
	TerraformPlanOutput *string          `db:"terraform_plan_output"`
	CheckovScore        *int             `db:"checkov_score"`
	CheckovWarnings     []CheckovWarning `db:"checkov_warnings"`
	CorrectionAttempts  int              `db:"correction_attempts"`
	Status              RequestStatus    `db:"status"`
	CreatedAt           time.Time        `db:"created_at"`
	UpdatedAt           time.Time        `db:"updated_at"`
}

// AuditLog records every state transition and action on an InfraRequest.
// The audit trail is append-only — rows are never updated or deleted.
type AuditLog struct {
	ID        pgtype.UUID            `db:"id"`
	RequestID pgtype.UUID            `db:"request_id"`
	Action    string                 `db:"action"`
	Actor     string                 `db:"actor"`
	Details   map[string]interface{} `db:"details"`
	Timestamp time.Time              `db:"timestamp"`
}

// TerraformModule stores reference HCL templates the LLM can consult
// when generating infrastructure. Seeded by cmd/seed/main.go.
type TerraformModule struct {
	ID            pgtype.UUID `db:"id"`
	Name          string      `db:"name"`
	Description   *string     `db:"description"`
	HCLTemplate   string      `db:"hcl_template"`
	ResourceTypes []string    `db:"resource_types"`
	CreatedAt     time.Time   `db:"created_at"`
}

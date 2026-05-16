package correction

import (
	"context"

	"github.com/hashir-zahoor-kh/terrasense/internal/store"
	"github.com/hashir-zahoor-kh/terrasense/internal/tools"
	"github.com/hashir-zahoor-kh/terrasense/internal/validators"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrorContext accumulates the terraform and checkov errors from a failed
// generation attempt. Both fields are populated before the correction prompt
// is built so the LLM sees every problem in a single call rather than one
// error at a time — fewer round-trips, richer context.
type ErrorContext struct {
	TerraformErrors []string
	CheckovWarnings []validators.CheckovWarning
}

// LoopInput is the input to a single correction loop invocation.
// WorkspaceID isolates each request's terraform state and HCL files on disk
// so concurrent requests never collide.
type LoopInput struct {
	NaturalLanguageRequest string
	WorkspaceID            string
}

// LoopResult is the output of the correction loop.
// On success, RequestID is the UUID of the pending InfraRequest row.
// On failure, Errors surfaces everything the LLM could not fix within the
// retry budget so the caller can return actionable feedback to the user.
type LoopResult struct {
	Success            bool
	RequestID          pgtype.UUID
	CorrectionAttempts int
	Errors             *ErrorContext // non-nil only on failure
}

// CorrectionLoop orchestrates the full HCL generation and validation pipeline.
// It depends on three collaborators injected at construction time:
//   - HCLGenerator: calls the Anthropic API to produce initial and corrected HCL
//   - PlanExecutor: validates HCL syntax by running terraform plan (never apply)
//   - CheckovRunner: validates security posture via static analysis
//   - InfraRequestStore / AuditLogStore: persist state and the full audit trail
//
// Dependency injection (rather than global singletons) keeps the loop testable:
// tests substitute mocks for every collaborator without touching the real API,
// real terraform binary, or real database.
type CorrectionLoop struct {
	generator    *tools.HCLGenerator
	executor     *tools.PlanExecutor
	checkov      *validators.CheckovRunner
	infraStore   *store.InfraRequestStore
	auditStore   *store.AuditLogStore
	llm          tools.LLMClient // used for correction prompts (raw HCL response)
	maxRetries   int
	workspaceDir string
}

// NewCorrectionLoop constructs a CorrectionLoop with all required collaborators.
// maxRetries caps the number of LLM correction attempts before the loop gives
// up and marks the request as failed — prevents infinite loops on pathological
// inputs while still tolerating transient LLM quality issues.
func NewCorrectionLoop(
	generator *tools.HCLGenerator,
	executor *tools.PlanExecutor,
	checkov *validators.CheckovRunner,
	infraStore *store.InfraRequestStore,
	auditStore *store.AuditLogStore,
	llm tools.LLMClient,
	maxRetries int,
	workspaceDir string,
) *CorrectionLoop {
	return &CorrectionLoop{
		generator:    generator,
		executor:     executor,
		checkov:      checkov,
		infraStore:   infraStore,
		auditStore:   auditStore,
		llm:          llm,
		maxRetries:   maxRetries,
		workspaceDir: workspaceDir,
	}
}

// Run executes the correction loop for a single infrastructure request.
// It is the single entry point — callers (HTTP handlers) call this and receive
// a LoopResult without knowing anything about the internal retry logic.
//
// The algorithm:
//  1. Create a pending InfraRequest row in the DB so the request is durable
//     even if the process crashes mid-loop.
//  2. Call GenerateHCL to get the initial candidate HCL.
//  3. Call runValidation: terraform plan + checkov. If both pass, persist and return.
//  4. If either fails, increment CorrectionAttempts. If at the retry cap, mark
//     the request failed and return. Otherwise build a correction prompt and loop.
//  5. Every attempt — success or failure — is written to audit_logs so the
//     approval portal can show a complete correction history.
func (l *CorrectionLoop) Run(ctx context.Context, input LoopInput) (LoopResult, error) {
	panic("not implemented")
}

// runValidation runs terraform plan and checkov against hclContent and returns
// a combined ErrorContext. Separating validation into its own function keeps
// the main loop body readable and makes the validation step independently
// testable without invoking the full loop.
func (l *CorrectionLoop) runValidation(ctx context.Context, hclContent, workspaceID string) (tools.PlanResult, validators.CheckovResult, ErrorContext, bool) {
	panic("not implemented")
}

// buildCorrectionPrompt constructs the LLM prompt used for correction attempts.
// Terraform errors and checkov warnings are injected inline so the LLM
// understands exactly what it needs to fix. The prompt demands raw HCL back
// (no JSON wrapper, no markdown) because the correction response is used
// directly as the next candidate HCL — one fewer parse step, one fewer
// failure mode.
func buildCorrectionPrompt(originalHCL string, ec ErrorContext) string {
	panic("not implemented")
}

// logAuditEvent writes a single entry to audit_logs. Centralising the call
// means every loop step gets a consistent actor ("system") and details shape,
// and the caller never forgets to log — it is not optional.
func (l *CorrectionLoop) logAuditEvent(ctx context.Context, requestID pgtype.UUID, action string, details map[string]interface{}) error {
	return l.auditStore.Create(ctx, requestID, action, "system", details)
}

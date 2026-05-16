package correction

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/hashir-zahoor-kh/terrasense/internal/tools"
	"github.com/hashir-zahoor-kh/terrasense/internal/validators"
	"github.com/jackc/pgx/v5/pgtype"
)

// generatorI is the subset of tools.HCLGenerator used by CorrectionLoop.
// Expressed as an interface so tests can substitute a mock without touching
// the real Anthropic API.
type generatorI interface {
	GenerateHCL(ctx context.Context, input tools.GenerateHCLInput) (tools.HCLResult, error)
}

// executorI is the subset of tools.PlanExecutor used by CorrectionLoop.
type executorI interface {
	RunTerraformPlan(ctx context.Context, input tools.PlanInput) (tools.PlanResult, error)
}

// checkovI is the subset of validators.CheckovRunner used by CorrectionLoop.
type checkovI interface {
	RunCheckov(ctx context.Context, input validators.CheckovInput) (validators.CheckovResult, error)
}

// infraStoreI is the subset of store.InfraRequestStore used by CorrectionLoop.
type infraStoreI interface {
	Create(ctx context.Context, naturalLanguage string) (models.InfraRequest, error)
	UpdateStatus(ctx context.Context, id pgtype.UUID, status models.RequestStatus) error
	UpdateHCLAndScore(ctx context.Context, id pgtype.UUID, hcl string, checkovScore int, checkovWarnings []models.CheckovWarning, correctionAttempts int) error
}

// auditStoreI is the subset of store.AuditLogStore used by CorrectionLoop.
type auditStoreI interface {
	Create(ctx context.Context, requestID pgtype.UUID, action, actor string, details map[string]interface{}) error
}

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
	generator    generatorI
	executor     executorI
	checkov      checkovI
	infraStore   infraStoreI
	auditStore   auditStoreI
	llm          tools.LLMClient // used for correction prompts (raw HCL response)
	maxRetries   int
	workspaceDir string
}

// NewCorrectionLoop constructs a CorrectionLoop with all required collaborators.
// maxRetries caps the number of LLM correction attempts before the loop gives
// up and marks the request as failed — prevents infinite loops on pathological
// inputs while still tolerating transient LLM quality issues.
func NewCorrectionLoop(
	generator generatorI,
	executor executorI,
	checkov checkovI,
	infraStore infraStoreI,
	auditStore auditStoreI,
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
	// Create a durable DB record before any work begins so the request survives
	// a mid-loop process crash — the row is queryable by the portal immediately.
	infra, err := l.infraStore.Create(ctx, input.NaturalLanguageRequest)
	if err != nil {
		return LoopResult{}, fmt.Errorf("create infra request: %w", err)
	}

	// Use the request UUID as workspace ID so ApproveChange can reconstruct the
	// workspace path without storing workspace_id in the DB.
	b := infra.ID.Bytes
	workspaceID := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])

	// Generate the initial HCL candidate from the natural language request.
	// All subsequent iterations refine this candidate; the original is never
	// mutated so the correction prompt can always show it for context.
	hclResult, err := l.generator.GenerateHCL(ctx, tools.GenerateHCLInput{
		ResourceDescription: input.NaturalLanguageRequest,
	})
	if err != nil {
		return LoopResult{}, fmt.Errorf("generate HCL: %w", err)
	}
	// Audit every generation attempt so the portal can show a complete history.
	_ = l.logAuditEvent(ctx, infra.ID, "generation_attempt", map[string]interface{}{
		"workspace_id": workspaceID,
	})

	hcl := hclResult.HCL
	correctionAttempts := 0
	for {
		// runValidation runs terraform plan then checkov in sequence and returns
		// a combined ErrorContext so each loop iteration has the full error picture.
		_, checkovResult, ec, passed := l.runValidation(ctx, hcl, workspaceID)

		if passed {
			// Persist the final HCL and checkov score so the portal can display
			// them before the human approves or rejects the change.
			_ = l.infraStore.UpdateHCLAndScore(ctx, infra.ID, hcl, checkovResult.Score, nil, correctionAttempts)
			// Both terraform plan and checkov passed — the HCL is syntactically
			// valid and meets the security score threshold. The DB row was already
			// created as "pending" (awaiting human approval in the portal).
			_ = l.logAuditEvent(ctx, infra.ID, "generation_complete", map[string]interface{}{
				"correction_attempts": correctionAttempts,
			})
			return LoopResult{
				Success:            true,
				RequestID:          infra.ID,
				CorrectionAttempts: correctionAttempts,
			}, nil
		}

		correctionAttempts++

		if correctionAttempts >= l.maxRetries {
			// Persist whatever was last generated before marking failed so the
			// portal can show the HCL and score even on a failed request.
			_ = l.infraStore.UpdateHCLAndScore(ctx, infra.ID, hcl, checkovResult.Score, nil, correctionAttempts)
			// Retry budget exhausted: mark the DB row failed so the portal surfaces
			// a clear error rather than leaving the request stuck in "pending".
			_ = l.infraStore.UpdateStatus(ctx, infra.ID, "failed")
			_ = l.logAuditEvent(ctx, infra.ID, "correction_exhausted", map[string]interface{}{
				"correction_attempts": correctionAttempts,
				"terraform_errors":    ec.TerraformErrors,
				"checkov_warnings":    ec.CheckovWarnings,
			})
			return LoopResult{
				Success:            false,
				RequestID:          infra.ID,
				CorrectionAttempts: correctionAttempts,
				Errors:             &ec,
			}, nil
		}

		// Build the correction prompt with both the original HCL and all validation
		// errors in a single message — one LLM call per retry, not one per error.
		prompt := buildCorrectionPrompt(hcl, ec)
		slog.Debug("correction prompt", "prompt", prompt)
		_ = l.logAuditEvent(ctx, infra.ID, "correction_attempt", map[string]interface{}{
			"attempt": correctionAttempts,
		})

		// Ask the LLM for a corrected HCL candidate. The prompt demands raw HCL
		// with no markdown wrapper so the response can be used directly next iteration.
		hcl, err = l.llm.Complete(ctx, "You are a Terraform expert. Return only raw HCL with no markdown, no explanation, and no code fences.", prompt)
		if err != nil {
			return LoopResult{}, fmt.Errorf("llm correction attempt %d: %w", correctionAttempts, err)
		}
	}
}

// runValidation runs terraform plan and checkov against hclContent and returns
// a combined ErrorContext. Separating validation into its own function keeps
// the main loop body readable and makes the validation step independently
// testable without invoking the full loop.
func (l *CorrectionLoop) runValidation(ctx context.Context, hclContent, workspaceID string) (tools.PlanResult, validators.CheckovResult, ErrorContext, bool) {
	var ec ErrorContext
	passed := true

	planResult, err := l.executor.RunTerraformPlan(ctx, tools.PlanInput{
		HCLContent:  hclContent,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		ec.TerraformErrors = append(ec.TerraformErrors, err.Error())
		passed = false
	}

	checkovResult, err := l.checkov.RunCheckov(ctx, validators.CheckovInput{
		HCLContent:   hclContent,
		WorkspaceDir: filepath.Join(l.workspaceDir, workspaceID),
	})
	if err != nil {
		ec.TerraformErrors = append(ec.TerraformErrors, err.Error())
		passed = false
	} else if checkovResult.Score < 80 {
		ec.CheckovWarnings = append(ec.CheckovWarnings, checkovResult.Warnings...)
		passed = false
	}

	return planResult, checkovResult, ec, passed
}

// buildCorrectionPrompt constructs the LLM prompt used for correction attempts.
// Terraform errors and checkov warnings are injected inline so the LLM
// understands exactly what it needs to fix. The prompt demands raw HCL back
// (no JSON wrapper, no markdown) because the correction response is used
// directly as the next candidate HCL — one fewer parse step, one fewer
// failure mode.
func buildCorrectionPrompt(originalHCL string, ec ErrorContext) string {
	var b strings.Builder

	b.WriteString("The following HCL failed validation. Fix all errors and warnings.\n\n")
	b.WriteString("Original HCL:\n")
	b.WriteString(originalHCL)
	b.WriteString("\n\n")

	if len(ec.TerraformErrors) > 0 {
		b.WriteString("Terraform errors:\n")
		for _, e := range ec.TerraformErrors {
			b.WriteString("- ")
			b.WriteString(e)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(ec.CheckovWarnings) > 0 {
		b.WriteString("Checkov warnings:\n")
		for _, w := range ec.CheckovWarnings {
			b.WriteString("- [")
			b.WriteString(w.CheckID)
			b.WriteString("] ")
			b.WriteString(w.Resource)
			b.WriteString(": ")
			b.WriteString(w.Message)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("Return ONLY valid HCL. No markdown. No explanation.")
	return b.String()
}

// logAuditEvent writes a single entry to audit_logs. Centralising the call
// means every loop step gets a consistent actor ("system") and details shape,
// and the caller never forgets to log — it is not optional.
func (l *CorrectionLoop) logAuditEvent(ctx context.Context, requestID pgtype.UUID, action string, details map[string]interface{}) error {
	return l.auditStore.Create(ctx, requestID, action, "system", details)
}

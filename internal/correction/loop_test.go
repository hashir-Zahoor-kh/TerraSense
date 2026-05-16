package correction

import (
	"context"
	"errors"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/hashir-zahoor-kh/terrasense/internal/tools"
	"github.com/hashir-zahoor-kh/terrasense/internal/validators"
	"github.com/jackc/pgx/v5/pgtype"
)

var testID = pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}

// --- mock implementations ---

type mockGenerator struct {
	hcl string
}

func (m *mockGenerator) GenerateHCL(_ context.Context, _ tools.GenerateHCLInput) (tools.HCLResult, error) {
	return tools.HCLResult{HCL: m.hcl}, nil
}

// mockExecutor fails for the first failUntil calls, then passes.
type mockExecutor struct {
	callCount int
	failUntil int
}

func (m *mockExecutor) RunTerraformPlan(_ context.Context, _ tools.PlanInput) (tools.PlanResult, error) {
	m.callCount++
	if m.callCount <= m.failUntil {
		return tools.PlanResult{}, errors.New("terraform plan failed")
	}
	return tools.PlanResult{}, nil
}

// mockCheckov always passes with a score above the threshold.
type mockCheckov struct{}

func (m *mockCheckov) RunCheckov(_ context.Context, _ validators.CheckovInput) (validators.CheckovResult, error) {
	return validators.CheckovResult{Score: 100, Passed: true}, nil
}

// mockInfraStore records the last status written by UpdateStatus.
type mockInfraStore struct {
	lastStatus models.RequestStatus
}

func (m *mockInfraStore) Create(_ context.Context, _ string) (models.InfraRequest, error) {
	return models.InfraRequest{ID: testID}, nil
}

func (m *mockInfraStore) UpdateStatus(_ context.Context, _ pgtype.UUID, status models.RequestStatus) error {
	m.lastStatus = status
	return nil
}

func (m *mockInfraStore) UpdateHCLAndScore(_ context.Context, _ pgtype.UUID, _ string, _ int, _ []models.CheckovWarning, _ int) error {
	return nil
}

// mockAuditStore discards every event — we only care about counts in these tests.
type mockAuditStore struct{}

func (m *mockAuditStore) Create(_ context.Context, _ pgtype.UUID, _, _ string, _ map[string]interface{}) error {
	return nil
}

// mockLLM returns successive results from its results slice; falls back to "bad-hcl".
type mockLLM struct {
	callCount int
	results   []string
}

func (m *mockLLM) Complete(_ context.Context, _, _ string) (string, error) {
	if m.callCount < len(m.results) {
		r := m.results[m.callCount]
		m.callCount++
		return r, nil
	}
	return "bad-hcl", nil
}

// --- tests ---

// TestRun_SucceedsAfterOneCorrection verifies that when the first validation
// attempt fails but the LLM correction produces valid HCL, the loop returns
// Success=true with CorrectionAttempts=1 and never calls UpdateStatus.
func TestRun_SucceedsAfterOneCorrection(t *testing.T) {
	infraStore := &mockInfraStore{}
	// failUntil=1: first RunTerraformPlan call fails, second passes.
	executor := &mockExecutor{failUntil: 1}

	loop := NewCorrectionLoop(
		&mockGenerator{hcl: "bad-hcl"},
		executor,
		&mockCheckov{},
		infraStore,
		&mockAuditStore{},
		&mockLLM{results: []string{"good-hcl"}},
		3,
		"/tmp/workspace",
	)

	result, err := loop.Run(context.Background(), LoopInput{
		NaturalLanguageRequest: "create an S3 bucket",
		WorkspaceID:            "ws-test-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected Success=true")
	}
	if result.CorrectionAttempts != 1 {
		t.Errorf("expected CorrectionAttempts=1, got %d", result.CorrectionAttempts)
	}
	// On success the DB row stays pending — UpdateStatus must not have been called.
	if infraStore.lastStatus != "" {
		t.Errorf("expected no UpdateStatus call, got status=%q", infraStore.lastStatus)
	}
}

// TestRun_ExhaustsMaxRetries verifies that when every correction attempt still
// produces invalid HCL, the loop exits after maxRetries, marks the DB row as
// failed, and returns Success=false with CorrectionAttempts==maxRetries.
func TestRun_ExhaustsMaxRetries(t *testing.T) {
	const maxRetries = 3
	infraStore := &mockInfraStore{}
	// failUntil=100: RunTerraformPlan always fails within the retry window.
	executor := &mockExecutor{failUntil: 100}

	loop := NewCorrectionLoop(
		&mockGenerator{hcl: "bad-hcl"},
		executor,
		&mockCheckov{},
		infraStore,
		&mockAuditStore{},
		&mockLLM{}, // always returns "bad-hcl"
		maxRetries,
		"/tmp/workspace",
	)

	result, err := loop.Run(context.Background(), LoopInput{
		NaturalLanguageRequest: "create an S3 bucket",
		WorkspaceID:            "ws-test-2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Errorf("expected Success=false")
	}
	if result.CorrectionAttempts != maxRetries {
		t.Errorf("expected CorrectionAttempts=%d, got %d", maxRetries, result.CorrectionAttempts)
	}
	// On exhaustion the DB row must be marked failed.
	if infraStore.lastStatus != "failed" {
		t.Errorf("expected status=failed, got %q", infraStore.lastStatus)
	}
}

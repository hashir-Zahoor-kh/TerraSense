package tools_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/tools"
)

// mockRunner implements tools.CommandRunner without invoking any real binary.
type mockRunner struct {
	// calls records (name, args) pairs in order so tests can assert no apply ran.
	calls  []string
	stdout []byte
	stderr []byte
	err    error
}

func (m *mockRunner) Run(_ context.Context, _, name string, args ...string) ([]byte, []byte, error) {
	m.calls = append(m.calls, name+" "+joinArgs(args))
	return m.stdout, m.stderr, m.err
}

func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}

// validPlanNDJSON is a minimal terraform plan -json NDJSON fixture with
// 2 creates, 1 update, and 0 destroys spread across two message lines.
const validPlanNDJSON = `{"type":"version","terraform":"1.5.0","ui":"1.2"}
{"type":"changes","changes":[{"change":{"actions":["create"]}},{"change":{"actions":["create"]}}]}
{"type":"changes","changes":[{"change":{"actions":["update"]}}]}
{"type":"apply_complete","hook":{"resource":{}}}
`

func TestRunTerraformPlan_Success(t *testing.T) {
	runner := &mockRunner{stdout: []byte(validPlanNDJSON)}
	dir := t.TempDir()
	ex := tools.NewPlanExecutorWithRunner(dir, runner)

	result, err := ex.RunTerraformPlan(context.Background(), tools.PlanInput{
		HCLContent:  `resource "aws_s3_bucket" "b" { bucket = "test" }`,
		WorkspaceID: "ws-test-1",
	})
	if err != nil {
		t.Fatalf("RunTerraformPlan: %v", err)
	}

	if result.ToAdd != 2 {
		t.Errorf("ToAdd: got %d, want 2", result.ToAdd)
	}
	if result.ToChange != 1 {
		t.Errorf("ToChange: got %d, want 1", result.ToChange)
	}
	if result.ToDestroy != 0 {
		t.Errorf("ToDestroy: got %d, want 0", result.ToDestroy)
	}
	if result.PlanOutput == "" {
		t.Error("expected non-empty PlanOutput")
	}
}

func TestRunTerraformPlan_NeverCallsApply(t *testing.T) {
	runner := &mockRunner{stdout: []byte(validPlanNDJSON)}
	dir := t.TempDir()
	ex := tools.NewPlanExecutorWithRunner(dir, runner)

	_, _ = ex.RunTerraformPlan(context.Background(), tools.PlanInput{
		HCLContent:  `resource "aws_s3_bucket" "b" { bucket = "test" }`,
		WorkspaceID: "ws-test-apply-guard",
	})

	for _, call := range runner.calls {
		if len(call) >= 19 && call[10:19] == "terraform" {
			// check the args portion
		}
		// simpler: just scan the full call string
		for i := 0; i+5 <= len(call); i++ {
			if call[i:i+5] == "apply" {
				t.Errorf("RunTerraformPlan issued a terraform apply call: %q", call)
			}
		}
	}
}

func TestRunTerraformPlan_ErrTerraformNotInstalled(t *testing.T) {
	// Clear PATH so exec.LookPath("terraform") fails.
	t.Setenv("PATH", "")

	ex := tools.NewPlanExecutor(t.TempDir())
	_, err := ex.RunTerraformPlan(context.Background(), tools.PlanInput{
		HCLContent:  `resource "aws_s3_bucket" "b" {}`,
		WorkspaceID: "ws-no-binary",
	})

	if err == nil {
		t.Fatal("expected ErrTerraformNotInstalled, got nil")
	}
	var notInstalled tools.ErrTerraformNotInstalled
	if !errors.As(err, &notInstalled) {
		t.Errorf("expected ErrTerraformNotInstalled, got %T: %v", err, err)
	}
}

func TestRunTerraformPlan_InitFailure(t *testing.T) {
	runner := &mockRunner{err: errors.New("init failed")}
	dir := t.TempDir()
	ex := tools.NewPlanExecutorWithRunner(dir, runner)

	_, err := ex.RunTerraformPlan(context.Background(), tools.PlanInput{
		HCLContent:  `resource "aws_s3_bucket" "b" {}`,
		WorkspaceID: "ws-init-fail",
	})
	if err == nil {
		t.Error("expected error on init failure, got nil")
	}
}

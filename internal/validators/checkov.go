package validators

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ErrCheckovNotInstalled is returned when the checkov binary cannot be found.
// Callers check for this type to surface a human-readable message rather than
// a generic exec failure.
type ErrCheckovNotInstalled struct{}

func (e ErrCheckovNotInstalled) Error() string {
	return "checkov binary not found — install with: pip install checkov"
}

// CheckovWarning is a single failed check from a Checkov scan.
type CheckovWarning struct {
	CheckID   string
	CheckType string
	Resource  string
	Message   string
	Severity  string
}

// CheckovResult is the structured output of a Checkov run.
type CheckovResult struct {
	PassedChecks int
	FailedChecks int
	Score        int              // passed / total * 100; 0 if no checks ran
	Warnings     []CheckovWarning
	Passed       bool             // true if Score >= 80
}

// CheckovInput is the structured input to RunCheckov.
type CheckovInput struct {
	HCLContent   string
	WorkspaceDir string
}

// CheckovRunner runs Checkov static analysis against HCL content.
type CheckovRunner struct {
	runner CommandRunner
}

// CommandRunner abstracts os/exec so Checkov subprocess calls can be mocked in tests.
type CommandRunner interface {
	Run(ctx context.Context, dir, name string, args ...string) (stdout, stderr []byte, err error)
}

// execCommandRunner is the production CommandRunner backed by os/exec.
type execCommandRunner struct{}

func (r *execCommandRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	// checkov exits non-zero when checks fail — that is expected, not fatal.
	// cmd.Output() captures stdout; stderr is discarded since we parse JSON from stdout.
	stdout, err := cmd.Output()
	return stdout, nil, err
}

func NewCheckovRunner() *CheckovRunner {
	return &CheckovRunner{runner: &execCommandRunner{}}
}

// NewCheckovRunnerWithRunner allows tests to inject a mock CommandRunner.
func NewCheckovRunnerWithRunner(runner CommandRunner) *CheckovRunner {
	return &CheckovRunner{runner: runner}
}

// RunCheckov writes HCL to a temp file in WorkspaceDir, executes
// checkov -d <dir> --output json, and parses the result into CheckovResult.
// Returns ErrCheckovNotInstalled if the binary is missing.
func (c *CheckovRunner) RunCheckov(ctx context.Context, input CheckovInput) (CheckovResult, error) {
	if _, err := exec.LookPath("checkov"); err != nil {
		return CheckovResult{}, ErrCheckovNotInstalled{}
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := os.MkdirAll(input.WorkspaceDir, 0755); err != nil {
		return CheckovResult{}, fmt.Errorf("create workspace dir: %w", err)
	}

	hclPath := filepath.Join(input.WorkspaceDir, "main.tf")
	if err := os.WriteFile(hclPath, []byte(input.HCLContent), 0644); err != nil {
		return CheckovResult{}, fmt.Errorf("write HCL for checkov: %w", err)
	}

	// checkov exits 1 when checks fail — that is expected and not an error.
	// We parse stdout regardless of exit code; only a missing binary or
	// unparseable output is a hard failure.
	stdout, _, _ := c.runner.Run(ctx, input.WorkspaceDir, "checkov",
		"-d", input.WorkspaceDir,
		"--output", "json",
		"--quiet",
	)

	if len(stdout) == 0 {
		return CheckovResult{}, fmt.Errorf("checkov produced no output")
	}

	return parseCheckovOutput(stdout)
}

// parseCheckovOutput parses the JSON stdout from checkov --output json
// into a CheckovResult.
//
// Checkov emits a single JSON object with a "results" key containing
// "passed_checks" and "failed_checks" arrays, and a "summary" key with
// aggregate counts. We derive Score from summary counts so the result
// is consistent even when individual check arrays are truncated.
func parseCheckovOutput(data []byte) (CheckovResult, error) {
	// rawCheck mirrors a single entry in passed_checks or failed_checks.
	type rawCheck struct {
		CheckID   string `json:"check_id"`
		CheckType string `json:"check_type"`
		Resource  string `json:"resource"`
		Check     struct {
			Name     string `json:"name"`
			Severity string `json:"severity"`
		} `json:"check"`
	}

	// rawOutput is the top-level shape of checkov --output json.
	type rawOutput struct {
		Results struct {
			PassedChecks []rawCheck `json:"passed_checks"`
			FailedChecks []rawCheck `json:"failed_checks"`
		} `json:"results"`
		Summary struct {
			Passed int `json:"passed"`
			Failed int `json:"failed"`
		} `json:"summary"`
	}

	var raw rawOutput
	if err := json.Unmarshal(data, &raw); err != nil {
		return CheckovResult{}, fmt.Errorf("parse checkov output: %w", err)
	}

	passed := raw.Summary.Passed
	failed := raw.Summary.Failed
	total := passed + failed

	score := 0
	if total > 0 {
		score = passed * 100 / total
	}

	warnings := make([]CheckovWarning, 0, len(raw.Results.FailedChecks))
	for _, fc := range raw.Results.FailedChecks {
		severity := fc.Check.Severity
		if severity == "" {
			severity = "UNKNOWN"
		}
		warnings = append(warnings, CheckovWarning{
			CheckID:   fc.CheckID,
			CheckType: fc.CheckType,
			Resource:  fc.Resource,
			Message:   fc.Check.Name,
			Severity:  severity,
		})
	}

	return CheckovResult{
		PassedChecks: passed,
		FailedChecks: failed,
		Score:        score,
		Warnings:     warnings,
		Passed:       score >= 80,
	}, nil
}

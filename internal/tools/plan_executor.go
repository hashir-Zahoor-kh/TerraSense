package tools

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// PlanInput is the structured input to RunTerraformPlan.
type PlanInput struct {
	HCLContent  string
	WorkspaceID string
}

// PlanResult is the parsed output of `terraform plan -json`.
type PlanResult struct {
	PlanOutput  string
	ToAdd       int
	ToChange    int
	ToDestroy   int
	PlanJSON    map[string]interface{}
}

// ErrTerraformNotInstalled is returned when the terraform binary cannot be found.
// Callers check for this type to surface a human-readable message instead of panicking.
type ErrTerraformNotInstalled struct{}

func (e ErrTerraformNotInstalled) Error() string {
	return "terraform binary not found — install terraform and ensure it is on PATH"
}

// CommandRunner abstracts os/exec so terraform subprocess calls can be
// replaced with a mock in tests — no real terraform binary required.
type CommandRunner interface {
	Run(ctx context.Context, dir, name string, args ...string) (stdout, stderr []byte, err error)
}

// execCommandRunner is the production CommandRunner backed by os/exec.
type execCommandRunner struct{}

func (r *execCommandRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	return out.Bytes(), errOut.Bytes(), err
}

// PlanExecutor runs terraform init and terraform plan in a temporary workspace.
// It never runs terraform apply — that is enforced at the call site.
type PlanExecutor struct {
	workingDir string
	runner     CommandRunner
}

func NewPlanExecutor(workingDir string) *PlanExecutor {
	return &PlanExecutor{workingDir: workingDir, runner: &execCommandRunner{}}
}

// NewPlanExecutorWithRunner allows tests to inject a mock CommandRunner.
func NewPlanExecutorWithRunner(workingDir string, runner CommandRunner) *PlanExecutor {
	return &PlanExecutor{workingDir: workingDir, runner: runner}
}

// RunTerraformPlan writes HCL to a workspace directory, runs terraform init
// and terraform plan -json, and returns a parsed PlanResult.
// Returns ErrTerraformNotInstalled if the binary is missing.
// Never runs terraform apply — the apply path exists only in the HTTP handler
// behind a human approval gate.
func (e *PlanExecutor) RunTerraformPlan(ctx context.Context, input PlanInput) (PlanResult, error) {
	// Detect missing binary early so the caller gets a typed, actionable error
	// rather than a cryptic exec failure buried in subprocess output.
	if _, err := exec.LookPath("terraform"); err != nil {
		if errors.Is(err, exec.ErrNotFound) || os.IsNotExist(err) {
			return PlanResult{}, ErrTerraformNotInstalled{}
		}
		return PlanResult{}, ErrTerraformNotInstalled{}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	workspaceDir := filepath.Join(e.workingDir, input.WorkspaceID, newRunID())

	if err := writeHCL(workspaceDir, input.HCLContent); err != nil {
		return PlanResult{}, fmt.Errorf("write HCL: %w", err)
	}

	// terraform init downloads providers; -input=false prevents it from
	// blocking on interactive prompts in a non-TTY environment.
	_, initStderr, err := e.runner.Run(ctx, workspaceDir, "terraform", "init", "-input=false", "-no-color")
	if err != nil {
		return PlanResult{}, fmt.Errorf("terraform init: %w\nstderr: %s", err, initStderr)
	}

	// terraform plan -json emits NDJSON to stdout; parsePlanJSON handles it.
	planStdout, planStderr, err := e.runner.Run(ctx, workspaceDir, "terraform", "plan", "-json", "-input=false", "-no-color")
	if err != nil {
		return PlanResult{}, fmt.Errorf("terraform plan: %w\nstderr: %s", err, planStderr)
	}

	result, err := parsePlanJSON(planStdout)
	if err != nil {
		return PlanResult{}, fmt.Errorf("parse plan output: %w", err)
	}

	return result, nil
}

// newRunID returns a random UUID v4 string using crypto/rand.
// Each RunTerraformPlan call gets its own subdirectory so concurrent runs
// with the same workspace_id never collide.
func newRunID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// writeHCL creates the workspace directory and writes HCL content to main.tf.
// Using a named file (main.tf) rather than a temp file so terraform init and
// plan can be run repeatedly in the same workspace without re-initialization.
func writeHCL(workspaceDir, hclContent string) error {
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("create workspace directory %q: %w", workspaceDir, err)
	}

	mainTF := filepath.Join(workspaceDir, "main.tf")
	if err := os.WriteFile(mainTF, []byte(hclContent), 0644); err != nil {
		return fmt.Errorf("write main.tf: %w", err)
	}

	return nil
}

// parsePlanJSON reads the NDJSON stream from terraform plan -json and
// aggregates resource add/change/destroy counts.
//
// terraform plan -json emits one JSON object per line (NDJSON). Each line has
// a "type" field. The line with type "planned_changes" contains the resource
// change summary we care about. We accumulate the full output as PlanOutput so
// the raw stream is available for the approval portal diff viewer.
func parsePlanJSON(output []byte) (PlanResult, error) {
	// plannedChange is the shape of a single resource action inside the
	// "planned_changes" message emitted by terraform plan -json.
	type resourceChange struct {
		Change struct {
			Actions []string `json:"actions"`
		} `json:"change"`
	}
	type planLine struct {
		Type    string `json:"type"`
		Changes []resourceChange `json:"changes"`
		// "resource_drift" and "planned_changes" both use this shape
		Change *struct {
			Actions []string `json:"actions"`
		} `json:"change"`
	}

	result := PlanResult{
		PlanOutput: string(output),
		PlanJSON:   map[string]interface{}{},
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var pl planLine
		if err := json.Unmarshal(line, &pl); err != nil {
			// Non-JSON lines (terraform sometimes emits plain text warnings)
			// are tolerated — skip rather than abort.
			continue
		}

		// Store every parsed line in PlanJSON keyed by type for portal use.
		var raw map[string]interface{}
		if err := json.Unmarshal(line, &raw); err == nil {
			result.PlanJSON[pl.Type] = raw
		}

		// "changes" lines carry per-resource actions.
		for _, rc := range pl.Changes {
			for _, action := range rc.Change.Actions {
				switch action {
				case "create":
					result.ToAdd++
				case "update":
					result.ToChange++
				case "delete":
					result.ToDestroy++
				}
			}
		}

		// Single-resource change lines also appear with a top-level "change".
		if pl.Change != nil {
			for _, action := range pl.Change.Actions {
				switch action {
				case "create":
					result.ToAdd++
				case "update":
					result.ToChange++
				case "delete":
					result.ToDestroy++
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return PlanResult{}, fmt.Errorf("scan plan output: %w", err)
	}

	return result, nil
}

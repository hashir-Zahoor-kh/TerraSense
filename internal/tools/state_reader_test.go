package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/tools"
)

func TestReadCurrentState_NoStateFile(t *testing.T) {
	dir := t.TempDir()
	r := tools.NewStateReader(dir)

	result, err := r.ReadCurrentState(context.Background(), "workspace-1")
	if err != nil {
		t.Fatalf("expected no error for missing state file, got: %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected empty Resources, got %d", len(result.Resources))
	}
	if result.Message != "No existing state" {
		t.Errorf("unexpected message: %q", result.Message)
	}
}

func TestReadCurrentState_WithResources(t *testing.T) {
	dir := t.TempDir()
	workspaceID := "workspace-2"

	// Write a minimal terraform.tfstate fixture
	stateDir := filepath.Join(dir, workspaceID)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	stateJSON := `{
		"version": 4,
		"resources": [
			{
				"type": "aws_s3_bucket",
				"name": "logs",
				"instances": [{"attributes": {"id": "my-logs-bucket"}}]
			},
			{
				"type": "aws_iam_role",
				"name": "app_role",
				"instances": [{"attributes": {"id": "app-role-id"}}]
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(stateDir, "terraform.tfstate"), []byte(stateJSON), 0644); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	r := tools.NewStateReader(dir)
	result, err := r.ReadCurrentState(context.Background(), workspaceID)
	if err != nil {
		t.Fatalf("ReadCurrentState: %v", err)
	}

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	byName := map[string]tools.ResourceSummary{}
	for _, res := range result.Resources {
		byName[res.Name] = res
	}

	s3 := byName["logs"]
	if s3.Type != "aws_s3_bucket" {
		t.Errorf("S3 Type mismatch: got %q", s3.Type)
	}
	if s3.ID != "my-logs-bucket" {
		t.Errorf("S3 ID mismatch: got %q", s3.ID)
	}

	iam := byName["app_role"]
	if iam.Type != "aws_iam_role" {
		t.Errorf("IAM Type mismatch: got %q", iam.Type)
	}
	if iam.ID != "app-role-id" {
		t.Errorf("IAM ID mismatch: got %q", iam.ID)
	}
}

func TestReadCurrentState_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	workspaceID := "workspace-3"

	stateDir := filepath.Join(dir, workspaceID)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "terraform.tfstate"), []byte(`not json`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := tools.NewStateReader(dir)
	_, err := r.ReadCurrentState(context.Background(), workspaceID)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

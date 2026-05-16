package validators_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/validators"
)

// mockCheckovRunner implements commandRunner via the exported
// NewCheckovRunnerWithRunner constructor. It returns predetermined stdout
// without invoking any real checkov binary.
type mockCheckovRunner struct {
	stdout []byte
	err    error
}

func (m *mockCheckovRunner) Run(_ context.Context, _, _ string, _ ...string) (stdout, stderr []byte, err error) {
	return m.stdout, nil, m.err
}

// insecureCheckovJSON simulates a checkov run on a public S3 bucket with no
// encryption: 3 passed, 7 failed → score = 30 (< 80, Passed == false).
const insecureCheckovJSON = `{
  "results": {
    "passed_checks": [
      {"check_id":"CKV_AWS_1","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure versioning","severity":"LOW"}},
      {"check_id":"CKV_AWS_2","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure tagging","severity":"LOW"}},
      {"check_id":"CKV_AWS_3","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure name set","severity":"LOW"}}
    ],
    "failed_checks": [
      {"check_id":"CKV_AWS_18","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure access logs enabled","severity":"MEDIUM"}},
      {"check_id":"CKV_AWS_19","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure encryption enabled","severity":"HIGH"}},
      {"check_id":"CKV_AWS_20","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure bucket is private","severity":"HIGH"}},
      {"check_id":"CKV_AWS_21","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure MFA delete enabled","severity":"HIGH"}},
      {"check_id":"CKV_AWS_52","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure S3 policies not public","severity":"HIGH"}},
      {"check_id":"CKV_AWS_53","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure ACL not public-read","severity":"HIGH"}},
      {"check_id":"CKV_AWS_54","check_type":"terraform","resource":"aws_s3_bucket.bad","check":{"name":"Ensure no public ACL on bucket","severity":"HIGH"}}
    ]
  },
  "summary": {"passed": 3, "failed": 7}
}`

// secureCheckovJSON simulates a checkov run on a hardened S3 bucket:
// 9 passed, 1 failed → score = 90 (>= 80, Passed == true).
const secureCheckovJSON = `{
  "results": {
    "passed_checks": [
      {"check_id":"CKV_AWS_18","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure access logs enabled","severity":"MEDIUM"}},
      {"check_id":"CKV_AWS_19","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure encryption enabled","severity":"HIGH"}},
      {"check_id":"CKV_AWS_20","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure bucket is private","severity":"HIGH"}},
      {"check_id":"CKV_AWS_21","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure MFA delete enabled","severity":"HIGH"}},
      {"check_id":"CKV_AWS_52","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure S3 policies not public","severity":"HIGH"}},
      {"check_id":"CKV_AWS_53","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure ACL not public-read","severity":"HIGH"}},
      {"check_id":"CKV_AWS_54","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure no public ACL","severity":"HIGH"}},
      {"check_id":"CKV_AWS_1","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure versioning","severity":"LOW"}},
      {"check_id":"CKV_AWS_2","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure tagging","severity":"LOW"}}
    ],
    "failed_checks": [
      {"check_id":"CKV_AWS_144","check_type":"terraform","resource":"aws_s3_bucket.good","check":{"name":"Ensure cross-region replication","severity":"LOW"}}
    ]
  },
  "summary": {"passed": 9, "failed": 1}
}`

func TestRunCheckov_InsecureHCL(t *testing.T) {
	runner := validators.NewCheckovRunnerWithRunner(&mockCheckovRunner{stdout: []byte(insecureCheckovJSON)})

	result, err := runner.RunCheckov(context.Background(), validators.CheckovInput{
		HCLContent:   `resource "aws_s3_bucket" "bad" { bucket = "public-bucket" }`,
		WorkspaceDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("RunCheckov: %v", err)
	}

	if result.Score >= 80 {
		t.Errorf("expected Score < 80 for insecure HCL, got %d", result.Score)
	}
	if result.Passed {
		t.Error("expected Passed == false for insecure HCL")
	}
	if len(result.Warnings) == 0 {
		t.Error("expected non-empty Warnings for insecure HCL")
	}
	if result.FailedChecks == 0 {
		t.Error("expected FailedChecks > 0 for insecure HCL")
	}
}

func TestRunCheckov_SecureHCL(t *testing.T) {
	runner := validators.NewCheckovRunnerWithRunner(&mockCheckovRunner{stdout: []byte(secureCheckovJSON)})

	result, err := runner.RunCheckov(context.Background(), validators.CheckovInput{
		HCLContent:   `resource "aws_s3_bucket" "good" { bucket = var.bucket_name }`,
		WorkspaceDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("RunCheckov: %v", err)
	}

	if result.Score < 80 {
		t.Errorf("expected Score >= 80 for secure HCL, got %d", result.Score)
	}
	if !result.Passed {
		t.Error("expected Passed == true for secure HCL")
	}
	if result.PassedChecks == 0 {
		t.Error("expected PassedChecks > 0 for secure HCL")
	}
}

func TestRunCheckov_ErrCheckovNotInstalled(t *testing.T) {
	t.Setenv("PATH", "")

	runner := validators.NewCheckovRunner()
	_, err := runner.RunCheckov(context.Background(), validators.CheckovInput{
		HCLContent:   `resource "aws_s3_bucket" "b" {}`,
		WorkspaceDir: t.TempDir(),
	})

	if err == nil {
		t.Fatal("expected ErrCheckovNotInstalled, got nil")
	}
	var notInstalled validators.ErrCheckovNotInstalled
	if !errors.As(err, &notInstalled) {
		t.Errorf("expected ErrCheckovNotInstalled, got %T: %v", err, err)
	}
}

func TestRunCheckov_EmptyOutput(t *testing.T) {
	runner := validators.NewCheckovRunnerWithRunner(&mockCheckovRunner{stdout: []byte{}})

	_, err := runner.RunCheckov(context.Background(), validators.CheckovInput{
		HCLContent:   `resource "aws_s3_bucket" "b" {}`,
		WorkspaceDir: t.TempDir(),
	})
	if err == nil {
		t.Error("expected error for empty checkov output, got nil")
	}
}

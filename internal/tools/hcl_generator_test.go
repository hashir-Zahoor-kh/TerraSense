package tools_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/tools"
)

// mockLLM implements tools.LLMClient without touching the real Anthropic API.
type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

func TestGenerateHCL_ValidResponse(t *testing.T) {
	validJSON := `{
		"hcl": "resource \"aws_s3_bucket\" \"logs\" { bucket = var.bucket_name }",
		"resources_created": ["aws_s3_bucket.logs"],
		"explanation": "Creates a private S3 bucket for application logs."
	}`

	gen := tools.NewHCLGeneratorWithLLM(&mockLLM{response: validJSON})
	result, err := gen.GenerateHCL(context.Background(), tools.GenerateHCLInput{
		ResourceDescription: "private S3 bucket for application logs",
		ResourceType:        "aws_s3_bucket",
	})
	if err != nil {
		t.Fatalf("GenerateHCL: %v", err)
	}
	if result.HCL == "" {
		t.Error("expected non-empty HCL")
	}
	if len(result.ResourcesCreated) == 0 {
		t.Error("expected non-empty ResourcesCreated")
	}
	if result.Explanation == "" {
		t.Error("expected non-empty Explanation")
	}
}

func TestGenerateHCL_MalformedResponse(t *testing.T) {
	gen := tools.NewHCLGeneratorWithLLM(&mockLLM{response: "not json"})
	_, err := gen.GenerateHCL(context.Background(), tools.GenerateHCLInput{
		ResourceDescription: "some resource",
		ResourceType:        "aws_s3_bucket",
	})
	if err == nil {
		t.Error("expected error for malformed LLM response, got nil")
	}
}

func TestGenerateHCL_EmptyHCLField(t *testing.T) {
	// LLM returns valid JSON but hcl field is empty — must be rejected
	gen := tools.NewHCLGeneratorWithLLM(&mockLLM{response: `{"hcl":"","resources_created":[],"explanation":"oops"}`})
	_, err := gen.GenerateHCL(context.Background(), tools.GenerateHCLInput{
		ResourceDescription: "some resource",
		ResourceType:        "aws_s3_bucket",
	})
	if err == nil {
		t.Error("expected error for empty hcl field, got nil")
	}
}

func TestGenerateHCL_LLMError(t *testing.T) {
	gen := tools.NewHCLGeneratorWithLLM(&mockLLM{err: errors.New("api unavailable")})
	_, err := gen.GenerateHCL(context.Background(), tools.GenerateHCLInput{
		ResourceDescription: "some resource",
		ResourceType:        "aws_s3_bucket",
	})
	if err == nil {
		t.Error("expected error when LLM returns error, got nil")
	}
}

func TestGenerateHCL_WithExistingStateAndModules(t *testing.T) {
	validJSON := `{
		"hcl": "resource \"aws_instance\" \"app\" { ami = var.ami }",
		"resources_created": ["aws_instance.app"],
		"explanation": "Creates an EC2 instance."
	}`

	gen := tools.NewHCLGeneratorWithLLM(&mockLLM{response: validJSON})
	result, err := gen.GenerateHCL(context.Background(), tools.GenerateHCLInput{
		ResourceDescription: "EC2 instance in us-east-1",
		ResourceType:        "aws_instance",
		ExistingState: tools.StateResult{
			Resources: []tools.ResourceSummary{
				{Type: "aws_vpc", Name: "main", ID: "vpc-123"},
			},
		},
		RelevantModules: []tools.ModuleResult{
			{Name: "ec2-secure", Description: "hardened EC2", ResourceTypes: []string{"aws_instance"}},
		},
	})
	if err != nil {
		t.Fatalf("GenerateHCL with context: %v", err)
	}
	if result.HCL == "" {
		t.Error("expected non-empty HCL")
	}
}

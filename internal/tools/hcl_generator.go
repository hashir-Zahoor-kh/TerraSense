package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

// LLMClient abstracts the language model completion call.
// The real implementation wraps the Anthropic SDK; tests inject a mock.
// Keeping the interface narrow (one method) makes mocking trivial.
type LLMClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// anthropicLLMClient is the production LLMClient backed by the Anthropic API.
type anthropicLLMClient struct {
	client *anthropic.Client
}

func (a *anthropicLLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic API call: %w", err)
	}

	for _, block := range msg.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic response contained no text block")
}

// GenerateHCLInput is the structured input to GenerateHCL.
type GenerateHCLInput struct {
	ResourceDescription string
	ResourceType        string
	ExistingState       StateResult
	RelevantModules     []ModuleResult
}

// HCLResult is the structured output from GenerateHCL.
// JSON schema is enforced on the Anthropic response — malformed responses return an error.
type HCLResult struct {
	HCL              string   `json:"hcl"`
	ResourcesCreated []string `json:"resources_created"`
	Explanation      string   `json:"explanation"`
}

// HCLGenerator calls the LLM to produce valid, secure Terraform HCL.
type HCLGenerator struct {
	llm LLMClient
}

// NewHCLGenerator wraps the real Anthropic client behind the LLMClient interface.
func NewHCLGenerator(client *anthropic.Client) *HCLGenerator {
	return &HCLGenerator{llm: &anthropicLLMClient{client: client}}
}

// NewHCLGeneratorWithLLM allows tests to inject a mock LLMClient without
// touching the real Anthropic API.
func NewHCLGeneratorWithLLM(llm LLMClient) *HCLGenerator {
	return &HCLGenerator{llm: llm}
}

// GenerateHCL sends a structured prompt to the LLM and unmarshals the response
// into HCLResult. Returns an error if the response does not conform to the
// expected JSON schema — free-form text from the LLM is never trusted.
func (g *HCLGenerator) GenerateHCL(ctx context.Context, input GenerateHCLInput) (HCLResult, error) {
	system := buildSystemPrompt()
	user := buildUserPrompt(input)

	raw, err := g.llm.Complete(ctx, system, user)
	if err != nil {
		return HCLResult{}, fmt.Errorf("llm completion: %w", err)
	}

	var result HCLResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return HCLResult{}, fmt.Errorf("parse llm response as HCLResult: %w (raw: %.200s)", err, raw)
	}

	if result.HCL == "" {
		return HCLResult{}, fmt.Errorf("llm returned empty hcl field")
	}

	return result, nil
}

// buildSystemPrompt returns the system prompt with hard security constraints
// injected on every generation call. These are non-negotiable guardrails —
// the LLM must satisfy all of them regardless of the user's request.
func buildSystemPrompt() string {
	return `You are a Terraform infrastructure expert. Generate valid, production-ready HCL only.

HARD SECURITY CONSTRAINTS — never violate these:
- Always enable encryption at rest on every storage resource (S3, RDS, EBS, etc.)
- Never use 0.0.0.0/0 in security group ingress rules
- Always tag every resource with at minimum: Name and Environment
- Use variables for all values that may change between environments
- Never hardcode credentials, secrets, or account IDs

OUTPUT FORMAT — strict JSON, no markdown fences, no explanation outside the JSON:
{
  "hcl": "<complete valid HCL string>",
  "resources_created": ["<resource_type.resource_name>"],
  "explanation": "<one sentence describing what was created>"
}

Return ONLY that JSON object. Any deviation will be rejected.`
}

// buildUserPrompt formats the generation request from structured input into
// a prompt the LLM can act on. Existing state and relevant modules are injected
// so the LLM generates HCL that complements rather than duplicates what exists.
func buildUserPrompt(input GenerateHCLInput) string {
	prompt := fmt.Sprintf("Create Terraform HCL for the following request:\n%s\n\nResource type: %s\n",
		input.ResourceDescription, input.ResourceType)

	if len(input.ExistingState.Resources) > 0 {
		prompt += "\nExisting resources in this workspace (do not recreate these):\n"
		for _, r := range input.ExistingState.Resources {
			prompt += fmt.Sprintf("  - %s.%s (id: %s)\n", r.Type, r.Name, r.ID)
		}
	}

	if len(input.RelevantModules) > 0 {
		prompt += "\nReference modules you may draw from:\n"
		for _, m := range input.RelevantModules {
			prompt += fmt.Sprintf("  - %s: %s (covers: %v)\n", m.Name, m.Description, m.ResourceTypes)
		}
	}

	return prompt
}

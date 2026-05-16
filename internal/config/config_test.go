package config_test

import (
	"os"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/config"
)

func TestLoad_PanicsOnMissingAnthropicKey(t *testing.T) {
	// Clear all required vars so we can test each in isolation
	restore := clearEnv("DATABASE_URL", "ANTHROPIC_API_KEY", "AWS_REGION")
	defer restore()

	// Set every required var except ANTHROPIC_API_KEY
	os.Setenv("DATABASE_URL", "postgresql://x:x@localhost:5433/x")
	os.Setenv("AWS_REGION", "us-east-1")
	// ANTHROPIC_API_KEY deliberately left unset

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on missing ANTHROPIC_API_KEY, got none")
		}
	}()

	config.Load()
}

func TestLoad_PanicsOnMissingDatabaseURL(t *testing.T) {
	restore := clearEnv("DATABASE_URL", "ANTHROPIC_API_KEY", "AWS_REGION")
	defer restore()

	os.Setenv("ANTHROPIC_API_KEY", "sk-test")
	os.Setenv("AWS_REGION", "us-east-1")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on missing DATABASE_URL, got none")
		}
	}()

	config.Load()
}

func TestLoad_Defaults(t *testing.T) {
	restore := clearEnv(
		"DATABASE_URL", "ANTHROPIC_API_KEY", "AWS_REGION",
		"TERRAFORM_WORKING_DIR", "MAX_CORRECTION_RETRIES", "ENVIRONMENT",
	)
	defer restore()

	os.Setenv("DATABASE_URL", "postgresql://x:x@localhost:5433/x")
	os.Setenv("ANTHROPIC_API_KEY", "sk-test")
	os.Setenv("AWS_REGION", "us-east-1")

	cfg := config.Load()

	if cfg.TerraformWorkingDir != "/tmp/terrasense_workspaces" {
		t.Errorf("expected default TerraformWorkingDir, got %q", cfg.TerraformWorkingDir)
	}
	if cfg.MaxCorrectionRetries != 3 {
		t.Errorf("expected default MaxCorrectionRetries=3, got %d", cfg.MaxCorrectionRetries)
	}
	if cfg.Environment != "development" {
		t.Errorf("expected default Environment=development, got %q", cfg.Environment)
	}
}

func TestLoad_AllVarsSet(t *testing.T) {
	restore := clearEnv(
		"DATABASE_URL", "ANTHROPIC_API_KEY", "AWS_REGION",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"TERRAFORM_WORKING_DIR", "MAX_CORRECTION_RETRIES", "ENVIRONMENT",
	)
	defer restore()

	os.Setenv("DATABASE_URL", "postgresql://terrasense:terrasense@localhost:5433/terrasense")
	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("AWS_ACCESS_KEY_ID", "AKIA123")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	os.Setenv("TERRAFORM_WORKING_DIR", "/custom/workdir")
	os.Setenv("MAX_CORRECTION_RETRIES", "5")
	os.Setenv("ENVIRONMENT", "production")

	cfg := config.Load()

	if cfg.AnthropicAPIKey != "sk-ant-test" {
		t.Errorf("unexpected AnthropicAPIKey: %q", cfg.AnthropicAPIKey)
	}
	if cfg.MaxCorrectionRetries != 5 {
		t.Errorf("expected MaxCorrectionRetries=5, got %d", cfg.MaxCorrectionRetries)
	}
	if cfg.TerraformWorkingDir != "/custom/workdir" {
		t.Errorf("unexpected TerraformWorkingDir: %q", cfg.TerraformWorkingDir)
	}
}

// clearEnv unsets the given env vars and returns a function that restores them.
func clearEnv(keys ...string) func() {
	saved := make(map[string]string, len(keys))
	for _, k := range keys {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	return func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}
}

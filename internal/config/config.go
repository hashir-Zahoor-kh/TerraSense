package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration. Pass by value — no global state.
// Callers that need it receive it as a parameter; nothing imports this as a singleton.
type Config struct {
	DatabaseURL           string
	AnthropicAPIKey       string
	AWSAccessKeyID        string
	AWSSecretAccessKey    string
	AWSRegion             string
	TerraformWorkingDir   string
	MaxCorrectionRetries  int
	Environment           string
}

// Load reads config from environment variables.
// It attempts to load a .env file first (ignored if absent — production has real env vars).
// Panics on startup if any required variable is missing, so misconfiguration surfaces
// immediately rather than at the first database/API call.
func Load() Config {
	// Best-effort .env load; production environments won't have this file
	_ = godotenv.Load()

	cfg := Config{
		DatabaseURL:          requireEnv("DATABASE_URL"),
		AnthropicAPIKey:      requireEnv("ANTHROPIC_API_KEY"),
		AWSAccessKeyID:       os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretAccessKey:   os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSRegion:            requireEnv("AWS_REGION"),
		TerraformWorkingDir:  envWithDefault("TERRAFORM_WORKING_DIR", "/tmp/terrasense_workspaces"),
		MaxCorrectionRetries: envIntWithDefault("MAX_CORRECTION_RETRIES", 3),
		Environment:          envWithDefault("ENVIRONMENT", "development"),
	}

	return cfg
}

// requireEnv returns the value of the named env var or panics with a descriptive message.
// Fail-fast is intentional: a missing API key should crash the process on startup,
// not at the first request 10 minutes after deploy.
func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

func envWithDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envIntWithDefault(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		panic(fmt.Sprintf("environment variable %q must be an integer, got %q", key, v))
	}
	return n
}

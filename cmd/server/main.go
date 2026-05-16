package main

import (
	"log"
	"net/http"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/hashir-zahoor-kh/terrasense/internal/api"
	"github.com/hashir-zahoor-kh/terrasense/internal/config"
	"github.com/hashir-zahoor-kh/terrasense/internal/correction"
	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/store"
	"github.com/hashir-zahoor-kh/terrasense/internal/tools"
	"github.com/hashir-zahoor-kh/terrasense/internal/validators"
)

func main() {
	cfg := config.Load()

	pool, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}

	infraStore := store.NewInfraRequestStore(pool)
	auditStore := store.NewAuditLogStore(pool)

	anthropicClient := anthropic.NewClient(option.WithAPIKey(cfg.AnthropicAPIKey))

	hclGen := tools.NewHCLGenerator(&anthropicClient)
	planExec := tools.NewPlanExecutor(cfg.TerraformWorkingDir)
	checkov := validators.NewCheckovRunner()
	llm := tools.NewAnthropicLLMClient(&anthropicClient)

	loop := correction.NewCorrectionLoop(hclGen, planExec, checkov, infraStore, auditStore, llm, cfg.MaxCorrectionRetries, cfg.TerraformWorkingDir)

	handler := api.NewHandler(pool, infraStore, auditStore, loop, cfg.TerraformWorkingDir)

	router := api.NewRouter(handler, os.Getenv("API_KEY"))

	log.Println("TerraSense listening on :8000")
	if err := http.ListenAndServe(":8000", router); err != nil {
		log.Fatalf("server: %v", err)
	}
}

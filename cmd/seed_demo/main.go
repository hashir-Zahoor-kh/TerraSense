package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/demo"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	pool, err := db.Connect(dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	if err := demo.SeedDemoRequests(ctx, pool); err != nil {
		log.Fatalf("seed demo requests: %v", err)
	}

	fmt.Println("seeded 3 demo infra_requests")
	fmt.Println("  - secure S3 bucket (score 85, 0 corrections)")
	fmt.Println("  - EC2 API server  (score 72, 1 correction)")
	fmt.Println("  - RDS auth DB     (score 54, 2 corrections)")
}

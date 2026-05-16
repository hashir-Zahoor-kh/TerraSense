package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/hashir-zahoor-kh/terrasense/internal/store"
)

func testDB(t *testing.T) *store.InfraRequestStore {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable"
	}
	pool, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return store.NewInfraRequestStore(pool)
}

func TestCreate_InfraRequest(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	req := "Create a private S3 bucket with versioning and AES-256 encryption"
	got, err := s.Create(ctx, req)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	// Clean up the row after the test so the DB stays tidy
	t.Cleanup(func() {
		pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool != nil {
			pool.Exec(context.Background(), "DELETE FROM infra_requests WHERE id = $1", got.ID)
			pool.Close()
		}
	})

	if !got.ID.Valid {
		t.Error("expected a valid UUID, got zero value")
	}
	if got.NaturalLanguageReq != req {
		t.Errorf("NaturalLanguageReq mismatch: got %q, want %q", got.NaturalLanguageReq, req)
	}
	if got.Status != models.StatusPending {
		t.Errorf("expected status=pending, got %q", got.Status)
	}
	if got.CorrectionAttempts != 0 {
		t.Errorf("expected correction_attempts=0, got %d", got.CorrectionAttempts)
	}
	if got.GeneratedHCL != nil {
		t.Errorf("expected generated_hcl=nil on creation, got %q", *got.GeneratedHCL)
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
}

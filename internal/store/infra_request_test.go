package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/hashir-zahoor-kh/terrasense/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
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

func TestGetByID_InfraRequest(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	// Seed a row to fetch
	created, err := s.Create(ctx, "Create an EC2 instance in us-east-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool != nil {
			pool.Exec(context.Background(), "DELETE FROM infra_requests WHERE id = $1", created.ID)
			pool.Close()
		}
	})

	got, err := s.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("ID mismatch: got %v, want %v", got.ID, created.ID)
	}
	if got.NaturalLanguageReq != created.NaturalLanguageReq {
		t.Errorf("NaturalLanguageReq mismatch: got %q, want %q", got.NaturalLanguageReq, created.NaturalLanguageReq)
	}
	if got.Status != models.StatusPending {
		t.Errorf("expected status=pending, got %q", got.Status)
	}

	// Non-existent ID must return an error
	var zero pgtype.UUID
	_, err = s.GetByID(ctx, zero)
	if err == nil {
		t.Error("expected error for zero UUID, got nil")
	}
}

func TestListPending_InfraRequest(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	// Insert two pending rows
	r1, err := s.Create(ctx, "Pending request alpha")
	if err != nil {
		t.Fatalf("Create r1: %v", err)
	}
	r2, err := s.Create(ctx, "Pending request beta")
	if err != nil {
		t.Fatalf("Create r2: %v", err)
	}
	t.Cleanup(func() {
		pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool != nil {
			pool.Exec(context.Background(), "DELETE FROM infra_requests WHERE id = ANY($1)", []pgtype.UUID{r1.ID, r2.ID})
			pool.Close()
		}
	})

	list, err := s.ListPending(ctx)
	if err != nil {
		t.Fatalf("ListPending returned error: %v", err)
	}

	// Both inserted rows must appear in the result
	found := map[string]bool{}
	for _, r := range list {
		found[r.NaturalLanguageReq] = true
	}
	if !found["Pending request alpha"] {
		t.Error("expected 'Pending request alpha' in pending list")
	}
	if !found["Pending request beta"] {
		t.Error("expected 'Pending request beta' in pending list")
	}

	// Every returned row must have status=pending
	for _, r := range list {
		if r.Status != models.StatusPending {
			t.Errorf("expected status=pending, got %q for row %v", r.Status, r.ID)
		}
	}
}

func TestUpdateStatus_InfraRequest(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	created, err := s.Create(ctx, "Create an RDS instance")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool != nil {
			pool.Exec(context.Background(), "DELETE FROM infra_requests WHERE id = $1", created.ID)
			pool.Close()
		}
	})

	// Transition pending → approved
	if err := s.UpdateStatus(ctx, created.ID, models.StatusApproved); err != nil {
		t.Fatalf("UpdateStatus to approved: %v", err)
	}
	got, err := s.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got.Status != models.StatusApproved {
		t.Errorf("expected status=approved, got %q", got.Status)
	}

	// updated_at must have advanced
	if !got.UpdatedAt.After(created.UpdatedAt) {
		t.Error("expected updated_at to advance after status update")
	}

	// Unknown ID must return an error
	var zero pgtype.UUID
	if err := s.UpdateStatus(ctx, zero, models.StatusApproved); err == nil {
		t.Error("expected error for zero UUID, got nil")
	}
}

func TestUpdateHCLAndScore_InfraRequest(t *testing.T) {
	s := testDB(t)
	ctx := context.Background()

	created, err := s.Create(ctx, "Create a VPC with public and private subnets")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool != nil {
			pool.Exec(context.Background(), "DELETE FROM infra_requests WHERE id = $1", created.ID)
			pool.Close()
		}
	})

	hcl := `resource "aws_vpc" "main" { cidr_block = var.cidr }`
	warnings := []models.CheckovWarning{
		{CheckID: "CKV_AWS_1", CheckType: "terraform", Resource: "aws_vpc.main", Message: "ensure flow logs enabled", Severity: "MEDIUM"},
	}

	if err := s.UpdateHCLAndScore(ctx, created.ID, hcl, 85, warnings, 1); err != nil {
		t.Fatalf("UpdateHCLAndScore: %v", err)
	}

	got, err := s.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}

	if got.GeneratedHCL == nil || *got.GeneratedHCL != hcl {
		t.Errorf("GeneratedHCL mismatch: got %v, want %q", got.GeneratedHCL, hcl)
	}
	if got.CheckovScore == nil || *got.CheckovScore != 85 {
		t.Errorf("CheckovScore mismatch: got %v, want 85", got.CheckovScore)
	}
	if got.CorrectionAttempts != 1 {
		t.Errorf("CorrectionAttempts mismatch: got %d, want 1", got.CorrectionAttempts)
	}
	if len(got.CheckovWarnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(got.CheckovWarnings))
	}
	if got.CheckovWarnings[0].CheckID != "CKV_AWS_1" {
		t.Errorf("warning CheckID mismatch: got %q", got.CheckovWarnings[0].CheckID)
	}

	// Unknown ID must return an error
	var zero pgtype.UUID
	if err := s.UpdateHCLAndScore(ctx, zero, hcl, 85, nil, 0); err == nil {
		t.Error("expected error for zero UUID, got nil")
	}
}

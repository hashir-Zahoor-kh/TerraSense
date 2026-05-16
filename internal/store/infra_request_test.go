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

package store_test

import (
	"context"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/store"
)

func testAuditDB(t *testing.T) (*store.InfraRequestStore, *store.AuditLogStore) {
	t.Helper()
	pool, err := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return store.NewInfraRequestStore(pool), store.NewAuditLogStore(pool)
}

func TestCreate_AuditLog(t *testing.T) {
	infraStore, auditStore := testAuditDB(t)
	ctx := context.Background()

	// Audit logs require a valid infra_request FK
	req, err := infraStore.Create(ctx, "Create an S3 bucket for audit log test")
	if err != nil {
		t.Fatalf("Create infra request: %v", err)
	}
	t.Cleanup(func() {
		pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool != nil {
			// audit_logs rows are deleted by FK cascade when the parent is removed
			pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE request_id = $1", req.ID)
			pool.Exec(context.Background(), "DELETE FROM infra_requests WHERE id = $1", req.ID)
			pool.Close()
		}
	})

	details := map[string]interface{}{"attempt": 1, "errors_fixed": []string{"missing encryption"}}

	if err := auditStore.Create(ctx, req.ID, "generation_attempt", "system", details); err != nil {
		t.Fatalf("Create audit log: %v", err)
	}

	// Verify the row landed in the DB
	pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
	defer pool.Close()

	var action, actor string
	row := pool.QueryRow(ctx,
		"SELECT action, actor FROM audit_logs WHERE request_id = $1 ORDER BY timestamp DESC LIMIT 1",
		req.ID,
	)
	if err := row.Scan(&action, &actor); err != nil {
		t.Fatalf("scan audit log row: %v", err)
	}
	if action != "generation_attempt" {
		t.Errorf("action mismatch: got %q, want %q", action, "generation_attempt")
	}
	if actor != "system" {
		t.Errorf("actor mismatch: got %q, want %q", actor, "system")
	}

	// nil details must not error
	if err := auditStore.Create(ctx, req.ID, "generation_complete", "system", nil); err != nil {
		t.Fatalf("Create audit log with nil details: %v", err)
	}
}

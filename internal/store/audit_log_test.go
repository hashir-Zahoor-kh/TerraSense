package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
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

func TestList_AuditLog(t *testing.T) {
	infraStore, auditStore := testAuditDB(t)
	ctx := context.Background()

	req, err := infraStore.Create(ctx, "Create an EC2 instance for audit list test")
	if err != nil {
		t.Fatalf("Create infra request: %v", err)
	}
	t.Cleanup(func() {
		pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool != nil {
			pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE request_id = $1", req.ID)
			pool.Exec(context.Background(), "DELETE FROM infra_requests WHERE id = $1", req.ID)
			pool.Close()
		}
	})

	// Seed two audit log entries for this request
	if err := auditStore.Create(ctx, req.ID, "generation_attempt", "system", nil); err != nil {
		t.Fatalf("Create log 1: %v", err)
	}
	if err := auditStore.Create(ctx, req.ID, "generation_complete", "system", nil); err != nil {
		t.Fatalf("Create log 2: %v", err)
	}

	// Filter by request ID — must return exactly 2 rows
	logs, err := auditStore.List(ctx, store.AuditLogFilters{RequestID: &req.ID, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	for _, l := range logs {
		if l.RequestID != req.ID {
			t.Errorf("request_id mismatch: got %v, want %v", l.RequestID, req.ID)
		}
	}

	// StartDate in the future — must return 0 rows
	future := time.Now().Add(time.Hour)
	logs, err = auditStore.List(ctx, store.AuditLogFilters{RequestID: &req.ID, StartDate: &future, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("List with future StartDate: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs with future StartDate, got %d", len(logs))
	}

	// Zero/unknown request ID — must return 0 rows, no error
	var zero pgtype.UUID
	logs, err = auditStore.List(ctx, store.AuditLogFilters{RequestID: &zero, Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("List with zero UUID: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs for zero UUID, got %d", len(logs))
	}
}

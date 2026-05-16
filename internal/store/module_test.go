package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

func testModuleDB(t *testing.T) *store.ModuleStore {
	t.Helper()
	pool, err := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return store.NewModuleStore(pool)
}

func seedModule(t *testing.T, name, hcl string, resourceTypes []string) pgtype.UUID {
	t.Helper()
	pool, err := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
	if err != nil {
		t.Fatalf("connect for seed: %v", err)
	}
	defer pool.Close()

	rtJSON, err := json.Marshal(resourceTypes)
	if err != nil {
		t.Fatalf("marshal resource types: %v", err)
	}

	var id pgtype.UUID
	err = pool.QueryRow(context.Background(),
		`INSERT INTO terraform_modules (name, description, hcl_template, resource_types)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		name, "test module", hcl, rtJSON,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed module %q: %v", name, err)
	}
	return id
}

func cleanupModules(t *testing.T, ids ...pgtype.UUID) {
	t.Helper()
	pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
	if pool == nil {
		return
	}
	defer pool.Close()
	for _, id := range ids {
		pool.Exec(context.Background(), "DELETE FROM terraform_modules WHERE id = $1", id)
	}
}

func TestList_ModuleStore(t *testing.T) {
	s := testModuleDB(t)
	ctx := context.Background()

	id1 := seedModule(t, "test-s3-list", `resource "aws_s3_bucket" "b" {}`, []string{"aws_s3_bucket"})
	id2 := seedModule(t, "test-ec2-list", `resource "aws_instance" "i" {}`, []string{"aws_instance"})
	t.Cleanup(func() { cleanupModules(t, id1, id2) })

	// Unfiltered — both seeded modules must appear
	all, err := s.List(ctx, "")
	if err != nil {
		t.Fatalf("List unfiltered: %v", err)
	}
	found := map[string]bool{}
	for _, m := range all {
		found[m.Name] = true
	}
	if !found["test-s3-list"] {
		t.Error("expected test-s3-list in unfiltered list")
	}
	if !found["test-ec2-list"] {
		t.Error("expected test-ec2-list in unfiltered list")
	}

	// Filtered — only the S3 module must match
	filtered, err := s.List(ctx, "aws_s3_bucket")
	if err != nil {
		t.Fatalf("List filtered: %v", err)
	}
	for _, m := range filtered {
		if m.Name == "test-ec2-list" {
			t.Error("ec2 module must not appear when filtering for aws_s3_bucket")
		}
	}
	found2 := map[string]bool{}
	for _, m := range filtered {
		found2[m.Name] = true
	}
	if !found2["test-s3-list"] {
		t.Error("expected test-s3-list when filtering for aws_s3_bucket")
	}

	// Non-matching filter — must return empty slice, no error
	none, err := s.List(ctx, "aws_nonexistent_resource")
	if err != nil {
		t.Fatalf("List with no-match filter: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 results for non-matching filter, got %d", len(none))
	}
}

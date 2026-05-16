package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/models"
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

func TestGetByName_ModuleStore(t *testing.T) {
	s := testModuleDB(t)
	ctx := context.Background()

	id := seedModule(t, "test-rds-getbyname", `resource "aws_db_instance" "db" {}`, []string{"aws_db_instance"})
	t.Cleanup(func() { cleanupModules(t, id) })

	got, err := s.GetByName(ctx, "test-rds-getbyname")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}

	if got.Name != "test-rds-getbyname" {
		t.Errorf("Name mismatch: got %q, want %q", got.Name, "test-rds-getbyname")
	}
	if len(got.ResourceTypes) != 1 || got.ResourceTypes[0] != "aws_db_instance" {
		t.Errorf("ResourceTypes mismatch: got %v", got.ResourceTypes)
	}
	if got.HCLTemplate == "" {
		t.Error("expected non-empty HCLTemplate")
	}

	// Unknown name must return an error
	_, err = s.GetByName(ctx, "does-not-exist")
	if err == nil {
		t.Error("expected error for unknown module name, got nil")
	}
}

func TestBulkInsert_ModuleStore(t *testing.T) {
	s := testModuleDB(t)
	ctx := context.Background()

	desc := "bulk test module"
	modules := []models.TerraformModule{
		{Name: "test-bulk-s3", Description: &desc, HCLTemplate: `resource "aws_s3_bucket" "b" {}`, ResourceTypes: []string{"aws_s3_bucket"}},
		{Name: "test-bulk-ec2", Description: &desc, HCLTemplate: `resource "aws_instance" "i" {}`, ResourceTypes: []string{"aws_instance"}},
	}
	t.Cleanup(func() {
		pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool != nil {
			pool.Exec(context.Background(), "DELETE FROM terraform_modules WHERE name = ANY($1)", []string{"test-bulk-s3", "test-bulk-ec2"})
			pool.Close()
		}
	})

	if err := s.BulkInsert(ctx, modules); err != nil {
		t.Fatalf("BulkInsert: %v", err)
	}

	// Both rows must exist and be fetchable by name
	for _, want := range modules {
		got, err := s.GetByName(ctx, want.Name)
		if err != nil {
			t.Fatalf("GetByName(%q) after BulkInsert: %v", want.Name, err)
		}
		if got.HCLTemplate != want.HCLTemplate {
			t.Errorf("%q HCLTemplate mismatch: got %q", want.Name, got.HCLTemplate)
		}
		if len(got.ResourceTypes) != len(want.ResourceTypes) || got.ResourceTypes[0] != want.ResourceTypes[0] {
			t.Errorf("%q ResourceTypes mismatch: got %v", want.Name, got.ResourceTypes)
		}
	}

	// Re-running BulkInsert with the same names must upsert without error
	if err := s.BulkInsert(ctx, modules); err != nil {
		t.Fatalf("BulkInsert upsert: %v", err)
	}

	// Empty slice must be a no-op
	if err := s.BulkInsert(ctx, nil); err != nil {
		t.Fatalf("BulkInsert nil slice: %v", err)
	}
}

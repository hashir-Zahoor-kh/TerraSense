package tools_test

import (
	"context"
	"testing"

	"github.com/hashir-zahoor-kh/terrasense/internal/db"
	"github.com/hashir-zahoor-kh/terrasense/internal/store"
	"github.com/hashir-zahoor-kh/terrasense/internal/tools"
)

func newModuleLister(t *testing.T) (*tools.ModuleLister, func()) {
	t.Helper()
	pool, err := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	ms := store.NewModuleStore(pool)
	return tools.NewModuleLister(ms), pool.Close
}

func TestListExistingModules_Unfiltered(t *testing.T) {
	lister, cleanup := newModuleLister(t)
	defer cleanup()

	// Seed two modules directly via the store
	pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
	defer pool.Close()

	for _, name := range []string{"tool-test-s3", "tool-test-ec2"} {
		pool.Exec(context.Background(),
			`INSERT INTO terraform_modules (name, description, hcl_template, resource_types)
			 VALUES ($1, $2, $3, $4) ON CONFLICT (name) DO NOTHING`,
			name, "lister test", `resource "placeholder" "x" {}`, `["aws_placeholder"]`,
		)
	}
	t.Cleanup(func() {
		pool2, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool2 != nil {
			pool2.Exec(context.Background(), `DELETE FROM terraform_modules WHERE name = ANY($1)`, []string{"tool-test-s3", "tool-test-ec2"})
			pool2.Close()
		}
	})

	results, err := lister.ListExistingModules(context.Background(), "")
	if err != nil {
		t.Fatalf("ListExistingModules unfiltered: %v", err)
	}

	found := map[string]bool{}
	for _, r := range results {
		found[r.Name] = true
	}
	if !found["tool-test-s3"] {
		t.Error("expected tool-test-s3 in unfiltered results")
	}
	if !found["tool-test-ec2"] {
		t.Error("expected tool-test-ec2 in unfiltered results")
	}
}

func TestListExistingModules_Filtered(t *testing.T) {
	lister, cleanup := newModuleLister(t)
	defer cleanup()

	pool, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
	defer pool.Close()

	pool.Exec(context.Background(),
		`INSERT INTO terraform_modules (name, description, hcl_template, resource_types)
		 VALUES ($1, $2, $3, $4) ON CONFLICT (name) DO NOTHING`,
		"tool-test-rds", "lister filter test", `resource "aws_db_instance" "db" {}`, `["aws_db_instance"]`,
	)
	t.Cleanup(func() {
		pool2, _ := db.Connect("postgresql://terrasense:terrasense@localhost:5433/terrasense?sslmode=disable")
		if pool2 != nil {
			pool2.Exec(context.Background(), `DELETE FROM terraform_modules WHERE name = $1`, "tool-test-rds")
			pool2.Close()
		}
	})

	results, err := lister.ListExistingModules(context.Background(), "aws_db_instance")
	if err != nil {
		t.Fatalf("ListExistingModules filtered: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Name == "tool-test-rds" {
			found = true
			if r.Description == "" {
				t.Error("expected non-empty Description")
			}
			if len(r.ResourceTypes) == 0 {
				t.Error("expected non-empty ResourceTypes")
			}
		}
	}
	if !found {
		t.Error("expected tool-test-rds in filtered results")
	}

	// Non-matching filter must return empty slice, no error
	none, err := lister.ListExistingModules(context.Background(), "aws_nonexistent")
	if err != nil {
		t.Fatalf("ListExistingModules non-matching filter: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 results, got %d", len(none))
	}
}

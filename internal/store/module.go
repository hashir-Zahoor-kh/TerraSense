package store

import (
	"context"
	"fmt"

	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ModuleStore struct {
	db *pgxpool.Pool
}

func NewModuleStore(db *pgxpool.Pool) *ModuleStore {
	return &ModuleStore{db: db}
}

func (s *ModuleStore) List(ctx context.Context, resourceTypeFilter string) ([]models.TerraformModule, error) {
	q := `
		SELECT id, name, description, hcl_template, resource_types, created_at
		FROM terraform_modules`

	args := []interface{}{}

	// When a filter is provided, use the JSONB ? operator to check whether
	// the resource_types array contains the requested type string.
	if resourceTypeFilter != "" {
		q += " WHERE resource_types ? $1"
		args = append(args, resourceTypeFilter)
	}

	q += " ORDER BY name ASC"

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list terraform modules: %w", err)
	}
	defer rows.Close()

	var results []models.TerraformModule
	for rows.Next() {
		var m models.TerraformModule
		var resourceTypes []byte

		if err := rows.Scan(&m.ID, &m.Name, &m.Description, &m.HCLTemplate, &resourceTypes, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan terraform module row: %w", err)
		}
		if resourceTypes != nil {
			if err := jsonUnmarshal(resourceTypes, &m.ResourceTypes); err != nil {
				return nil, fmt.Errorf("unmarshal resource_types: %w", err)
			}
		}
		results = append(results, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate terraform module rows: %w", err)
	}

	return results, nil
}

func (s *ModuleStore) GetByName(ctx context.Context, name string) (models.TerraformModule, error) {
	const q = `
		SELECT id, name, description, hcl_template, resource_types, created_at
		FROM terraform_modules
		WHERE name = $1`

	var m models.TerraformModule
	var resourceTypes []byte

	row := s.db.QueryRow(ctx, q, name)
	if err := row.Scan(&m.ID, &m.Name, &m.Description, &m.HCLTemplate, &resourceTypes, &m.CreatedAt); err != nil {
		return models.TerraformModule{}, fmt.Errorf("get module by name %q: %w", name, err)
	}

	if resourceTypes != nil {
		if err := jsonUnmarshal(resourceTypes, &m.ResourceTypes); err != nil {
			return models.TerraformModule{}, fmt.Errorf("unmarshal resource_types: %w", err)
		}
	}

	return m, nil
}

func (s *ModuleStore) BulkInsert(ctx context.Context, modules []models.TerraformModule) error {
	if len(modules) == 0 {
		return nil
	}

	// Use a single transaction so either all modules land or none do.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin bulk insert transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const q = `
		INSERT INTO terraform_modules (name, description, hcl_template, resource_types)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE
		  SET description    = EXCLUDED.description,
		      hcl_template   = EXCLUDED.hcl_template,
		      resource_types = EXCLUDED.resource_types`

	for _, m := range modules {
		rtJSON, err := jsonMarshal(m.ResourceTypes)
		if err != nil {
			return fmt.Errorf("marshal resource_types for module %q: %w", m.Name, err)
		}
		if _, err := tx.Exec(ctx, q, m.Name, m.Description, m.HCLTemplate, rtJSON); err != nil {
			return fmt.Errorf("insert module %q: %w", m.Name, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit bulk insert: %w", err)
	}
	return nil
}

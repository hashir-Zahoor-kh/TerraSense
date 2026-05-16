package store

import (
	"context"

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
	panic("not implemented")
}

func (s *ModuleStore) GetByName(ctx context.Context, name string) (models.TerraformModule, error) {
	panic("not implemented")
}

func (s *ModuleStore) BulkInsert(ctx context.Context, modules []models.TerraformModule) error {
	panic("not implemented")
}

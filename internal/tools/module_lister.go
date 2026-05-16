package tools

import (
	"context"
	"fmt"

	"github.com/hashir-zahoor-kh/terrasense/internal/store"
)

// ModuleResult is a single terraform module returned to the caller.
type ModuleResult struct {
	Name          string
	Description   string
	ResourceTypes []string
}

// ModuleLister queries the terraform_modules table and returns matching modules.
type ModuleLister struct {
	store *store.ModuleStore
}

func NewModuleLister(store *store.ModuleStore) *ModuleLister {
	return &ModuleLister{store: store}
}

// ListExistingModules returns all modules, optionally filtered by resource type.
// An empty resourceTypeFilter returns all modules.
func (l *ModuleLister) ListExistingModules(ctx context.Context, resourceTypeFilter string) ([]ModuleResult, error) {
	modules, err := l.store.List(ctx, resourceTypeFilter)
	if err != nil {
		return nil, fmt.Errorf("list existing modules: %w", err)
	}

	results := make([]ModuleResult, 0, len(modules))
	for _, m := range modules {
		desc := ""
		if m.Description != nil {
			desc = *m.Description
		}
		results = append(results, ModuleResult{
			Name:          m.Name,
			Description:   desc,
			ResourceTypes: m.ResourceTypes,
		})
	}

	return results, nil
}

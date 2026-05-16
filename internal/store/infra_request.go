package store

import (
	"context"
	"fmt"

	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InfraRequestStore struct {
	db *pgxpool.Pool
}

func NewInfraRequestStore(db *pgxpool.Pool) *InfraRequestStore {
	return &InfraRequestStore{db: db}
}

func (s *InfraRequestStore) Create(ctx context.Context, naturalLanguageReq string) (models.InfraRequest, error) {
	const q = `
		INSERT INTO infra_requests (natural_language_req)
		VALUES ($1)
		RETURNING id, natural_language_req, generated_hcl, terraform_plan_output,
		          checkov_score, checkov_warnings, correction_attempts,
		          status, created_at, updated_at`

	var r models.InfraRequest
	var warnings []byte

	row := s.db.QueryRow(ctx, q, naturalLanguageReq)
	err := row.Scan(
		&r.ID,
		&r.NaturalLanguageReq,
		&r.GeneratedHCL,
		&r.TerraformPlanOutput,
		&r.CheckovScore,
		&warnings,
		&r.CorrectionAttempts,
		&r.Status,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		return models.InfraRequest{}, fmt.Errorf("create infra request: %w", err)
	}

	if warnings != nil {
		if err := unmarshalWarnings(warnings, &r.CheckovWarnings); err != nil {
			return models.InfraRequest{}, fmt.Errorf("unmarshal checkov_warnings: %w", err)
		}
	}

	return r, nil
}

func (s *InfraRequestStore) GetByID(ctx context.Context, id pgtype.UUID) (models.InfraRequest, error) {
	panic("not implemented")
}

func (s *InfraRequestStore) ListPending(ctx context.Context) ([]models.InfraRequest, error) {
	panic("not implemented")
}

func (s *InfraRequestStore) UpdateStatus(ctx context.Context, id pgtype.UUID, status models.RequestStatus) error {
	panic("not implemented")
}

func (s *InfraRequestStore) UpdateHCLAndScore(ctx context.Context, id pgtype.UUID, hcl string, score int, warnings []models.CheckovWarning, attempts int) error {
	panic("not implemented")
}

// unmarshalWarnings decodes the JSONB checkov_warnings column into a Go slice.
// Kept separate so each scan site stays readable.
func unmarshalWarnings(data []byte, dst *[]models.CheckovWarning) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return jsonUnmarshal(data, dst)
}

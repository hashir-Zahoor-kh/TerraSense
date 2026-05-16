package store

import (
	"context"
	"fmt"
	"time"

	"github.com/hashir-zahoor-kh/terrasense/internal/models"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditLogStore struct {
	db *pgxpool.Pool
}

func NewAuditLogStore(db *pgxpool.Pool) *AuditLogStore {
	return &AuditLogStore{db: db}
}

type AuditLogFilters struct {
	RequestID *pgtype.UUID
	Status    *models.RequestStatus
	StartDate *time.Time
	EndDate   *time.Time
	Page      int
	Limit     int
}

func (s *AuditLogStore) Create(ctx context.Context, requestID pgtype.UUID, action, actor string, details map[string]interface{}) error {
	detailsJSON, err := jsonMarshal(details)
	if err != nil {
		return fmt.Errorf("marshal audit log details: %w", err)
	}

	const q = `
		INSERT INTO audit_logs (request_id, action, actor, details)
		VALUES ($1, $2, $3, $4)`

	if _, err := s.db.Exec(ctx, q, requestID, action, actor, detailsJSON); err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func (s *AuditLogStore) List(ctx context.Context, filters AuditLogFilters) ([]models.AuditLog, error) {
	panic("not implemented")
}

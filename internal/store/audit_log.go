package store

import (
	"context"
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
	panic("not implemented")
}

func (s *AuditLogStore) List(ctx context.Context, filters AuditLogFilters) ([]models.AuditLog, error) {
	panic("not implemented")
}

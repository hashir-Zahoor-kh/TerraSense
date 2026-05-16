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
	// Build the query dynamically based on which filters are set.
	// $1 and $2 are always LIMIT and OFFSET; optional filters append to args.
	q := `
		SELECT id, request_id, action, actor, details, timestamp
		FROM audit_logs
		WHERE 1=1`

	args := []interface{}{}
	argN := 1

	if filters.RequestID != nil {
		q += fmt.Sprintf(" AND request_id = $%d", argN)
		args = append(args, *filters.RequestID)
		argN++
	}
	if filters.StartDate != nil {
		q += fmt.Sprintf(" AND timestamp >= $%d", argN)
		args = append(args, *filters.StartDate)
		argN++
	}
	if filters.EndDate != nil {
		q += fmt.Sprintf(" AND timestamp <= $%d", argN)
		args = append(args, *filters.EndDate)
		argN++
	}

	q += " ORDER BY timestamp DESC"

	limit := filters.Limit
	if limit <= 0 {
		limit = 20
	}
	page := filters.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	q += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argN, argN+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var results []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		var details []byte

		if err := rows.Scan(&l.ID, &l.RequestID, &l.Action, &l.Actor, &details, &l.Timestamp); err != nil {
			return nil, fmt.Errorf("scan audit log row: %w", err)
		}
		if details != nil {
			if err := jsonUnmarshal(details, &l.Details); err != nil {
				return nil, fmt.Errorf("unmarshal audit log details: %w", err)
			}
		}
		results = append(results, l)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit log rows: %w", err)
	}

	return results, nil
}

package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type Entry struct {
	EntityType  string
	EntityID    string
	Action      string
	PerformedBy *uuid.UUID
	OldValue    any
	NewValue    any
	IPAddress   string
	Notes       string
}

// Record writes a single audit trail entry. Every mutating action across
// every stage's service layer must go through this so nothing is skipped.
func (s *Service) Record(ctx context.Context, e Entry) error {
	oldJSON, err := marshalNullable(e.OldValue)
	if err != nil {
		return err
	}
	newJSON, err := marshalNullable(e.NewValue)
	if err != nil {
		return err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_trails
			(entity_type, entity_id, action, performed_by, old_value, new_value, ip_address, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.EntityType, e.EntityID, e.Action, e.PerformedBy, oldJSON, newJSON, e.IPAddress, e.Notes,
	)
	return err
}

func marshalNullable(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

type LogEntry struct {
	ID              int64      `json:"id"`
	EntityType      string     `json:"entity_type"`
	EntityID        string     `json:"entity_id"`
	Action          string     `json:"action"`
	PerformedBy     *uuid.UUID `json:"performed_by,omitempty"`
	PerformedByName *string    `json:"performed_by_name,omitempty"`
	PerformedAt     time.Time  `json:"performed_at"`
	OldValue        any        `json:"old_value,omitempty"`
	NewValue        any        `json:"new_value,omitempty"`
	IPAddress       string     `json:"ip_address,omitempty"`
	Notes           string     `json:"notes,omitempty"`
}

type ListFilter struct {
	EntityType string
	EntityID   string
	Limit      int
	Offset     int
}

// List returns audit trail entries, most recent first, optionally filtered
// by entity type and/or entity ID. Limit is clamped to a sane maximum.
func (s *Service) List(ctx context.Context, f ListFilter) ([]LogEntry, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query := `
		SELECT a.id, a.entity_type, a.entity_id, a.action, a.performed_by, u.full_name,
		       a.performed_at, a.old_value, a.new_value, a.ip_address, a.notes
		FROM audit_trails a
		LEFT JOIN users u ON u.id = a.performed_by
		WHERE ($1 = '' OR a.entity_type = $1) AND ($2 = '' OR a.entity_id = $2)
		ORDER BY a.performed_at DESC
		LIMIT $3 OFFSET $4`

	rows, err := s.pool.Query(ctx, query, f.EntityType, f.EntityID, limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []LogEntry{}
	for rows.Next() {
		var e LogEntry
		var oldRaw, newRaw []byte
		if err := rows.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.Action, &e.PerformedBy, &e.PerformedByName,
			&e.PerformedAt, &oldRaw, &newRaw, &e.IPAddress, &e.Notes); err != nil {
			return nil, err
		}
		if oldRaw != nil {
			if err := json.Unmarshal(oldRaw, &e.OldValue); err != nil {
				return nil, err
			}
		}
		if newRaw != nil {
			if err := json.Unmarshal(newRaw, &e.NewValue); err != nil {
				return nil, err
			}
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

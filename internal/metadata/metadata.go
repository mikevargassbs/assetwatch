package metadata

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FieldDefinition struct {
	ID         int    `json:"id"`
	Stage      string `json:"stage"`
	FieldKey   string `json:"field_key"`
	Label      string `json:"label"`
	DataType   string `json:"data_type"`
	IsRequired bool   `json:"is_required"`
	SortOrder  int    `json:"sort_order"`
	Active     bool   `json:"active"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// ListForStage returns the dynamic field definitions for a stage, ordered
// for form rendering. The frontend renders active ones alongside the
// stage's fixed/core fields without needing a schema migration.
// includeInactive additionally returns deactivated definitions, for the
// Admin field-management screen.
func (s *Service) ListForStage(ctx context.Context, stage string, includeInactive bool) ([]FieldDefinition, error) {
	query := `
		SELECT id, stage, field_key, label, data_type, is_required, sort_order, active
		FROM custom_field_definitions
		WHERE stage = $1`
	if !includeInactive {
		query += ` AND active`
	}
	query += ` ORDER BY sort_order, field_key`

	rows, err := s.pool.Query(ctx, query, stage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	defs := []FieldDefinition{}
	for rows.Next() {
		var d FieldDefinition
		if err := rows.Scan(&d.ID, &d.Stage, &d.FieldKey, &d.Label, &d.DataType, &d.IsRequired, &d.SortOrder, &d.Active); err != nil {
			return nil, err
		}
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

type CreateFieldDefinitionInput struct {
	Stage      string `json:"stage"`
	FieldKey   string `json:"field_key"`
	Label      string `json:"label"`
	DataType   string `json:"data_type"`
	IsRequired bool   `json:"is_required"`
	SortOrder  int    `json:"sort_order"`
}

func (s *Service) Create(ctx context.Context, in CreateFieldDefinitionInput) (FieldDefinition, error) {
	var d FieldDefinition
	err := s.pool.QueryRow(ctx, `
		INSERT INTO custom_field_definitions (stage, field_key, label, data_type, is_required, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, stage, field_key, label, data_type, is_required, sort_order, active`,
		in.Stage, in.FieldKey, in.Label, in.DataType, in.IsRequired, in.SortOrder,
	).Scan(&d.ID, &d.Stage, &d.FieldKey, &d.Label, &d.DataType, &d.IsRequired, &d.SortOrder, &d.Active)
	return d, err
}

func (s *Service) Deactivate(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `UPDATE custom_field_definitions SET active = FALSE WHERE id = $1`, id)
	return err
}

func (s *Service) Reactivate(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `UPDATE custom_field_definitions SET active = TRUE WHERE id = $1`, id)
	return err
}

type UpdateFieldDefinitionInput struct {
	Label      string
	IsRequired bool
	SortOrder  int
}

// Update only allows changing label/required/sort-order — not stage,
// field_key, or data_type, since those are baked into data already saved
// under this field across hardware units.
func (s *Service) Update(ctx context.Context, id int, in UpdateFieldDefinitionInput) (FieldDefinition, error) {
	var d FieldDefinition
	err := s.pool.QueryRow(ctx, `
		UPDATE custom_field_definitions SET label = $2, is_required = $3, sort_order = $4
		WHERE id = $1
		RETURNING id, stage, field_key, label, data_type, is_required, sort_order, active`,
		id, in.Label, in.IsRequired, in.SortOrder,
	).Scan(&d.ID, &d.Stage, &d.FieldKey, &d.Label, &d.DataType, &d.IsRequired, &d.SortOrder, &d.Active)
	return d, err
}

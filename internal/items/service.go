// Package items maintains the admin-managed master list of items (make,
// model, description, qty, sales order number) used to populate the New
// Hardware Unit form's item lookup.
package items

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Item struct {
	ID               int     `json:"id"`
	Make             string  `json:"make"`
	Model            string  `json:"model"`
	Description      *string `json:"description,omitempty"`
	Qty              int     `json:"qty"`
	SalesOrderNumber *string `json:"sales_order_number,omitempty"`
	Active           bool    `json:"active"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

const selectColumns = `id, make, model, description, qty, sales_order_number, active`

func scanItem(row pgx.Row) (Item, error) {
	var it Item
	err := row.Scan(&it.ID, &it.Make, &it.Model, &it.Description, &it.Qty, &it.SalesOrderNumber, &it.Active)
	return it, err
}

// List returns active items, used to populate the New Hardware Unit form's
// item lookup. Admins managing the list separately see inactive ones too.
func (s *Service) List(ctx context.Context, includeInactive bool) ([]Item, error) {
	query := `SELECT ` + selectColumns + ` FROM items`
	if !includeInactive {
		query += ` WHERE active`
	}
	query += ` ORDER BY make, model`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Make, &it.Model, &it.Description, &it.Qty, &it.SalesOrderNumber, &it.Active); err != nil {
			return nil, err
		}
		list = append(list, it)
	}
	return list, rows.Err()
}

type UpsertInput struct {
	Make             string
	Model            string
	Description      *string
	Qty              int
	SalesOrderNumber *string
}

func (s *Service) Create(ctx context.Context, in UpsertInput) (Item, error) {
	return scanItem(s.pool.QueryRow(ctx, `
		INSERT INTO items (make, model, description, qty, sales_order_number) VALUES ($1, $2, $3, $4, $5)
		RETURNING `+selectColumns, in.Make, in.Model, in.Description, in.Qty, in.SalesOrderNumber,
	))
}

func (s *Service) Update(ctx context.Context, id int, in UpsertInput) (Item, error) {
	return scanItem(s.pool.QueryRow(ctx, `
		UPDATE items SET make = $2, model = $3, description = $4, qty = $5, sales_order_number = $6 WHERE id = $1
		RETURNING `+selectColumns, id, in.Make, in.Model, in.Description, in.Qty, in.SalesOrderNumber,
	))
}

func (s *Service) Deactivate(ctx context.Context, id int) error {
	return s.setActive(ctx, id, false)
}

func (s *Service) Reactivate(ctx context.Context, id int) error {
	return s.setActive(ctx, id, true)
}

func (s *Service) setActive(ctx context.Context, id int, active bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE items SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

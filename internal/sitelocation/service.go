// Package sitelocation maintains the admin-managed list of site names used
// to populate the Shipment form's Site Name field, so field/logistics staff
// pick from a consistent list instead of retyping free text each time.
package sitelocation

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNameInUse = errors.New("site location name is already in use")

type SiteLocation struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Active     bool    `json:"active"`
	Region     *string `json:"region,omitempty"`
	IPGateway  *string `json:"ip_gateway,omitempty"`
	SubnetMask *string `json:"subnet_mask,omitempty"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

const selectColumns = `id, name, active, region, ip_gateway, subnet_mask`

func scanLocation(row pgx.Row) (SiteLocation, error) {
	var l SiteLocation
	err := row.Scan(&l.ID, &l.Name, &l.Active, &l.Region, &l.IPGateway, &l.SubnetMask)
	return l, err
}

// List returns active site locations, used to populate the Shipment form's
// suggestions. Admins managing the list separately see inactive ones too.
func (s *Service) List(ctx context.Context, includeInactive bool) ([]SiteLocation, error) {
	query := `SELECT ` + selectColumns + ` FROM site_locations`
	if !includeInactive {
		query += ` WHERE active`
	}
	query += ` ORDER BY name`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locations := []SiteLocation{}
	for rows.Next() {
		var l SiteLocation
		if err := rows.Scan(&l.ID, &l.Name, &l.Active, &l.Region, &l.IPGateway, &l.SubnetMask); err != nil {
			return nil, err
		}
		locations = append(locations, l)
	}
	return locations, rows.Err()
}

type UpsertInput struct {
	Name       string
	Region     *string
	IPGateway  *string
	SubnetMask *string
}

func (s *Service) Create(ctx context.Context, in UpsertInput) (SiteLocation, error) {
	name := strings.TrimSpace(in.Name)
	l, err := scanLocation(s.pool.QueryRow(ctx, `
		INSERT INTO site_locations (name, region, ip_gateway, subnet_mask) VALUES ($1, $2, $3, $4)
		RETURNING `+selectColumns, name, in.Region, in.IPGateway, in.SubnetMask,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return SiteLocation{}, ErrNameInUse
		}
		return SiteLocation{}, err
	}
	return l, nil
}

func (s *Service) Update(ctx context.Context, id int, in UpsertInput) (SiteLocation, error) {
	name := strings.TrimSpace(in.Name)
	l, err := scanLocation(s.pool.QueryRow(ctx, `
		UPDATE site_locations SET name = $2, region = $3, ip_gateway = $4, subnet_mask = $5 WHERE id = $1
		RETURNING `+selectColumns, id, name, in.Region, in.IPGateway, in.SubnetMask,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteLocation{}, pgx.ErrNoRows
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return SiteLocation{}, ErrNameInUse
		}
		return SiteLocation{}, err
	}
	return l, nil
}

func (s *Service) Deactivate(ctx context.Context, id int) error {
	return s.setActive(ctx, id, false)
}

func (s *Service) Reactivate(ctx context.Context, id int) error {
	return s.setActive(ctx, id, true)
}

func (s *Service) setActive(ctx context.Context, id int, active bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE site_locations SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// Package appsettings maintains admin-configurable global settings, stored
// as a single row in app_settings so new settings can be added as columns
// without needing a key/value schema.
package appsettings

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BarcodeLabelSettings struct {
	WidthMM  float64  `json:"barcode_label_width_mm"`
	HeightMM float64  `json:"barcode_label_height_mm"`
	Fields   []string `json:"barcode_label_fields"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) GetBarcodeLabelSettings(ctx context.Context) (BarcodeLabelSettings, error) {
	var settings BarcodeLabelSettings
	err := s.pool.QueryRow(ctx, `
		SELECT barcode_label_width_mm, barcode_label_height_mm, barcode_label_fields FROM app_settings`,
	).Scan(&settings.WidthMM, &settings.HeightMM, &settings.Fields)
	return settings, err
}

// ValidateBarcodeLabelSize is shared by the update endpoint and the preview
// endpoint, which validates a size that may not be saved yet. It only checks
// the physical dimensions — field keys are interpreted (and defensively
// skipped if unknown) by the hardware package that renders the sticker, so
// this package stays decoupled from that field list.
func ValidateBarcodeLabelSize(in BarcodeLabelSettings) error {
	if in.WidthMM < 10 || in.WidthMM > 300 || in.HeightMM < 10 || in.HeightMM > 300 {
		return fmt.Errorf("label dimensions must be between 10mm and 300mm")
	}
	return nil
}

// UpdateBarcodeLabelSettings is registered behind rbac.RequireRole(rbac.Admin).
func (s *Service) UpdateBarcodeLabelSettings(ctx context.Context, in BarcodeLabelSettings) (BarcodeLabelSettings, error) {
	if err := ValidateBarcodeLabelSize(in); err != nil {
		return BarcodeLabelSettings{}, err
	}

	var settings BarcodeLabelSettings
	err := s.pool.QueryRow(ctx, `
		UPDATE app_settings SET barcode_label_width_mm = $1, barcode_label_height_mm = $2, barcode_label_fields = $3
		RETURNING barcode_label_width_mm, barcode_label_height_mm, barcode_label_fields`,
		in.WidthMM, in.HeightMM, in.Fields,
	).Scan(&settings.WidthMM, &settings.HeightMM, &settings.Fields)
	return settings, err
}

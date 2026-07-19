package hardware

import (
	"errors"
	"time"

	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"sbs-bsp-cctv/internal/audit"
)

type UpsertStage1BInput struct {
	FirmwareUpdated bool
	FirmwareVersion *string
	MetaData        map[string]any
}

func (s *Service) UpsertStage1B(ctx context.Context, unitID, actor uuid.UUID, in UpsertStage1BInput) (*Stage1B, error) {
	metaRaw, err := toJSONB(in.MetaData)
	if err != nil {
		return nil, err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO firmware_configurations (hardware_unit_id, firmware_updated, firmware_version, meta_data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			firmware_updated = EXCLUDED.firmware_updated,
			firmware_version = EXCLUDED.firmware_version,
			meta_data = EXCLUDED.meta_data,
			updated_at = now()`,
		unitID, in.FirmwareUpdated, in.FirmwareVersion, metaRaw,
	)
	if err != nil {
		return nil, err
	}

	stage, err := s.GetStage1B(ctx, unitID)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "stage1b_update",
		PerformedBy: &actor, NewValue: stage,
	})
	return stage, nil
}

func scanStage1B(row pgx.Row) (*Stage1B, error) {
	var st Stage1B
	var metaRaw []byte
	err := row.Scan(&st.ID, &st.HardwareUnitID, &st.ConfiguredBy, &st.ConfiguredByName, &st.ConfiguredDate,
		&st.FirmwareUpdated, &st.FirmwareVersion, &st.ConfigurationQCBy, &st.ConfigurationQCByName, &st.QCDate,
		&st.SignedOffAt, &metaRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	st.MetaData, err = fromJSONB(metaRaw)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Service) GetStage1B(ctx context.Context, unitID uuid.UUID) (*Stage1B, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT fc.id, fc.hardware_unit_id, fc.configured_by, cu.full_name, fc.configured_date,
		       fc.firmware_updated, fc.firmware_version, fc.configuration_qc_by, qu.full_name, fc.qc_date,
		       fc.signed_off_at, fc.meta_data
		FROM firmware_configurations fc
		LEFT JOIN users cu ON cu.id = fc.configured_by
		LEFT JOIN users qu ON qu.id = fc.configuration_qc_by
		WHERE fc.hardware_unit_id = $1`, unitID)
	return scanStage1B(row)
}

// SignOffStage1B records the given step (configured|qc). Once both are
// recorded, the record is signed off and the unit's Kanban card moves from
// Configuration to Shipment.
func (s *Service) SignOffStage1B(ctx context.Context, unitID, actor uuid.UUID, step string, performedAt *time.Time) (*Stage1B, error) {
	if step == "qc_fail" {
		if _, err := s.pool.Exec(ctx, `UPDATE firmware_configurations SET configured_by = NULL, configured_date = NULL, updated_at = now() WHERE hardware_unit_id = $1`, unitID); err != nil {
			return nil, err
		}
		st, err := s.GetStage1B(ctx, unitID)
		if err != nil {
			return nil, err
		}
		_ = s.audit.Record(ctx, audit.Entry{
			EntityType: "hardware_unit", EntityID: unitID.String(), Action: "stage1b_qc_failed",
			PerformedBy: &actor, NewValue: st,
		})
		return st, nil
	}

	var byColumn, dateColumn string
	switch step {
	case "configured":
		byColumn, dateColumn = "configured_by", "configured_date"
	case "qc":
		byColumn, dateColumn = "configuration_qc_by", "qc_date"
	default:
		return nil, ErrInvalidSignOffStep
	}
	ts := time.Now()
	if performedAt != nil {
		ts = *performedAt
	}

	_, err := s.pool.Exec(ctx, `UPDATE firmware_configurations SET `+byColumn+` = $2, `+dateColumn+` = $3, updated_at = now() WHERE hardware_unit_id = $1`, unitID, actor, ts)
	if err != nil {
		return nil, err
	}

	st, err := s.GetStage1B(ctx, unitID)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "stage1b_signoff_" + step,
		PerformedBy: &actor, NewValue: st,
	})

	if st.ConfiguredBy != nil && st.ConfigurationQCBy != nil && st.SignedOffAt == nil {
		if _, err := s.pool.Exec(ctx, `UPDATE firmware_configurations SET signed_off_at = $2 WHERE hardware_unit_id = $1`, unitID, ts); err != nil {
			return nil, err
		}
		st.SignedOffAt = &ts

		if err := s.AdvanceBoardColumn(ctx, unitID, "shipment", "logistics"); err != nil {
			return nil, err
		}
		_ = s.audit.Record(ctx, audit.Entry{
			EntityType: "hardware_unit", EntityID: unitID.String(), Action: "stage1b_signed_off",
			PerformedBy: &actor, NewValue: st,
		})
	}

	return st, nil
}

package hardware

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"sbs-bsp-cctv/internal/appsettings"
	"sbs-bsp-cctv/internal/audit"
)

const pgUniqueViolation = "23505"

var (
	ErrNotFound                 = errors.New("not found")
	ErrSerialNumberInUse        = errors.New("serial number is already in use")
	ErrClientIPInUse            = errors.New("client IP address is already in use at this site")
	ErrNotRetired               = errors.New("unit must be retired before it can be permanently deleted")
	ErrInvalidBoardColumn       = errors.New("invalid board column")
	ErrBackwardMoveNotConfirmed = errors.New("moving a unit backward requires confirming that its acceptance sign-off will be cleared")
)

// boardColumnStages gives each board_column_t value the current_stage label
// used when a unit is moved there. Mirrors the stage names each workflow
// stage already sets when it normally advances a unit into that column.
var boardColumnStages = map[string]string{
	"pre_deployment": "receiving",
	"configuration":  "device_configuration_qc",
	"shipment":       "logistics",
	"installation":   "installation",
	"commissioning":  "commissioning",
	"completed":      "completed",
}

var boardColumnOrder = []string{
	"pre_deployment", "configuration", "shipment", "installation", "commissioning", "completed",
}

// readinessQuery, keyed by board_column, mirrors the completion check the
// frontend already runs before allowing a forward drag — each selects
// whether the sign-offs required to leave that column are all in place.
var readinessQuery = map[string]string{
	"pre_deployment": `SELECT encoded_by IS NOT NULL FROM device_configurations WHERE hardware_unit_id = $1`,
	"configuration":  `SELECT configured_by IS NOT NULL AND qc_by IS NOT NULL FROM firmware_configurations WHERE hardware_unit_id = $1`,
	"shipment": `SELECT d.dispatched_at IS NOT NULL AND d.received_at IS NOT NULL
		FROM delivery_dockets d
		JOIN delivery_docket_items i ON i.docket_id = d.id
		WHERE i.hardware_unit_id = $1
		ORDER BY d.created_at DESC
		LIMIT 1`,
	"installation": `SELECT installed_by IS NOT NULL AND inspected_by IS NOT NULL AND fit_focus_by IS NOT NULL FROM site_installations WHERE hardware_unit_id = $1`,
	"commissioning": `SELECT bsp_acceptance_date IS NOT NULL OR client_signed_at IS NOT NULL OR head_office_signed_at IS NOT NULL
		FROM client_acceptances WHERE hardware_unit_id = $1`,
}

type Service struct {
	pool        *pgxpool.Pool
	audit       *audit.Service
	appSettings *appsettings.Service
}

func NewService(pool *pgxpool.Pool, auditSvc *audit.Service, appSettingsSvc *appsettings.Service) *Service {
	return &Service{pool: pool, audit: auditSvc, appSettings: appSettingsSvc}
}

// ---- hardware_units ----

const unitColumns = `id, barcode, alias, serial_number, device_make, device_model, part_number, status, current_stage, board_column, is_exception, allocated_branch, meta_data, created_at, updated_at`

func scanUnit(row pgx.Row) (*Unit, error) {
	var u Unit
	var metaRaw []byte
	err := row.Scan(
		&u.ID, &u.Barcode, &u.Alias, &u.SerialNumber, &u.DeviceMake, &u.DeviceModel, &u.PartNumber,
		&u.Status, &u.CurrentStage, &u.BoardColumn, &u.IsException, &u.AllocatedBranch,
		&metaRaw, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	u.MetaData, err = fromJSONB(metaRaw)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func scanUnitWithSignoff(row pgx.Row) (*Unit, error) {
	var u Unit
	var metaRaw []byte
	err := row.Scan(
		&u.ID, &u.Barcode, &u.Alias, &u.SerialNumber, &u.DeviceMake, &u.DeviceModel, &u.PartNumber,
		&u.Status, &u.CurrentStage, &u.BoardColumn, &u.IsException, &u.AllocatedBranch,
		&metaRaw, &u.CreatedAt, &u.UpdatedAt, &u.Encoded, &u.Configured, &u.QC,
	)
	if err != nil {
		return nil, err
	}
	u.MetaData, err = fromJSONB(metaRaw)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

type CreateUnitInput struct {
	Alias        *string
	SerialNumber string
	DeviceMake   *string
	DeviceModel  *string
	PartNumber   *string
	// Barcode, if non-empty, is used as-is instead of generating one — for
	// units that arrive with a pre-printed sticker already on them.
	Barcode *string
	// AllocatedBranch is the early "intended destination" site/branch
	// assignment, separate from the full site_name/site_location captured
	// later at the Shipment stage.
	AllocatedBranch *string
	MetaData        map[string]any
}

func (s *Service) CreateUnit(ctx context.Context, actor uuid.UUID, in CreateUnitInput) (*Unit, error) {
	metaRaw, err := toJSONB(in.MetaData)
	if err != nil {
		return nil, err
	}

	code := ""
	if in.Barcode != nil && *in.Barcode != "" {
		code = *in.Barcode
	} else {
		code, err = generateBarcode()
		if err != nil {
			return nil, err
		}
	}

	u, err := scanUnit(s.pool.QueryRow(ctx, `
		INSERT INTO hardware_units (barcode, alias, serial_number, device_make, device_model, part_number, allocated_branch, meta_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+unitColumns,
		code, in.Alias, in.SerialNumber, in.DeviceMake, in.DeviceModel, in.PartNumber, in.AllocatedBranch, metaRaw,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "idx_hardware_units_serial_number" {
			return nil, ErrSerialNumberInUse
		}
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: u.ID.String(), Action: "create",
		PerformedBy: &actor, NewValue: u,
	})

	return u, nil
}

// UpdateUnitMetaData replaces the unit's general-stage attributes
// (hardware_units.meta_data) — separate from Stage1A/1B's own meta_data,
// which live on device_configurations/firmware_configurations.
func (s *Service) UpdateUnitMetaData(ctx context.Context, actor, unitID uuid.UUID, metaData map[string]any) (*Unit, error) {
	metaRaw, err := toJSONB(metaData)
	if err != nil {
		return nil, err
	}

	u, err := scanUnit(s.pool.QueryRow(ctx, `
		UPDATE hardware_units SET meta_data = $2, updated_at = now() WHERE id = $1
		RETURNING `+unitColumns, unitID, metaRaw))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "update_attributes",
		PerformedBy: &actor, NewValue: u,
	})
	return u, nil
}

type UpdateUnitIdentityInput struct {
	Alias           *string
	SerialNumber    *string
	DeviceMake      *string
	DeviceModel     *string
	PartNumber      *string
	AllocatedBranch *string
	// Barcode must be non-empty — the column is NOT NULL, since every unit
	// needs a printable identifier.
	Barcode string
}

// UpdateUnitIdentity edits the identifying fields captured at intake
// (hardware_units.alias/serial_number/device_make/device_model/part_number/allocated_branch/barcode).
func (s *Service) UpdateUnitIdentity(ctx context.Context, actor, unitID uuid.UUID, in UpdateUnitIdentityInput) (*Unit, error) {
	u, err := scanUnit(s.pool.QueryRow(ctx, `
		UPDATE hardware_units
		SET alias = $2, serial_number = $3, device_make = $4, device_model = $5, part_number = $6,
			allocated_branch = $7, barcode = $8, updated_at = now()
		WHERE id = $1
		RETURNING `+unitColumns,
		unitID, in.Alias, in.SerialNumber, in.DeviceMake, in.DeviceModel, in.PartNumber, in.AllocatedBranch, in.Barcode))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "idx_hardware_units_serial_number" {
			return nil, ErrSerialNumberInUse
		}
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "update_identity",
		PerformedBy: &actor, NewValue: u,
	})
	return u, nil
}

func (s *Service) GetUnit(ctx context.Context, id uuid.UUID) (*Unit, error) {
	u, err := scanUnit(s.pool.QueryRow(ctx, `SELECT `+unitColumns+` FROM hardware_units WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// FindByBarcodeOrSerial looks up a unit by its barcode or serial number, for
// scan-to-add flows (e.g. delivery docket line items).
func (s *Service) FindByBarcodeOrSerial(ctx context.Context, identifier string) (*Unit, error) {
	u, err := scanUnit(s.pool.QueryRow(ctx,
		`SELECT `+unitColumns+` FROM hardware_units WHERE barcode = $1 OR serial_number = $1`, identifier))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) ListUnits(ctx context.Context, includeArchived bool) ([]Unit, error) {
	query := `SELECT hu.id, hu.barcode, hu.alias, hu.serial_number, hu.device_make, hu.device_model,
			hu.part_number, hu.status, hu.current_stage, hu.board_column, hu.is_exception,
			hu.allocated_branch, hu.meta_data, hu.created_at, hu.updated_at,
			dc.encoded_by IS NOT NULL, dc.configured_by IS NOT NULL, dc.qc_by IS NOT NULL
		FROM hardware_units hu
		LEFT JOIN device_configurations dc ON dc.hardware_unit_id = hu.id`
	if !includeArchived {
		query += ` WHERE hu.status != 'retired'`
	}
	query += ` ORDER BY hu.created_at DESC`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	units := []Unit{}
	for rows.Next() {
		u, err := scanUnitWithSignoff(rows)
		if err != nil {
			return nil, err
		}
		units = append(units, *u)
	}
	return units, rows.Err()
}

// DeleteUnit retires the unit rather than removing its row, so its full
// history (stage records, audit trail) stays intact and reversible.
func (s *Service) DeleteUnit(ctx context.Context, actor, unitID uuid.UUID) error {
	if err := s.SetStatus(ctx, unitID, "retired"); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "retire",
		PerformedBy: &actor,
	})
	return nil
}

// RestoreUnit reverses a retire, putting the unit back to active status.
func (s *Service) RestoreUnit(ctx context.Context, actor, unitID uuid.UUID) error {
	if err := s.SetStatus(ctx, unitID, "active"); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "restore",
		PerformedBy: &actor,
	})
	return nil
}

// PurgeUnit permanently deletes a retired unit and all its related records
// (via ON DELETE CASCADE). This is irreversible — only retired units may be
// purged, and the pre-delete snapshot is kept in the audit trail.
func (s *Service) PurgeUnit(ctx context.Context, actor, unitID uuid.UUID) error {
	u, err := s.GetUnit(ctx, unitID)
	if err != nil {
		return err
	}
	if u.Status != "retired" {
		return ErrNotRetired
	}

	tag, err := s.pool.Exec(ctx, `DELETE FROM hardware_units WHERE id = $1 AND status = 'retired'`, unitID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotRetired
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "purge",
		PerformedBy: &actor, OldValue: u,
	})
	return nil
}

// DistinctDeviceMakes and DistinctDeviceModels back the New Unit form's
// creatable make/model comboboxes — suggestions drawn from what's already
// been entered, with free text always allowed for a genuinely new value.
func (s *Service) DistinctDeviceMakes(ctx context.Context) ([]string, error) {
	return s.distinctColumn(ctx, "device_make")
}

func (s *Service) DistinctDeviceModels(ctx context.Context) ([]string, error) {
	return s.distinctColumn(ctx, "device_model")
}

// allowed columns for distinctColumn — guards against the string-built query
// below ever being reachable with anything other than these fixed literals.
var distinctColumnAllowlist = map[string]bool{
	"device_make":  true,
	"device_model": true,
}

func (s *Service) distinctColumn(ctx context.Context, column string) ([]string, error) {
	if !distinctColumnAllowlist[column] {
		return nil, fmt.Errorf("distinctColumn: column %q is not allowlisted", column)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT `+column+` FROM hardware_units
		WHERE `+column+` IS NOT NULL AND `+column+` != ''
		ORDER BY `+column)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func (s *Service) AdvanceBoardColumn(ctx context.Context, unitID uuid.UUID, column, stage string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE hardware_units SET board_column = $2, current_stage = $3, updated_at = now() WHERE id = $1`,
		unitID, column, stage)
	return err
}

func boardColumnIndex(column string) int {
	for i, c := range boardColumnOrder {
		if c == column {
			return i
		}
	}
	return -1
}

// MoveBoardColumn is an admin-only manual override that moves a unit to any
// board column, including backward — the normal AdvanceBoardColumn path only
// ever moves a unit forward, as a side effect of completing that stage's
// sign-off. Moving backward is refused unless resetAcceptance is true: a
// unit dragged back past Commissioning would otherwise keep its old BSP/
// client acceptance sign-off, letting SyncBoardColumn silently re-complete
// it later using stale data. Confirming the reset clears that record so the
// unit genuinely has to be re-accepted.
func (s *Service) MoveBoardColumn(ctx context.Context, actor, unitID uuid.UUID, column string, resetAcceptance bool) (*Unit, error) {
	stage, ok := boardColumnStages[column]
	if !ok {
		return nil, ErrInvalidBoardColumn
	}

	current, err := s.GetUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	isBackward := boardColumnIndex(column) < boardColumnIndex(current.BoardColumn)
	if isBackward && !resetAcceptance {
		return nil, ErrBackwardMoveNotConfirmed
	}

	if err := s.AdvanceBoardColumn(ctx, unitID, column, stage); err != nil {
		return nil, err
	}

	if isBackward {
		if _, err := s.pool.Exec(ctx, `DELETE FROM client_acceptances WHERE hardware_unit_id = $1`, unitID); err != nil {
			return nil, err
		}
	}

	u, err := s.GetUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}

	action := "manual_board_move"
	if isBackward {
		action = "manual_board_move_backward"
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: action,
		PerformedBy: &actor, NewValue: u,
	})
	return u, nil
}

// SyncBoardColumn re-derives whether the unit is ready to leave its current
// board column (by re-checking the underlying sign-off tables, not trusting
// the caller) and, if so, advances it. This exists because AdvanceBoardColumn
// only runs as a side effect of a *new* sign-off — a unit whose column was
// moved backward (or was signed off before board_column tracking caught up)
// has nothing left to trigger a fresh advance, so the board needs a way to
// resync board_column with already-completed sign-offs. Open to any
// authenticated user: it can only ever catch board_column up to state that a
// properly-authorized sign-off already produced, never grant a new one.
func (s *Service) SyncBoardColumn(ctx context.Context, actor, unitID uuid.UUID) (*Unit, bool, error) {
	u, err := s.GetUnit(ctx, unitID)
	if err != nil {
		return nil, false, err
	}

	idx := -1
	for i, c := range boardColumnOrder {
		if c == u.BoardColumn {
			idx = i
			break
		}
	}
	if idx < 0 || idx == len(boardColumnOrder)-1 {
		return u, false, nil
	}

	query, ok := readinessQuery[u.BoardColumn]
	if !ok {
		return u, false, nil
	}
	var ready bool
	err = s.pool.QueryRow(ctx, query, unitID).Scan(&ready)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !ready {
		return u, false, nil
	}

	nextColumn := boardColumnOrder[idx+1]
	if err := s.AdvanceBoardColumn(ctx, unitID, nextColumn, boardColumnStages[nextColumn]); err != nil {
		return nil, false, err
	}
	u, err = s.GetUnit(ctx, unitID)
	if err != nil {
		return nil, false, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "board_column_sync",
		PerformedBy: &actor, NewValue: u,
	})
	return u, true, nil
}

func (s *Service) SetException(ctx context.Context, unitID uuid.UUID, isException bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE hardware_units SET is_exception = $2, updated_at = now() WHERE id = $1`, unitID, isException)
	return err
}

func (s *Service) SetStatus(ctx context.Context, unitID uuid.UUID, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE hardware_units SET status = $2, updated_at = now() WHERE id = $1`, unitID, status)
	return err
}

// ---- Stage 0: receiving ----

const escalationSupplierContact = "CH10"

const receivingColumns = `id, hardware_unit_id, received_by, received_date, po_or_waybill_ref, items_correct, discrepancy_notes, escalated_to, escalated_at`

func scanReceiving(row pgx.Row) (*Stage0Receiving, error) {
	var r Stage0Receiving
	err := row.Scan(&r.ID, &r.HardwareUnitID, &r.ReceivedBy, &r.ReceivedDate, &r.PoOrWaybillRef, &r.ItemsCorrect, &r.DiscrepancyNotes, &r.EscalatedTo, &r.EscalatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Service) GetReceiving(ctx context.Context, unitID uuid.UUID) (*Stage0Receiving, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+receivingColumns+` FROM goods_receipts WHERE hardware_unit_id = $1`, unitID)
	return scanReceiving(row)
}

type RecordReceivingInput struct {
	ReceivedBy       uuid.UUID
	PoOrWaybillRef   *string
	ItemsCorrect     bool
	DiscrepancyNotes *string
}

func (s *Service) RecordReceiving(ctx context.Context, unitID uuid.UUID, in RecordReceivingInput) (*Stage0Receiving, error) {
	var escalatedTo *string
	var escalatedAt *time.Time
	if !in.ItemsCorrect {
		contact := escalationSupplierContact
		now := time.Now()
		escalatedTo = &contact
		escalatedAt = &now
	}

	var r Stage0Receiving
	err := s.pool.QueryRow(ctx, `
		INSERT INTO goods_receipts
			(hardware_unit_id, received_by, po_or_waybill_ref, items_correct, discrepancy_notes, escalated_to, escalated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			received_by = EXCLUDED.received_by,
			po_or_waybill_ref = EXCLUDED.po_or_waybill_ref,
			items_correct = EXCLUDED.items_correct,
			discrepancy_notes = EXCLUDED.discrepancy_notes,
			escalated_to = EXCLUDED.escalated_to,
			escalated_at = EXCLUDED.escalated_at
		RETURNING id, hardware_unit_id, received_by, received_date, po_or_waybill_ref, items_correct, discrepancy_notes, escalated_to, escalated_at`,
		unitID, in.ReceivedBy, in.PoOrWaybillRef, in.ItemsCorrect, in.DiscrepancyNotes, escalatedTo, escalatedAt,
	).Scan(&r.ID, &r.HardwareUnitID, &r.ReceivedBy, &r.ReceivedDate, &r.PoOrWaybillRef, &r.ItemsCorrect, &r.DiscrepancyNotes, &r.EscalatedTo, &r.EscalatedAt)
	if err != nil {
		return nil, err
	}

	if err := s.SetException(ctx, unitID, !in.ItemsCorrect); err != nil {
		return nil, err
	}

	action := "receiving_confirmed"
	if !in.ItemsCorrect {
		action = "receiving_discrepancy"
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: action,
		PerformedBy: &in.ReceivedBy, NewValue: r,
	})

	return &r, nil
}

// ---- Store custody ----

type StoreCustodyInput struct {
	Action  string // checked_out | checked_in
	Purpose string // for_configuration | for_dispatch
	By      uuid.UUID
	Notes   *string
}

func (s *Service) LogStoreCustody(ctx context.Context, unitID uuid.UUID, in StoreCustodyInput) error {
	if in.Action != "checked_out" && in.Action != "checked_in" {
		return fmt.Errorf("invalid action %q", in.Action)
	}
	if in.Purpose != "for_configuration" && in.Purpose != "for_dispatch" {
		return fmt.Errorf("invalid purpose %q", in.Purpose)
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO store_custody_logs (hardware_unit_id, action, by_user, purpose, notes)
		VALUES ($1, $2, $3, $4, $5)`, unitID, in.Action, in.By, in.Purpose, in.Notes)
	if err != nil {
		return err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "store_" + in.Action,
		PerformedBy: &in.By, Notes: in.Purpose,
	})
	return nil
}

package logistics

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sbs-bsp-cctv/internal/audit"
	"sbs-bsp-cctv/internal/hardware"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrDocketLocked  = errors.New("docket has already been dispatched")
	ErrItemDuplicate = errors.New("unit is already on this docket")
	ErrUnitNotReady  = errors.New("unit is not in the shipment stage yet")
)

type Service struct {
	pool     *pgxpool.Pool
	audit    *audit.Service
	hardware *hardware.Service
}

func NewService(pool *pgxpool.Pool, auditSvc *audit.Service, hardwareSvc *hardware.Service) *Service {
	return &Service{pool: pool, audit: auditSvc, hardware: hardwareSvc}
}

func toJSONB(v map[string]any) ([]byte, error) {
	if v == nil {
		v = map[string]any{}
	}
	return json.Marshal(v)
}

func fromJSONB(b []byte) (map[string]any, error) {
	m := map[string]any{}
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

const docketColumns = `id, docket_number, waybill_number, shipped_via, shipping_provider, destination_site,
	status, dispatched_by, dispatched_at, received_by, received_at, notes, meta_data, created_at, updated_at`

func scanDocket(row pgx.Row) (*Docket, error) {
	var d Docket
	var metaRaw []byte
	err := row.Scan(&d.ID, &d.DocketNumber, &d.WaybillNumber, &d.ShippedVia, &d.ShippingProvider,
		&d.DestinationSite, &d.Status, &d.DispatchedBy, &d.DispatchedAt, &d.ReceivedBy, &d.ReceivedAt,
		&d.Notes, &metaRaw, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.MetaData, err = fromJSONB(metaRaw)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ---- Docket CRUD ----

type CreateDocketInput struct {
	WaybillNumber    *string
	ShippedVia       *string
	ShippingProvider *string
	DestinationSite  *string
	Notes            *string
	MetaData         map[string]any
}

func (s *Service) CreateDocket(ctx context.Context, actor uuid.UUID, in CreateDocketInput) (*Docket, error) {
	metaRaw, err := toJSONB(in.MetaData)
	if err != nil {
		return nil, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO delivery_dockets (waybill_number, shipped_via, shipping_provider, destination_site, notes, meta_data, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+docketColumns,
		in.WaybillNumber, in.ShippedVia, in.ShippingProvider, in.DestinationSite, in.Notes, metaRaw, actor,
	)

	docket, err := scanDocket(row)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "delivery_docket", EntityID: docket.ID.String(), Action: "docket_created",
		PerformedBy: &actor, NewValue: docket,
	})
	return docket, nil
}

type ListDocketsFilter struct {
	Status string
}

func (s *Service) ListDockets(ctx context.Context, filter ListDocketsFilter) ([]Docket, error) {
	query := `SELECT ` + docketColumns + `, (SELECT count(*) FROM delivery_docket_items i WHERE i.docket_id = d.id)
		FROM delivery_dockets d`
	args := []any{}
	if filter.Status != "" {
		query += ` WHERE status = $1`
		args = append(args, filter.Status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dockets := []Docket{}
	for rows.Next() {
		var d Docket
		var metaRaw []byte
		if err := rows.Scan(&d.ID, &d.DocketNumber, &d.WaybillNumber, &d.ShippedVia, &d.ShippingProvider,
			&d.DestinationSite, &d.Status, &d.DispatchedBy, &d.DispatchedAt, &d.ReceivedBy, &d.ReceivedAt,
			&d.Notes, &metaRaw, &d.CreatedAt, &d.UpdatedAt, &d.ItemCount); err != nil {
			return nil, err
		}
		d.MetaData, err = fromJSONB(metaRaw)
		if err != nil {
			return nil, err
		}
		dockets = append(dockets, d)
	}
	return dockets, rows.Err()
}

func (s *Service) GetDocket(ctx context.Context, docketID uuid.UUID) (*DocketDetail, error) {
	docket, err := scanDocket(s.pool.QueryRow(ctx, `SELECT `+docketColumns+` FROM delivery_dockets WHERE id = $1`, docketID))
	if err != nil {
		return nil, err
	}

	items, err := s.listItems(ctx, docketID)
	if err != nil {
		return nil, err
	}
	events, err := s.listTrackingEvents(ctx, docketID)
	if err != nil {
		return nil, err
	}

	docket.ItemCount = len(items)
	return &DocketDetail{Docket: *docket, Items: items, TrackingEvents: events}, nil
}

// ---- Items (detail lines) ----

func (s *Service) listItems(ctx context.Context, docketID uuid.UUID) ([]DocketItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT i.id, i.docket_id, i.hardware_unit_id, hu.barcode, hu.serial_number, hu.device_make, hu.device_model,
		       hu.part_number, i.serial_number_snapshot, i.added_at
		FROM delivery_docket_items i
		JOIN hardware_units hu ON hu.id = i.hardware_unit_id
		WHERE i.docket_id = $1
		ORDER BY i.added_at`, docketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []DocketItem{}
	for rows.Next() {
		var it DocketItem
		if err := rows.Scan(&it.ID, &it.DocketID, &it.HardwareUnitID, &it.Barcode, &it.SerialNumber,
			&it.DeviceMake, &it.DeviceModel, &it.PartNumber, &it.SerialNumberSnapshot, &it.AddedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// AddItem adds a hardware unit to a draft docket, looked up by barcode or
// serial number (scan-to-add). The docket must still be in draft status.
func (s *Service) AddItem(ctx context.Context, docketID, actor uuid.UUID, identifier string) (*DocketItem, error) {
	docket, err := scanDocket(s.pool.QueryRow(ctx, `SELECT `+docketColumns+` FROM delivery_dockets WHERE id = $1`, docketID))
	if err != nil {
		return nil, err
	}
	if docket.Status != "draft" {
		return nil, ErrDocketLocked
	}

	unit, err := s.hardware.FindByBarcodeOrSerial(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if unit.BoardColumn != "shipment" {
		return nil, ErrUnitNotReady
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO delivery_docket_items (docket_id, hardware_unit_id, serial_number_snapshot, added_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (docket_id, hardware_unit_id) DO NOTHING
		RETURNING id, docket_id, hardware_unit_id, serial_number_snapshot, added_at`,
		docketID, unit.ID, unit.SerialNumber, actor,
	)

	var it DocketItem
	if err := row.Scan(&it.ID, &it.DocketID, &it.HardwareUnitID, &it.SerialNumberSnapshot, &it.AddedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemDuplicate
		}
		return nil, err
	}
	it.Barcode = unit.Barcode
	it.SerialNumber = unit.SerialNumber
	it.DeviceMake = unit.DeviceMake
	it.DeviceModel = unit.DeviceModel
	it.PartNumber = unit.PartNumber

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "delivery_docket", EntityID: docketID.String(), Action: "docket_item_added",
		PerformedBy: &actor, NewValue: it,
	})
	return &it, nil
}

func (s *Service) RemoveItem(ctx context.Context, docketID, itemID, actor uuid.UUID) error {
	docket, err := scanDocket(s.pool.QueryRow(ctx, `SELECT `+docketColumns+` FROM delivery_dockets WHERE id = $1`, docketID))
	if err != nil {
		return err
	}
	if docket.Status != "draft" {
		return ErrDocketLocked
	}

	tag, err := s.pool.Exec(ctx, `DELETE FROM delivery_docket_items WHERE id = $1 AND docket_id = $2`, itemID, docketID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "delivery_docket", EntityID: docketID.String(), Action: "docket_item_removed",
		PerformedBy: &actor, NewValue: map[string]any{"item_id": itemID},
	})
	return nil
}

// ---- Sign-off: dispatch / receive ----

type RecordDispatchInput struct {
	DispatchedBy     string
	DispatchedAt     *time.Time
	WaybillNumber    *string
	ShippedVia       *string
	ShippingProvider *string
}

// RecordDispatch marks the docket dispatched, logs a system tracking event,
// and checks every item's unit out of store custody. The waybill number,
// carrier, and shipping provider are usually only known once the shipment is
// actually booked, so they're captured here rather than at docket creation.
func (s *Service) RecordDispatch(ctx context.Context, docketID, actor uuid.UUID, in RecordDispatchInput) (*DocketDetail, error) {
	when := time.Now()
	if in.DispatchedAt != nil {
		when = *in.DispatchedAt
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE delivery_dockets SET status = 'dispatched', dispatched_by = $2, dispatched_at = $3,
			waybill_number = COALESCE($4, waybill_number), shipped_via = COALESCE($5, shipped_via),
			shipping_provider = COALESCE($6, shipping_provider), updated_at = now()
		WHERE id = $1`, docketID, in.DispatchedBy, when, in.WaybillNumber, in.ShippedVia, in.ShippingProvider)
	if err != nil {
		return nil, err
	}

	items, err := s.listItems(ctx, docketID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if err := s.hardware.LogStoreCustody(ctx, item.HardwareUnitID, hardware.StoreCustodyInput{
			Action: "checked_out", Purpose: "for_dispatch", By: actor,
		}); err != nil {
			return nil, err
		}
	}

	if err := s.recordTrackingEvent(ctx, docketID, actor, strPtr("dispatched"), "Dispatched", when, true); err != nil {
		return nil, err
	}

	detail, err := s.GetDocket(ctx, docketID)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "delivery_docket", EntityID: docketID.String(), Action: "docket_dispatched",
		PerformedBy: &actor, NewValue: detail,
	})
	return detail, nil
}

// RecordReceipt marks the docket delivered, logs a system tracking event,
// checks every item's unit back into custody, and advances each unit's
// Kanban card into Installation.
func (s *Service) RecordReceipt(ctx context.Context, docketID, actor uuid.UUID, receivedBy string, at *time.Time) (*DocketDetail, error) {
	when := time.Now()
	if at != nil {
		when = *at
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE delivery_dockets SET status = 'delivered', received_by = $2, received_at = $3, updated_at = now()
		WHERE id = $1`, docketID, receivedBy, when)
	if err != nil {
		return nil, err
	}

	items, err := s.listItems(ctx, docketID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if err := s.hardware.LogStoreCustody(ctx, item.HardwareUnitID, hardware.StoreCustodyInput{
			Action: "checked_in", Purpose: "for_dispatch", By: actor,
		}); err != nil {
			return nil, err
		}
		if err := s.hardware.AdvanceBoardColumn(ctx, item.HardwareUnitID, "installation", "installation"); err != nil {
			return nil, err
		}
	}

	if err := s.recordTrackingEvent(ctx, docketID, actor, strPtr("delivered"), "Delivered", when, true); err != nil {
		return nil, err
	}

	detail, err := s.GetDocket(ctx, docketID)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "delivery_docket", EntityID: docketID.String(), Action: "docket_delivered",
		PerformedBy: &actor, NewValue: detail,
	})
	return detail, nil
}

// ---- Tracking events ----

func strPtr(s string) *string { return &s }

func (s *Service) listTrackingEvents(ctx context.Context, docketID uuid.UUID) ([]TrackingEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, docket_id, status, description, occurred_at, is_system_generated, created_at
		FROM delivery_docket_tracking_events
		WHERE docket_id = $1
		ORDER BY occurred_at DESC`, docketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []TrackingEvent{}
	for rows.Next() {
		var e TrackingEvent
		if err := rows.Scan(&e.ID, &e.DocketID, &e.Status, &e.Description, &e.OccurredAt, &e.IsSystemGenerated, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Service) recordTrackingEvent(ctx context.Context, docketID, actor uuid.UUID, status *string, description string, occurredAt time.Time, systemGenerated bool) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO delivery_docket_tracking_events (docket_id, status, description, occurred_at, is_system_generated, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		docketID, status, description, occurredAt, systemGenerated, actor)
	return err
}

// AddTrackingEvent powers both the one-click stage buttons (In Transit /
// Arrived at Site — systemGenerated=true, status set) and free-text manual
// entries with an optional custom date/time (systemGenerated=false).
func (s *Service) AddTrackingEvent(ctx context.Context, docketID, actor uuid.UUID, status *string, description string, occurredAt *time.Time, systemGenerated bool) (*TrackingEvent, error) {
	when := time.Now()
	if occurredAt != nil {
		when = *occurredAt
	}

	if status != nil && *status != "" {
		if _, err := s.pool.Exec(ctx, `
			UPDATE delivery_dockets SET status = $2, updated_at = now()
			WHERE id = $1 AND status NOT IN ('delivered')`, docketID, *status); err != nil {
			return nil, err
		}
	}

	if err := s.recordTrackingEvent(ctx, docketID, actor, status, description, when, systemGenerated); err != nil {
		return nil, err
	}

	events, err := s.listTrackingEvents(ctx, docketID)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, ErrNotFound
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "delivery_docket", EntityID: docketID.String(), Action: "docket_tracking_event",
		PerformedBy: &actor, NewValue: events[0],
	})
	return &events[0], nil
}

// ---- Per-unit read-only view ----

// GetUnitLogistics finds the most recent docket carrying this unit and
// returns its waybill/sign-off details plus its full tracking timeline, for
// the unit's read-only Logistics tab.
func (s *Service) GetUnitLogistics(ctx context.Context, unitID uuid.UUID) (*UnitLogisticsView, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT d.id FROM delivery_dockets d
		JOIN delivery_docket_items i ON i.docket_id = d.id
		WHERE i.hardware_unit_id = $1
		ORDER BY d.created_at DESC
		LIMIT 1`, unitID)

	var docketID uuid.UUID
	err := row.Scan(&docketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return &UnitLogisticsView{Docket: nil, TrackingEvents: []TrackingEvent{}}, nil
	}
	if err != nil {
		return nil, err
	}

	docket, err := scanDocket(s.pool.QueryRow(ctx, `SELECT `+docketColumns+` FROM delivery_dockets WHERE id = $1`, docketID))
	if err != nil {
		return nil, err
	}
	events, err := s.listTrackingEvents(ctx, docketID)
	if err != nil {
		return nil, err
	}

	return &UnitLogisticsView{Docket: docket, TrackingEvents: events}, nil
}

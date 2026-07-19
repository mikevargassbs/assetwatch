package defective

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sbs-bsp-cctv/internal/audit"
	"sbs-bsp-cctv/internal/hardware"
	"sbs-bsp-cctv/internal/mailer"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrInvalidDefectType  = errors.New("invalid defect type")
	ErrInvalidReplacement = errors.New("replacement can only be recorded once the unit has been shipped back")
	ErrNotShippedYet      = errors.New("unit must be marked shipped back before this step")
)

type Service struct {
	pool     *pgxpool.Pool
	audit    *audit.Service
	hardware *hardware.Service
	mailCfg  mailer.Config
}

func NewService(pool *pgxpool.Pool, auditSvc *audit.Service, hardwareSvc *hardware.Service, mailCfg mailer.Config) *Service {
	return &Service{pool: pool, audit: auditSvc, hardware: hardwareSvc, mailCfg: mailCfg}
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

const selectColumns = `id, hardware_unit_id, declared_by, declared_date, defect_type, description,
	report_generated_at, emailed_to_supplier_at, supplier_email, replacement_status,
	replacement_hardware_unit_id, tracking_number, carrier, shipped_back_at, delivered_at,
	supplier_received_at, meta_data`

func scanDefect(row pgx.Row) (*DefectReport, error) {
	var d DefectReport
	var metaRaw []byte
	err := row.Scan(&d.ID, &d.HardwareUnitID, &d.DeclaredBy, &d.DeclaredDate, &d.DefectType, &d.Description,
		&d.ReportGeneratedAt, &d.EmailedToSupplierAt, &d.SupplierEmail, &d.ReplacementStatus,
		&d.ReplacementHardwareUnitID, &d.TrackingNumber, &d.Carrier, &d.ShippedBackAt, &d.DeliveredAt,
		&d.SupplierReceivedAt, &metaRaw)
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

func (s *Service) GetDefectReport(ctx context.Context, unitID uuid.UUID) (*DefectReport, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+selectColumns+` FROM defect_reports WHERE hardware_unit_id = $1`, unitID)
	return scanDefect(row)
}

var validDefectTypes = map[string]bool{"defective": true, "damaged": true, "wrong_item": true}

type DeclareDefectInput struct {
	DefectType  string
	Description *string
}

// DeclareDefect records the defect, flags the unit as an exception on the
// Kanban board, and marks the unit's status so it's excluded from normal
// "in progress" reporting. This can happen at any stage of the unit's
// lifecycle — it's a branch off the main flow, not a forward stage.
func (s *Service) DeclareDefect(ctx context.Context, unitID, actor uuid.UUID, in DeclareDefectInput) (*DefectReport, error) {
	if !validDefectTypes[in.DefectType] {
		return nil, ErrInvalidDefectType
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO defect_reports (hardware_unit_id, declared_by, defect_type, description)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			declared_by = EXCLUDED.declared_by,
			declared_date = now(),
			defect_type = EXCLUDED.defect_type,
			description = EXCLUDED.description,
			updated_at = now()
		RETURNING `+selectColumns,
		unitID, actor, in.DefectType, in.Description,
	)

	d, err := scanDefect(row)
	if err != nil {
		return nil, err
	}

	if err := s.hardware.SetException(ctx, unitID, true); err != nil {
		return nil, err
	}
	if err := s.hardware.SetStatus(ctx, unitID, "defective"); err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "defect_declared",
		PerformedBy: &actor, NewValue: d,
	})
	return d, nil
}

// GenerateReport renders a printable PDF describing the declared defect,
// and marks it as generated.
func (s *Service) GenerateReport(ctx context.Context, unitID uuid.UUID) ([]byte, error) {
	pdfBytes, err := s.renderReportPDF(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE defect_reports SET report_generated_at = now() WHERE hardware_unit_id = $1`, unitID); err != nil {
		return nil, err
	}
	return pdfBytes, nil
}

// EmailToSupplier attempts to send the defect report to the supplier by
// email. If SMTP isn't configured, the intended action is still recorded
// (supplier_email + emailed_to_supplier_at) so the workflow isn't blocked
// on infrastructure that may not exist yet in a given deployment.
func (s *Service) EmailToSupplier(ctx context.Context, unitID, actor uuid.UUID, supplierEmail string) (*DefectReport, bool, error) {
	pdfBytes, err := s.renderReportPDF(ctx, unitID)
	if err != nil {
		return nil, false, err
	}

	unit, err := s.hardware.GetUnit(ctx, unitID)
	if err != nil {
		return nil, false, err
	}

	sendErr := mailer.Send(s.mailCfg, supplierEmail,
		fmt.Sprintf("Defect Report — %s", unit.Barcode),
		fmt.Sprintf("Please find attached the defect report for hardware unit %s.", unit.Barcode),
		&mailer.Attachment{Filename: "defect-report.pdf", ContentType: "application/pdf", Data: pdfBytes},
	)
	sent := sendErr == nil
	if sendErr != nil && !errors.Is(sendErr, mailer.ErrNotConfigured) {
		return nil, false, sendErr
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE defect_reports SET
			supplier_email = $2,
			emailed_to_supplier_at = now(),
			report_generated_at = COALESCE(report_generated_at, now()),
			updated_at = now()
		WHERE hardware_unit_id = $1
		RETURNING `+selectColumns,
		unitID, supplierEmail,
	)

	d, err := scanDefect(row)
	if err != nil {
		return nil, false, err
	}

	action := "defect_emailed_to_supplier"
	notes := "sent via SMTP"
	if !sent {
		notes = "SMTP not configured; recorded without sending"
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: action,
		PerformedBy: &actor, NewValue: d, Notes: notes,
	})

	return d, sent, nil
}

type MarkShippedBackInput struct {
	TrackingNumber *string
	Carrier        *string
}

// MarkShippedBack records that the defective unit has been shipped back to
// the supplier. The unit stays "defective" (and visible/actionable) rather
// than being retired — being in transit back to the supplier isn't the same
// as the unit being permanently out of service.
func (s *Service) MarkShippedBack(ctx context.Context, unitID, actor uuid.UUID, in MarkShippedBackInput) (*DefectReport, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE defect_reports SET
			replacement_status = 'shipped_back',
			shipped_back_at = now(),
			tracking_number = $2,
			carrier = $3,
			updated_at = now()
		WHERE hardware_unit_id = $1
		RETURNING `+selectColumns, unitID, in.TrackingNumber, in.Carrier,
	)
	d, err := scanDefect(row)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "defect_shipped_back",
		PerformedBy: &actor, NewValue: d,
	})
	return d, nil
}

// MarkDelivered records that the carrier has delivered the shipment to the
// supplier's premises. This is a manual confirmation step (e.g. from a
// carrier tracking page) — there's no carrier integration yet.
func (s *Service) MarkDelivered(ctx context.Context, unitID, actor uuid.UUID) (*DefectReport, error) {
	existing, err := s.GetDefectReport(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if existing.ShippedBackAt == nil {
		return nil, ErrNotShippedYet
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE defect_reports SET delivered_at = now(), updated_at = now()
		WHERE hardware_unit_id = $1
		RETURNING `+selectColumns, unitID,
	)
	d, err := scanDefect(row)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "defect_shipment_delivered",
		PerformedBy: &actor, NewValue: d,
	})
	return d, nil
}

// MarkSupplierReceived records that the supplier has confirmed receipt of
// the returned unit — the last step of the outbound shipment before a
// replacement can be recorded.
func (s *Service) MarkSupplierReceived(ctx context.Context, unitID, actor uuid.UUID) (*DefectReport, error) {
	existing, err := s.GetDefectReport(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if existing.ShippedBackAt == nil {
		return nil, ErrNotShippedYet
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE defect_reports SET supplier_received_at = now(), updated_at = now()
		WHERE hardware_unit_id = $1
		RETURNING `+selectColumns, unitID,
	)
	d, err := scanDefect(row)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "defect_shipment_received_by_supplier",
		PerformedBy: &actor, NewValue: d,
	})
	return d, nil
}

// RecordReplacement creates a brand-new hardware unit (starting fresh at
// Pre-Deployment) to replace the defective one, and links the two records.
// "We do the process again" — the replacement goes through the full
// lifecycle independently; it is not the same physical unit.
func (s *Service) RecordReplacement(ctx context.Context, unitID, actor uuid.UUID, replacementSerialNumber string) (*DefectReport, *hardware.Unit, error) {
	existing, err := s.GetDefectReport(ctx, unitID)
	if err != nil {
		return nil, nil, err
	}
	if existing.ReplacementStatus != "shipped_back" {
		return nil, nil, ErrInvalidReplacement
	}

	replacementUnit, err := s.hardware.CreateUnit(ctx, actor, hardware.CreateUnitInput{SerialNumber: replacementSerialNumber})
	if err != nil {
		return nil, nil, err
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE defect_reports SET
			replacement_status = 'replacement_received',
			replacement_hardware_unit_id = $2,
			updated_at = now()
		WHERE hardware_unit_id = $1
		RETURNING `+selectColumns,
		unitID, replacementUnit.ID,
	)
	d, err := scanDefect(row)
	if err != nil {
		return nil, nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "defect_replacement_received",
		PerformedBy: &actor, NewValue: d,
	})

	return d, replacementUnit, nil
}

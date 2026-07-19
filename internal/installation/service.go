package installation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sbs-bsp-cctv/internal/audit"
	"sbs-bsp-cctv/internal/hardware"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrInvalidSignOffStep = errors.New("invalid sign-off step")
	ErrMaxPhotos          = errors.New("maximum number of photos reached")
	ErrPhotoRequired      = errors.New("at least one installation photo is required")
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

func scanInstallation(row pgx.Row) (*SiteInstallation, error) {
	var s SiteInstallation
	var metaRaw []byte
	err := row.Scan(&s.ID, &s.HardwareUnitID, &s.ReceivedOnSiteAt, &s.ConfirmedCorrect, &s.DiscrepancyNotes,
		&s.EscalatedToPMPCAt, &s.DateInstalled, &s.InstalledBy, &s.InstalledAt, &s.InspectedBy, &s.InspectedAt,
		&s.FitFocusBy, &s.FitFocusCompletedAt, &s.NetworkAttached, &s.DeviceContactable, &s.PingCheckedAt,
		&s.InstalledLocation, &s.InstalledHeightM, &s.SignedOffAt,
		&s.SiteName, &s.SiteLocation, &s.SiteIP, &s.SiteSubnet, &s.SiteGateway,
		&s.DeploymentDate, &s.DeploymentTeam, &s.TeamLeader, &metaRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.MetaData, err = fromJSONB(metaRaw)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

const selectColumns = `id, hardware_unit_id, received_on_site_at, confirmed_correct, discrepancy_notes,
	escalated_to_pmpc_at, date_installed, installed_by, installed_at, inspected_by, inspected_at,
	fit_focus_by, fit_focus_completed_at, network_attached, device_contactable, ping_checked_at,
	installed_location, installed_height_m, signed_off_at,
	site_name, site_location, host(site_ip), site_subnet::text, host(site_gateway),
	deployment_date, deployment_team, team_leader, meta_data`

const selectColumnsWithNames = `si.id, si.hardware_unit_id, si.received_on_site_at, si.confirmed_correct, si.discrepancy_notes,
	si.escalated_to_pmpc_at, si.date_installed, si.installed_by, iu.full_name, si.installed_at,
	si.inspected_by, nu.full_name, si.inspected_at,
	si.fit_focus_by, fu.full_name, si.fit_focus_completed_at, si.network_attached, si.device_contactable, si.ping_checked_at,
	si.installed_location, si.installed_height_m, si.signed_off_at,
	si.site_name, si.site_location, host(si.site_ip), si.site_subnet::text, host(si.site_gateway),
	si.deployment_date, si.deployment_team, si.team_leader, si.meta_data`

func scanInstallationWithNames(row pgx.Row) (*SiteInstallation, error) {
	var s SiteInstallation
	var metaRaw []byte
	err := row.Scan(&s.ID, &s.HardwareUnitID, &s.ReceivedOnSiteAt, &s.ConfirmedCorrect, &s.DiscrepancyNotes,
		&s.EscalatedToPMPCAt, &s.DateInstalled, &s.InstalledBy, &s.InstalledByName, &s.InstalledAt,
		&s.InspectedBy, &s.InspectedByName, &s.InspectedAt,
		&s.FitFocusBy, &s.FitFocusByName, &s.FitFocusCompletedAt, &s.NetworkAttached, &s.DeviceContactable, &s.PingCheckedAt,
		&s.InstalledLocation, &s.InstalledHeightM, &s.SignedOffAt,
		&s.SiteName, &s.SiteLocation, &s.SiteIP, &s.SiteSubnet, &s.SiteGateway,
		&s.DeploymentDate, &s.DeploymentTeam, &s.TeamLeader, &metaRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.MetaData, err = fromJSONB(metaRaw)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Service) GetInstallation(ctx context.Context, unitID uuid.UUID) (*SiteInstallation, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+selectColumnsWithNames+` FROM site_installations si
		LEFT JOIN users iu ON iu.id = si.installed_by
		LEFT JOIN users nu ON nu.id = si.inspected_by
		LEFT JOIN users fu ON fu.id = si.fit_focus_by
		WHERE si.hardware_unit_id = $1`, unitID)
	return scanInstallationWithNames(row)
}

// ---- Site receipt (confirm the shipped unit arrived correctly) ----

type RecordSiteReceiptInput struct {
	ConfirmedCorrect bool
	DiscrepancyNotes *string
}

func (s *Service) RecordSiteReceipt(ctx context.Context, unitID, actor uuid.UUID, in RecordSiteReceiptInput) (*SiteInstallation, error) {
	var escalatedAt *time.Time
	if !in.ConfirmedCorrect {
		now := time.Now()
		escalatedAt = &now
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO site_installations (hardware_unit_id, received_on_site_at, confirmed_correct, discrepancy_notes, escalated_to_pmpc_at)
		VALUES ($1, now(), $2, $3, $4)
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			received_on_site_at = now(),
			confirmed_correct = EXCLUDED.confirmed_correct,
			discrepancy_notes = EXCLUDED.discrepancy_notes,
			escalated_to_pmpc_at = EXCLUDED.escalated_to_pmpc_at,
			updated_at = now()
		RETURNING `+selectColumns,
		unitID, in.ConfirmedCorrect, in.DiscrepancyNotes, escalatedAt,
	)

	inst, err := scanInstallation(row)
	if err != nil {
		return nil, err
	}

	if err := s.hardware.SetException(ctx, unitID, !in.ConfirmedCorrect); err != nil {
		return nil, err
	}

	action := "site_receipt_confirmed"
	if !in.ConfirmedCorrect {
		action = "site_receipt_discrepancy"
	}
	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: action,
		PerformedBy: &actor, NewValue: inst,
	})

	return inst, nil
}

// ---- Installation core fields ----

type UpsertInstallationInput struct {
	InstalledLocation *string
	InstalledHeightM  *float64
	NetworkAttached   *bool
	DeviceContactable *bool
	SiteName          *string
	SiteLocation      *string
	SiteIP            *string
	SiteSubnet        *string
	SiteGateway       *string
	DeploymentDate    *time.Time
	DeploymentTeam    *string
	TeamLeader        *string
	MetaData          map[string]any
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Service) UpsertInstallation(ctx context.Context, unitID, actor uuid.UUID, in UpsertInstallationInput) (*SiteInstallation, error) {
	metaRaw, err := toJSONB(in.MetaData)
	if err != nil {
		return nil, err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO site_installations
			(hardware_unit_id, installed_location, installed_height_m, network_attached, device_contactable,
			 site_name, site_location, site_ip, site_subnet, site_gateway,
			 deployment_date, deployment_team, team_leader, meta_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8,'')::inet, NULLIF($9,'')::cidr, NULLIF($10,'')::inet,
		        $11, $12, $13, $14)
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			installed_location = EXCLUDED.installed_location,
			installed_height_m = EXCLUDED.installed_height_m,
			network_attached = EXCLUDED.network_attached,
			device_contactable = EXCLUDED.device_contactable,
			site_name = EXCLUDED.site_name,
			site_location = EXCLUDED.site_location,
			site_ip = EXCLUDED.site_ip,
			site_subnet = EXCLUDED.site_subnet,
			site_gateway = EXCLUDED.site_gateway,
			deployment_date = EXCLUDED.deployment_date,
			deployment_team = EXCLUDED.deployment_team,
			team_leader = EXCLUDED.team_leader,
			meta_data = EXCLUDED.meta_data,
			updated_at = now()
		RETURNING `+selectColumns,
		unitID, in.InstalledLocation, in.InstalledHeightM, in.NetworkAttached, in.DeviceContactable,
		in.SiteName, in.SiteLocation, derefStr(in.SiteIP), derefStr(in.SiteSubnet), derefStr(in.SiteGateway),
		in.DeploymentDate, in.DeploymentTeam, in.TeamLeader, metaRaw,
	)

	inst, err := scanInstallation(row)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "installation_update",
		PerformedBy: &actor, NewValue: inst,
	})
	return inst, nil
}

// CheckContactability attempts a TCP reachability check against the unit's
// assigned site IP as a practical stand-in for an ICMP ping — raw ICMP needs
// elevated privileges on both Windows and Linux, which the roadmap's
// cross-platform/no-cgo requirement rules out. A closed port still answers a
// SYN/RST, so this reliably detects "host is up" for typical network gear
// even when nothing is listening on the probed port.
func (s *Service) CheckContactability(ctx context.Context, unitID, actor uuid.UUID) (*SiteInstallation, error) {
	existing, err := s.GetInstallation(ctx, unitID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	contactable := false
	if existing != nil && existing.SiteIP != nil && *existing.SiteIP != "" {
		conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(*existing.SiteIP, "80"), 3*time.Second)
		if dialErr == nil {
			contactable = true
			_ = conn.Close()
		} else {
			var opErr *net.OpError
			if errors.As(dialErr, &opErr) && opErr.Timeout() {
				contactable = false
			} else {
				// A connection-refused error still means the host answered.
				contactable = true
			}
		}
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO site_installations (hardware_unit_id, device_contactable, ping_checked_at)
		VALUES ($1, $2, now())
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			device_contactable = EXCLUDED.device_contactable,
			ping_checked_at = now(),
			updated_at = now()
		RETURNING `+selectColumns,
		unitID, contactable,
	)

	inst, err := scanInstallation(row)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "installation_contactability_check",
		PerformedBy: &actor, NewValue: inst,
	})
	return inst, nil
}

// SignOffInstallation records the given step (installed|inspected|fit_focus).
// Once all three are recorded, the record is signed off and the unit's
// Kanban card moves from Installation to Commissioning.
func (s *Service) SignOffInstallation(ctx context.Context, unitID, actor uuid.UUID, step string, performedAt *time.Time) (*SiteInstallation, error) {
	var byColumn, atColumn string
	switch step {
	case "installed":
		byColumn, atColumn = "installed_by", "installed_at"
	case "inspected":
		byColumn, atColumn = "inspected_by", "inspected_at"
	case "fit_focus":
		byColumn, atColumn = "fit_focus_by", "fit_focus_completed_at"
	default:
		return nil, ErrInvalidSignOffStep
	}

	if step != "installed" {
		var photoCount int
		if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM installation_photos WHERE hardware_unit_id = $1`, unitID).Scan(&photoCount); err != nil {
			return nil, err
		}
		if photoCount == 0 {
			return nil, ErrPhotoRequired
		}
	}

	ts := time.Now()
	if performedAt != nil {
		ts = *performedAt
	}

	// The "installed" step also stamps date_installed — the date the unit
	// was physically installed on site is the same event as this sign-off,
	// and there's no separate data-entry point for it anywhere else.
	insertCols, insertVals, updateSet := byColumn, "$2", byColumn+" = $2"
	if step == "installed" {
		insertCols += ", " + atColumn + ", date_installed"
		insertVals += ", $3, $3"
		updateSet += ", " + atColumn + " = $3, date_installed = $3"
	} else {
		insertCols += ", " + atColumn
		insertVals += ", $3"
		updateSet += ", " + atColumn + " = $3"
	}
	query := fmt.Sprintf(`
		INSERT INTO site_installations (hardware_unit_id, %s)
		VALUES ($1, %s)
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			%s, updated_at = now()`,
		insertCols, insertVals, updateSet)
	if _, err := s.pool.Exec(ctx, query, unitID, actor, ts); err != nil {
		return nil, err
	}

	inst, err := s.GetInstallation(ctx, unitID)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "installation_signoff_" + step,
		PerformedBy: &actor, NewValue: inst,
	})

	if inst.InstalledBy != nil && inst.InspectedBy != nil && inst.FitFocusBy != nil && inst.SignedOffAt == nil {
		if _, err := s.pool.Exec(ctx, `UPDATE site_installations SET signed_off_at = $2 WHERE hardware_unit_id = $1`, unitID, ts); err != nil {
			return nil, err
		}
		inst.SignedOffAt = &ts

		if err := s.hardware.AdvanceBoardColumn(ctx, unitID, "commissioning", "commissioning"); err != nil {
			return nil, err
		}
		_ = s.audit.Record(ctx, audit.Entry{
			EntityType: "hardware_unit", EntityID: unitID.String(), Action: "installation_signed_off",
			PerformedBy: &actor, NewValue: inst,
		})
	}

	return inst, nil
}

// ---- Installation photos ----

func (s *Service) ListPhotos(ctx context.Context, unitID uuid.UUID) ([]InstallationPhoto, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, hardware_unit_id, uploaded_by, uploaded_at
		FROM installation_photos WHERE hardware_unit_id = $1 ORDER BY uploaded_at ASC`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := []InstallationPhoto{}
	for rows.Next() {
		var p InstallationPhoto
		if err := rows.Scan(&p.ID, &p.HardwareUnitID, &p.UploadedBy, &p.UploadedAt); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

func (s *Service) AddPhoto(ctx context.Context, unitID, actor uuid.UUID, filePath string) (*InstallationPhoto, error) {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM installation_photos WHERE hardware_unit_id = $1`, unitID).Scan(&count); err != nil {
		return nil, err
	}
	if count >= MaxInstallationPhotos {
		return nil, ErrMaxPhotos
	}

	var p InstallationPhoto
	row := s.pool.QueryRow(ctx, `
		INSERT INTO installation_photos (hardware_unit_id, file_path, uploaded_by)
		VALUES ($1, $2, $3)
		RETURNING id, hardware_unit_id, uploaded_by, uploaded_at`,
		unitID, filePath, actor)
	if err := row.Scan(&p.ID, &p.HardwareUnitID, &p.UploadedBy, &p.UploadedAt); err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "installation_photo_uploaded",
		PerformedBy: &actor, NewValue: p,
	})
	return &p, nil
}

func (s *Service) GetPhotoPath(ctx context.Context, unitID, photoID uuid.UUID) (string, error) {
	var path string
	err := s.pool.QueryRow(ctx, `SELECT file_path FROM installation_photos WHERE id = $1 AND hardware_unit_id = $2`, photoID, unitID).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return path, err
}

func (s *Service) DeletePhoto(ctx context.Context, unitID, photoID, actor uuid.UUID) (string, error) {
	var path string
	err := s.pool.QueryRow(ctx, `
		DELETE FROM installation_photos WHERE id = $1 AND hardware_unit_id = $2
		RETURNING file_path`, photoID, unitID).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "installation_photo_deleted",
		PerformedBy: &actor, NewValue: map[string]any{"photo_id": photoID},
	})
	return path, nil
}

package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

const summaryColumns = `hardware_unit_id, barcode, status, current_stage, board_column, is_exception,
	allocated_branch, created_at, device_make, device_model, serial_number, mac_address,
	device_config_signed_off_at, firmware_version, firmware_signed_off_at, site_name, site_location,
	deployment_team, shipped_via, waybill_number, shipment_delivered_at, installed_location,
	installed_height_m, installation_signed_off_at, acceptance_signed_off_at, defect_type,
	defect_replacement_status, defect_declared_date`

func buildWhere(f HardwareFilters) (string, []any) {
	var clauses []string
	var args []any

	add := func(clause string, val any) {
		args = append(args, val)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}

	if f.UnitID != nil && *f.UnitID != "" {
		add("hardware_unit_id = $%d::uuid", *f.UnitID)
	}
	if f.BoardColumn != nil && *f.BoardColumn != "" {
		add("board_column = $%d", *f.BoardColumn)
	}
	if f.Status != nil && *f.Status != "" {
		add("status = $%d", *f.Status)
	}
	if f.SiteName != nil && *f.SiteName != "" {
		args = append(args, *f.SiteName)
		siteArg := len(args)
		clauses = append(clauses, fmt.Sprintf("(site_name = $%d OR (site_name IS NULL AND allocated_branch = $%d))", siteArg, siteArg))
	}
	if f.DeviceModel != nil && *f.DeviceModel != "" {
		add("device_model = $%d", *f.DeviceModel)
	}
	if f.CreatedFrom != nil {
		add("created_at >= $%d", *f.CreatedFrom)
	}
	if f.CreatedTo != nil {
		add("created_at <= $%d", *f.CreatedTo)
	}
	if f.OnlyDefects {
		clauses = append(clauses, "defect_type IS NOT NULL")
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (s *Service) ListHardwareSummary(ctx context.Context, f HardwareFilters) ([]HardwareSummaryRow, error) {
	where, args := buildWhere(f)
	query := fmt.Sprintf(`SELECT %s FROM hardware_summary %s ORDER BY created_at DESC`, summaryColumns, where)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []HardwareSummaryRow{}
	for rows.Next() {
		var r HardwareSummaryRow
		if err := rows.Scan(&r.HardwareUnitID, &r.Barcode, &r.Status, &r.CurrentStage, &r.BoardColumn, &r.IsException,
			&r.AllocatedBranch, &r.CreatedAt, &r.DeviceMake, &r.DeviceModel, &r.SerialNumber, &r.MACAddress,
			&r.DeviceConfigSignedOffAt, &r.FirmwareVersion, &r.FirmwareSignedOffAt, &r.SiteName, &r.SiteLocation,
			&r.DeploymentTeam, &r.ShippedVia, &r.WaybillNumber, &r.ShipmentDeliveredAt, &r.InstalledLocation,
			&r.InstalledHeightM, &r.InstallationSignedOffAt, &r.AcceptanceSignedOffAt, &r.DefectType,
			&r.DefectReplacementStatus, &r.DefectDeclaredDate,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetUnitInfoSheet gathers everything known about a single unit — identity,
// device/network configuration, firmware, shipment, installation, and
// acceptance — for the printable "Hardware Information Sheet". Returns
// ErrNotFound if the unit itself doesn't exist.
func (s *Service) GetUnitInfoSheet(ctx context.Context, unitID string) (*UnitInfoSheet, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			hu.id::text, hu.barcode, hu.alias, hu.serial_number, hu.device_make, hu.device_model, hu.part_number,
			hu.status, hu.current_stage, hu.board_column, hu.is_exception, hu.allocated_branch, hu.created_at, hu.meta_data,

			dc.id IS NOT NULL, dc.device_make, dc.device_model, dc.serial_number,
			dc.device_name_dns, dc.client_ip_address::text, dc.mac_address::text,
			dc.dns_server_1::text, dc.dns_server_2::text, dc.ntp_server, dc.frequency_hz, dc.default_username,
			eu.full_name, dc.encoded_at, cu.full_name, dc.configured_at, qu.full_name, dc.qc_at,
			dc.signed_off_at, dc.meta_data,

			fc.id IS NOT NULL, fc.firmware_updated, fc.firmware_version, fc.signed_off_at,

			ld.docket_id IS NOT NULL, si.site_name, si.site_location, si.site_ip::text, si.site_subnet::text,
			si.site_gateway::text, si.deployment_team, si.team_leader, ld.shipped_via, ld.shipping_provider,
			ld.waybill_number, ld.received_at,

			si.id IS NOT NULL, si.installed_location, si.installed_height_m, si.date_installed,
			si.network_attached, si.device_contactable, si.signed_off_at,

			ca.id IS NOT NULL, ca.client_name, ca.client_signed_at, ca.client_signature_data,
			bu.full_name, ca.bsp_signed_by_name, ca.bsp_acceptance_date, ca.bsp_signature,
			ca.head_office_name, ca.head_office_signed_at, ca.head_office_signature_data,
			ca.comments, ca.signed_off_at,

			dr.id IS NOT NULL, dr.defect_type, dr.description, dr.replacement_status, dr.declared_date
		FROM hardware_units hu
		LEFT JOIN device_configurations dc ON dc.hardware_unit_id = hu.id
		LEFT JOIN users eu ON eu.id = dc.encoded_by
		LEFT JOIN users cu ON cu.id = dc.configured_by
		LEFT JOIN users qu ON qu.id = dc.qc_by
		LEFT JOIN firmware_configurations fc ON fc.hardware_unit_id = hu.id
		LEFT JOIN site_installations si ON si.hardware_unit_id = hu.id
		LEFT JOIN LATERAL (
			SELECT d.id AS docket_id, d.shipped_via, d.shipping_provider, d.waybill_number, d.received_at
			FROM delivery_dockets d
			JOIN delivery_docket_items i ON i.docket_id = d.id
			WHERE i.hardware_unit_id = hu.id
			ORDER BY d.created_at DESC
			LIMIT 1
		) ld ON true
		LEFT JOIN client_acceptances ca ON ca.hardware_unit_id = hu.id
		LEFT JOIN users bu ON bu.id = ca.bsp_acceptance_by
		LEFT JOIN defect_reports dr ON dr.hardware_unit_id = hu.id
		WHERE hu.id = $1`, unitID)

	var sheet UnitInfoSheet
	var metaRaw, unitMetaRaw []byte
	err := row.Scan(
		&sheet.ID, &sheet.Barcode, &sheet.Alias, &sheet.SerialNumber, &sheet.DeviceMake, &sheet.DeviceModel, &sheet.PartNumber,
		&sheet.Status, &sheet.CurrentStage, &sheet.BoardColumn, &sheet.IsException, &sheet.AllocatedBranch, &sheet.CreatedAt, &unitMetaRaw,

		&sheet.HasDeviceConfig, &sheet.ConfigDeviceMake, &sheet.ConfigDeviceModel, &sheet.ConfigSerialNumber,
		&sheet.DeviceNameDNS, &sheet.ClientIPAddress, &sheet.MACAddress,
		&sheet.DNSServer1, &sheet.DNSServer2, &sheet.NTPServer, &sheet.FrequencyHz, &sheet.DefaultUsername,
		&sheet.EncodedByName, &sheet.EncodedAt, &sheet.ConfiguredByName, &sheet.ConfiguredAt,
		&sheet.QCByName, &sheet.QCAt, &sheet.DeviceConfigSignedOffAt, &metaRaw,

		&sheet.HasFirmwareConfig, &sheet.FirmwareUpdated, &sheet.FirmwareVersion, &sheet.FirmwareSignedOffAt,

		&sheet.HasShipment, &sheet.SiteName, &sheet.SiteLocation, &sheet.SiteIP, &sheet.SiteSubnet,
		&sheet.SiteGateway, &sheet.DeploymentTeam, &sheet.TeamLeader, &sheet.ShippedVia, &sheet.ShippingProvider,
		&sheet.WaybillNumber, &sheet.ShipmentDeliveredAt,

		&sheet.HasInstallation, &sheet.InstalledLocation, &sheet.InstalledHeightM, &sheet.DateInstalled,
		&sheet.NetworkAttached, &sheet.DeviceContactable, &sheet.InstallationSignedOffAt,

		&sheet.HasAcceptance, &sheet.ClientName, &sheet.ClientSignedAt, &sheet.ClientSignatureData,
		&sheet.BSPAcceptanceBy, &sheet.BSPSignedByName, &sheet.BSPAcceptanceDate, &sheet.BSPSignature,
		&sheet.HeadOfficeName, &sheet.HeadOfficeSignedAt, &sheet.HeadOfficeSignatureData,
		&sheet.AcceptanceComments, &sheet.AcceptanceSignedOffAt,

		&sheet.HasDefect, &sheet.DefectType, &sheet.DefectDescription, &sheet.DefectReplacementStatus, &sheet.DefectDeclaredDate,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if sheet.HasDeviceConfig {
		sheet.AxisSettings, err = s.dynamicFieldsForStage(ctx, "stage1a", metaRaw)
		if err != nil {
			return nil, err
		}
		sheet.Accessories, err = accessoriesFromMeta(metaRaw)
		if err != nil {
			return nil, err
		}
	}

	sheet.Attributes, err = s.dynamicFieldsForStage(ctx, "general", unitMetaRaw)
	if err != nil {
		return nil, err
	}

	sheet.Photos, err = s.listUnitPhotos(ctx, unitID)
	if err != nil {
		return nil, err
	}

	return &sheet, nil
}

// listUnitPhotos returns the installation photos uploaded for a unit, oldest
// first, for the info sheet's "Uploaded Images" page.
func (s *Service) listUnitPhotos(ctx context.Context, unitID string) ([]UnitPhoto, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT file_path, uploaded_at FROM installation_photos
		WHERE hardware_unit_id = $1 ORDER BY uploaded_at ASC`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := []UnitPhoto{}
	for rows.Next() {
		var p UnitPhoto
		if err := rows.Scan(&p.FilePath, &p.UploadedAt); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// dynamicFieldsForStage pairs every active custom field definition for the
// given stage with its saved value from the matching meta_data JSONB blob,
// formatted for display — in definition order, so the PDF matches the order
// users see on the corresponding form (Stage 1-A's "Axis Settings", the
// unit-level "Attributes" tab, etc).
func (s *Service) dynamicFieldsForStage(ctx context.Context, stage string, metaRaw []byte) ([]AxisSettingField, error) {
	var meta map[string]any
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &meta); err != nil {
			return nil, err
		}
	}

	rows, err := s.pool.Query(ctx, `
		SELECT label, field_key, data_type
		FROM custom_field_definitions
		WHERE stage = $1 AND active
		ORDER BY sort_order`, stage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := []AxisSettingField{}
	for rows.Next() {
		var label, key, dataType string
		if err := rows.Scan(&label, &key, &dataType); err != nil {
			return nil, err
		}
		fields = append(fields, AxisSettingField{Label: label, Value: formatMetaValue(meta[key], dataType)})
	}
	return fields, rows.Err()
}

// accessoriesFromMeta pulls the fixed "Accessories" fields (mic/IO settings)
// out of Stage 1-A's meta_data blob — unlike AxisSettings, these aren't
// admin-defined custom_field_definitions, just fixed keys the frontend saves
// alongside them in the same JSONB column. Returns nil if none were ever set.
func accessoriesFromMeta(metaRaw []byte) (*AccessoriesInfo, error) {
	var meta map[string]any
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &meta); err != nil {
			return nil, err
		}
	}

	getStr := func(key string) *string {
		if v, ok := meta[key].(string); ok && v != "" {
			return &v
		}
		return nil
	}

	acc := AccessoriesInfo{
		Type:      getStr("accessories_type"),
		InputType: getStr("accessories_input_type"),
		Model:     getStr("accessories_model"),
		PowerType: getStr("accessories_power_type"),
	}
	if v, ok := meta["accessories_automatic_gain_control"].(bool); ok {
		acc.AutomaticGainControl = &v
	}

	if acc.Type == nil && acc.InputType == nil && acc.Model == nil && acc.PowerType == nil && acc.AutomaticGainControl == nil {
		return nil, nil
	}
	return &acc, nil
}

func formatMetaValue(v any, dataType string) string {
	if v == nil {
		return emptyPlaceholder
	}
	if dataType == "boolean" {
		if b, ok := v.(bool); ok {
			if b {
				return "Yes"
			}
			return "No"
		}
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return emptyPlaceholder
		}
		return s
	}
	return fmt.Sprintf("%v", v)
}

// ListPackingListForSite returns every hardware unit shipped to the given
// site, for a reprintable packing list report.
func (s *Service) ListPackingListForSite(ctx context.Context, siteName string) ([]PackingListEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT hu.barcode, dc.device_model, dc.serial_number, ld.waybill_number, ld.shipped_via, si.deployment_team
		FROM site_installations si
		JOIN hardware_units hu ON hu.id = si.hardware_unit_id
		LEFT JOIN device_configurations dc ON dc.hardware_unit_id = hu.id
		LEFT JOIN LATERAL (
			SELECT d.shipped_via, d.waybill_number
			FROM delivery_dockets d
			JOIN delivery_docket_items i ON i.docket_id = d.id
			WHERE i.hardware_unit_id = hu.id
			ORDER BY d.created_at DESC
			LIMIT 1
		) ld ON true
		WHERE si.site_name = $1
		ORDER BY hu.barcode`, siteName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []PackingListEntry{}
	for rows.Next() {
		var e PackingListEntry
		if err := rows.Scan(&e.Barcode, &e.DeviceModel, &e.SerialNumber, &e.WaybillNumber, &e.ShippedVia, &e.DeploymentTeam); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

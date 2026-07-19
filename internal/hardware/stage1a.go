package hardware

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"sbs-bsp-cctv/internal/audit"
)

var ErrInvalidSignOffStep = errors.New("invalid sign-off step")

type UpsertStage1AInput struct {
	DeviceMake      *string
	DeviceModel     *string
	DeviceNameDNS   *string
	ClientIPAddress *string
	SerialNumber    *string
	MACAddress      *string
	DNSServer1      *string
	DNSServer2      *string
	NTPServer       *string
	FrequencyHz     *int
	DefaultUsername *string
	DefaultPassword *string
	MetaData        map[string]any
}

func (s *Service) UpsertStage1A(ctx context.Context, unitID, actor uuid.UUID, in UpsertStage1AInput) (*Stage1A, error) {
	metaRaw, err := toJSONB(in.MetaData)
	if err != nil {
		return nil, err
	}

	if in.ClientIPAddress != nil && *in.ClientIPAddress != "" {
		conflict, err := s.clientIPInUseAtSameSite(ctx, unitID, *in.ClientIPAddress)
		if err != nil {
			return nil, err
		}
		if conflict {
			return nil, ErrClientIPInUse
		}
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO device_configurations
			(hardware_unit_id, device_make, device_model, device_name_dns, client_ip_address,
			 serial_number, mac_address, dns_server_1, dns_server_2, ntp_server, frequency_hz,
			 default_username, default_password, meta_data)
		VALUES ($1, $2, $3, $4, NULLIF($5,'')::inet, $6, NULLIF($7,'')::macaddr,
		        NULLIF($8,'')::inet, NULLIF($9,'')::inet, $10, $11, $12, $13, $14)
		ON CONFLICT (hardware_unit_id) DO UPDATE SET
			device_make = EXCLUDED.device_make,
			device_model = EXCLUDED.device_model,
			device_name_dns = EXCLUDED.device_name_dns,
			client_ip_address = EXCLUDED.client_ip_address,
			serial_number = EXCLUDED.serial_number,
			mac_address = EXCLUDED.mac_address,
			dns_server_1 = EXCLUDED.dns_server_1,
			dns_server_2 = EXCLUDED.dns_server_2,
			ntp_server = EXCLUDED.ntp_server,
			frequency_hz = EXCLUDED.frequency_hz,
			default_username = EXCLUDED.default_username,
			default_password = EXCLUDED.default_password,
			meta_data = EXCLUDED.meta_data,
			updated_at = now()`,
		unitID, in.DeviceMake, in.DeviceModel, in.DeviceNameDNS, derefStr(in.ClientIPAddress),
		in.SerialNumber, derefStr(in.MACAddress), derefStr(in.DNSServer1), derefStr(in.DNSServer2),
		in.NTPServer, in.FrequencyHz, in.DefaultUsername, in.DefaultPassword, metaRaw,
	)
	if err != nil {
		return nil, err
	}

	stage, err := s.GetStage1A(ctx, unitID)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "stage1a_update",
		PerformedBy: &actor, NewValue: stage,
	})
	return stage, nil
}

func scanStage1A(row pgx.Row) (*Stage1A, error) {
	var st Stage1A
	var metaRaw []byte
	err := row.Scan(&st.ID, &st.HardwareUnitID, &st.DeviceMake, &st.DeviceModel, &st.DeviceNameDNS,
		&st.ClientIPAddress, &st.SerialNumber, &st.MACAddress, &st.DNSServer1, &st.DNSServer2,
		&st.NTPServer, &st.FrequencyHz, &st.DefaultUsername, &st.DefaultPassword,
		&st.EncodedBy, &st.EncodedByName, &st.EncodedAt,
		&st.ConfiguredBy, &st.ConfiguredByName, &st.ConfiguredAt,
		&st.QCBy, &st.QCByName, &st.QCAt,
		&st.BarcodePrintedAt, &st.SignedOffAt, &metaRaw)
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

func (s *Service) GetStage1A(ctx context.Context, unitID uuid.UUID) (*Stage1A, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT dc.id, dc.hardware_unit_id, dc.device_make, dc.device_model, dc.device_name_dns,
		       host(dc.client_ip_address), dc.serial_number, dc.mac_address::text, host(dc.dns_server_1),
		       host(dc.dns_server_2), dc.ntp_server, dc.frequency_hz, dc.default_username, dc.default_password,
		       dc.encoded_by, eu.full_name, dc.encoded_at,
		       dc.configured_by, cu.full_name, dc.configured_at,
		       dc.qc_by, qu.full_name, dc.qc_at,
		       dc.barcode_printed_at, dc.signed_off_at, dc.meta_data
		FROM device_configurations dc
		LEFT JOIN users eu ON eu.id = dc.encoded_by
		LEFT JOIN users cu ON cu.id = dc.configured_by
		LEFT JOIN users qu ON qu.id = dc.qc_by
		WHERE dc.hardware_unit_id = $1`, unitID)
	return scanStage1A(row)
}

// clientIPInUseAtSameSite reports whether ip is already assigned to some
// other unit allocated to the same branch/site as unitID. Units without an
// allocated_branch are excluded from the check on both sides, since there's
// no site to scope the collision to.
func (s *Service) clientIPInUseAtSameSite(ctx context.Context, unitID uuid.UUID, ip string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM device_configurations dc
			JOIN hardware_units hu ON hu.id = dc.hardware_unit_id
			WHERE dc.hardware_unit_id != $1
			  AND dc.client_ip_address = $2::inet
			  AND hu.allocated_branch IS NOT NULL
			  AND hu.allocated_branch = (SELECT allocated_branch FROM hardware_units WHERE id = $1)
		)`, unitID, ip).Scan(&exists)
	return exists, err
}

// networkColumns whitelists the device_configurations network columns that
// NetworkDefaultsForSite may query — this keeps the column name out of any
// query built from caller input. raw is the bare column reference (used in
// WHERE/ORDER BY), text is the expression that reads it as text (inet
// columns need host(), ntp_server is already text).
var networkColumns = map[string]struct{ raw, text string }{
	"client_ip_address": {"dc.client_ip_address", "host(dc.client_ip_address)"},
	"dns_server_1":      {"dc.dns_server_1", "host(dc.dns_server_1)"},
	"dns_server_2":      {"dc.dns_server_2", "host(dc.dns_server_2)"},
	"ntp_server":        {"dc.ntp_server", "dc.ntp_server"},
}

// lastNetworkValueAtSite returns the most recently saved value of the given
// device_configurations network column among units allocated to the given
// branch/site.
func (s *Service) lastNetworkValueAtSite(ctx context.Context, allocatedBranch, column string) (string, error) {
	col, ok := networkColumns[column]
	if !ok {
		return "", fmt.Errorf("unsupported network column %q", column)
	}
	var value string
	err := s.pool.QueryRow(ctx, `
		SELECT `+col.text+`
		FROM device_configurations dc
		JOIN hardware_units hu ON hu.id = dc.hardware_unit_id
		WHERE hu.allocated_branch = $1 AND `+col.raw+` IS NOT NULL
		ORDER BY dc.updated_at DESC
		LIMIT 1`, allocatedBranch).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return value, err
}

// distinctNetworkValuesAtSite lists the distinct values already saved for the
// given network column among units allocated to the given branch/site, most
// recently used first — DNS/NTP servers tend to be shared infrastructure
// reused across many units at the same site, so this backs an autocomplete
// suggestion list (unlike client_ip_address, which is unique per unit and so
// has no suggestion list, only a last-used pre-fill).
func (s *Service) distinctNetworkValuesAtSite(ctx context.Context, allocatedBranch, column string) ([]string, error) {
	col, ok := networkColumns[column]
	if !ok {
		return nil, fmt.Errorf("unsupported network column %q", column)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (`+col.text+`) `+col.text+`
		FROM device_configurations dc
		JOIN hardware_units hu ON hu.id = dc.hardware_unit_id
		WHERE hu.allocated_branch = $1 AND `+col.raw+` IS NOT NULL
		ORDER BY `+col.text+`, dc.updated_at DESC`, allocatedBranch)
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

type NetworkDefaults struct {
	ClientIPAddress   string   `json:"client_ip_address"`
	DNSServer1        string   `json:"dns_server_1"`
	DNSServer2        string   `json:"dns_server_2"`
	NTPServer         string   `json:"ntp_server"`
	DNSServer1Options []string `json:"dns_server_1_options"`
	DNSServer2Options []string `json:"dns_server_2_options"`
	NTPServerOptions  []string `json:"ntp_server_options"`
}

// NetworkDefaultsForSite backs the New/blank Stage 1-A form's pre-fill and
// autocomplete: the last client IP used at the site (so the encoder can just
// bump the last octet), plus the last value and full suggestion list for
// each of the DNS/NTP fields, which are typically identical across every
// unit at a given site.
func (s *Service) NetworkDefaultsForSite(ctx context.Context, allocatedBranch string) (*NetworkDefaults, error) {
	var nd NetworkDefaults
	var err error

	if nd.ClientIPAddress, err = s.lastNetworkValueAtSite(ctx, allocatedBranch, "client_ip_address"); err != nil {
		return nil, err
	}
	if nd.DNSServer1, err = s.lastNetworkValueAtSite(ctx, allocatedBranch, "dns_server_1"); err != nil {
		return nil, err
	}
	if nd.DNSServer2, err = s.lastNetworkValueAtSite(ctx, allocatedBranch, "dns_server_2"); err != nil {
		return nil, err
	}
	if nd.NTPServer, err = s.lastNetworkValueAtSite(ctx, allocatedBranch, "ntp_server"); err != nil {
		return nil, err
	}
	if nd.DNSServer1Options, err = s.distinctNetworkValuesAtSite(ctx, allocatedBranch, "dns_server_1"); err != nil {
		return nil, err
	}
	if nd.DNSServer2Options, err = s.distinctNetworkValuesAtSite(ctx, allocatedBranch, "dns_server_2"); err != nil {
		return nil, err
	}
	if nd.NTPServerOptions, err = s.distinctNetworkValuesAtSite(ctx, allocatedBranch, "ntp_server"); err != nil {
		return nil, err
	}
	return &nd, nil
}

// SignOffStage1A records the given step (encoded|configured|qc) as performed
// by actor. As soon as encoding is done, the unit's Kanban card moves from
// Pre-Deployment to Configuration; the remaining two sign-offs are then
// completed while the unit sits in that column. Once all three are recorded,
// the record is marked fully signed off.
func (s *Service) SignOffStage1A(ctx context.Context, unitID, actor uuid.UUID, step string, performedAt *time.Time) (*Stage1A, error) {
	if step == "qc_fail" {
		if _, err := s.pool.Exec(ctx, `UPDATE device_configurations SET configured_by = NULL, configured_at = NULL, updated_at = now() WHERE hardware_unit_id = $1`, unitID); err != nil {
			return nil, err
		}
		st, err := s.GetStage1A(ctx, unitID)
		if err != nil {
			return nil, err
		}
		_ = s.audit.Record(ctx, audit.Entry{
			EntityType: "hardware_unit", EntityID: unitID.String(), Action: "stage1a_qc_failed",
			PerformedBy: &actor, NewValue: st,
		})
		return st, nil
	}

	var byColumn, atColumn string
	switch step {
	case "encoded":
		byColumn, atColumn = "encoded_by", "encoded_at"
	case "configured":
		byColumn, atColumn = "configured_by", "configured_at"
	case "qc":
		byColumn, atColumn = "qc_by", "qc_at"
	default:
		return nil, ErrInvalidSignOffStep
	}
	ts := time.Now()
	if performedAt != nil {
		ts = *performedAt
	}

	query := fmt.Sprintf(`UPDATE device_configurations SET %s = $2, %s = $3, updated_at = now() WHERE hardware_unit_id = $1`, byColumn, atColumn)
	if _, err := s.pool.Exec(ctx, query, unitID, actor, ts); err != nil {
		return nil, err
	}

	st, err := s.GetStage1A(ctx, unitID)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Record(ctx, audit.Entry{
		EntityType: "hardware_unit", EntityID: unitID.String(), Action: "stage1a_signoff_" + step,
		PerformedBy: &actor, NewValue: st,
	})

	if step == "encoded" {
		if err := s.AdvanceBoardColumn(ctx, unitID, "configuration", "device_configuration_qc"); err != nil {
			return nil, err
		}
	}

	if st.EncodedBy != nil && st.ConfiguredBy != nil && st.QCBy != nil && st.SignedOffAt == nil {
		if _, err := s.pool.Exec(ctx, `UPDATE device_configurations SET signed_off_at = $2 WHERE hardware_unit_id = $1`, unitID, ts); err != nil {
			return nil, err
		}
		st.SignedOffAt = &ts

		_ = s.audit.Record(ctx, audit.Entry{
			EntityType: "hardware_unit", EntityID: unitID.String(), Action: "stage1a_signed_off",
			PerformedBy: &actor, NewValue: st,
		})
	}

	return st, nil
}

func (s *Service) MarkBarcodePrinted(ctx context.Context, unitID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE device_configurations SET barcode_printed_at = now() WHERE hardware_unit_id = $1`, unitID)
	return err
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

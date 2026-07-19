package hardware

import (
	"time"

	"github.com/google/uuid"
)

type Unit struct {
	ID              uuid.UUID      `json:"id"`
	Barcode         string         `json:"barcode"`
	Alias           *string        `json:"alias,omitempty"`
	SerialNumber    *string        `json:"serial_number,omitempty"`
	DeviceMake      *string        `json:"device_make,omitempty"`
	DeviceModel     *string        `json:"device_model,omitempty"`
	PartNumber      *string        `json:"part_number,omitempty"`
	Status          string         `json:"status"`
	CurrentStage    string         `json:"current_stage"`
	BoardColumn     string         `json:"board_column"`
	IsException     bool           `json:"is_exception"`
	AllocatedBranch *string        `json:"allocated_branch,omitempty"`
	MetaData        map[string]any `json:"meta_data"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	// Encoded/Configured/QC reflect the Stage 1-A (Pre-Deployment Config &
	// QC) sign-off state. Populated on ListUnits only, for the board's
	// per-card sign-off indicators.
	Encoded    bool `json:"encoded"`
	Configured bool `json:"configured"`
	QC         bool `json:"qc"`
}

type Stage0Receiving struct {
	ID               uuid.UUID  `json:"id"`
	HardwareUnitID   uuid.UUID  `json:"hardware_unit_id"`
	ReceivedBy       *uuid.UUID `json:"received_by,omitempty"`
	ReceivedDate     time.Time  `json:"received_date"`
	PoOrWaybillRef   *string    `json:"po_or_waybill_ref,omitempty"`
	ItemsCorrect     *bool      `json:"items_correct,omitempty"`
	DiscrepancyNotes *string    `json:"discrepancy_notes,omitempty"`
	EscalatedTo      *string    `json:"escalated_to,omitempty"`
	EscalatedAt      *time.Time `json:"escalated_at,omitempty"`
}

type Stage1A struct {
	ID               uuid.UUID      `json:"id"`
	HardwareUnitID   uuid.UUID      `json:"hardware_unit_id"`
	DeviceMake       *string        `json:"device_make,omitempty"`
	DeviceModel      *string        `json:"device_model,omitempty"`
	DeviceNameDNS    *string        `json:"device_name_dns,omitempty"`
	ClientIPAddress  *string        `json:"client_ip_address,omitempty"`
	SerialNumber     *string        `json:"serial_number,omitempty"`
	MACAddress       *string        `json:"mac_address,omitempty"`
	DNSServer1       *string        `json:"dns_server_1,omitempty"`
	DNSServer2       *string        `json:"dns_server_2,omitempty"`
	NTPServer        *string        `json:"ntp_server,omitempty"`
	FrequencyHz      *int           `json:"frequency_hz,omitempty"`
	DefaultUsername  *string        `json:"default_username,omitempty"`
	DefaultPassword  *string        `json:"default_password,omitempty"`
	EncodedBy        *uuid.UUID     `json:"encoded_by,omitempty"`
	EncodedByName    *string        `json:"encoded_by_name,omitempty"`
	EncodedAt        *time.Time     `json:"encoded_at,omitempty"`
	ConfiguredBy     *uuid.UUID     `json:"configured_by,omitempty"`
	ConfiguredByName *string        `json:"configured_by_name,omitempty"`
	ConfiguredAt     *time.Time     `json:"configured_at,omitempty"`
	QCBy             *uuid.UUID     `json:"qc_by,omitempty"`
	QCByName         *string        `json:"qc_by_name,omitempty"`
	QCAt             *time.Time     `json:"qc_at,omitempty"`
	BarcodePrintedAt *time.Time     `json:"barcode_printed_at,omitempty"`
	SignedOffAt      *time.Time     `json:"signed_off_at,omitempty"`
	MetaData         map[string]any `json:"meta_data"`
}

type Stage1B struct {
	ID                    uuid.UUID      `json:"id"`
	HardwareUnitID        uuid.UUID      `json:"hardware_unit_id"`
	ConfiguredBy          *uuid.UUID     `json:"configured_by,omitempty"`
	ConfiguredByName      *string        `json:"configured_by_name,omitempty"`
	ConfiguredDate        *time.Time     `json:"configured_date,omitempty"`
	FirmwareUpdated       bool           `json:"firmware_updated"`
	FirmwareVersion       *string        `json:"firmware_version,omitempty"`
	ConfigurationQCBy     *uuid.UUID     `json:"configuration_qc_by,omitempty"`
	ConfigurationQCByName *string        `json:"configuration_qc_by_name,omitempty"`
	QCDate                *time.Time     `json:"qc_date,omitempty"`
	SignedOffAt           *time.Time     `json:"signed_off_at,omitempty"`
	MetaData              map[string]any `json:"meta_data"`
}

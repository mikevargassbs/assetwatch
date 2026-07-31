package reporting

import "time"

type HardwareSummaryRow struct {
	HardwareUnitID          string     `json:"hardware_unit_id"`
	Barcode                 string     `json:"barcode"`
	Status                  string     `json:"status"`
	CurrentStage            string     `json:"current_stage"`
	BoardColumn             string     `json:"board_column"`
	IsException             bool       `json:"is_exception"`
	AllocatedBranch         *string    `json:"allocated_branch,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	DeviceMake              *string    `json:"device_make,omitempty"`
	DeviceModel             *string    `json:"device_model,omitempty"`
	SerialNumber            *string    `json:"serial_number,omitempty"`
	MACAddress              *string    `json:"mac_address,omitempty"`
	DeviceConfigSignedOffAt *time.Time `json:"device_config_signed_off_at,omitempty"`
	FirmwareVersion         *string    `json:"firmware_version,omitempty"`
	FirmwareSignedOffAt     *time.Time `json:"firmware_signed_off_at,omitempty"`
	SiteName                *string    `json:"site_name,omitempty"`
	SiteLocation            *string    `json:"site_location,omitempty"`
	DeploymentTeam          *string    `json:"deployment_team,omitempty"`
	ShippedVia              *string    `json:"shipped_via,omitempty"`
	WaybillNumber           *string    `json:"waybill_number,omitempty"`
	ShipmentDeliveredAt     *time.Time `json:"shipment_delivered_at,omitempty"`
	InstalledLocation       *string    `json:"installed_location,omitempty"`
	InstalledHeightM        *float64   `json:"installed_height_m,omitempty"`
	InstallationSignedOffAt *time.Time `json:"installation_signed_off_at,omitempty"`
	AcceptanceSignedOffAt   *time.Time `json:"acceptance_signed_off_at,omitempty"`
	DefectType              *string    `json:"defect_type,omitempty"`
	DefectReplacementStatus *string    `json:"defect_replacement_status,omitempty"`
	DefectDeclaredDate      *time.Time `json:"defect_declared_date,omitempty"`
}

type HardwareFilters struct {
	UnitID      *string
	BoardColumn *string
	Status      *string
	SiteName    *string
	DeviceModel *string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	OnlyDefects bool
}

// UnitInfoSheet is the full single-unit "Hardware Information Sheet" —
// everything gathered about a unit across its lifecycle, joined in one
// query. Unlike HardwareSummaryRow (a condensed row for multi-unit list
// reports), sections here are only populated once the unit has reached
// that stage; the Has* flags let the PDF renderer skip sections for stages
// not yet reached instead of printing a page of dashes.
type UnitInfoSheet struct {
	ID              string
	Barcode         string
	Alias           *string
	SerialNumber    *string
	DeviceMake      *string
	DeviceModel     *string
	PartNumber      *string
	Status          string
	CurrentStage    string
	BoardColumn     string
	IsException     bool
	AllocatedBranch *string
	CreatedAt       time.Time

	HasDeviceConfig bool
	// ConfigDeviceMake/Model/SerialNumber come from the Stage 1-A device
	// configuration captured during encoding — used as a fallback in the
	// PDF when the unit-level identity fields (set at intake) were left
	// blank, since both describe the same physical device.
	ConfigDeviceMake        *string
	ConfigDeviceModel       *string
	ConfigSerialNumber      *string
	DeviceNameDNS           *string
	ClientIPAddress         *string
	MACAddress              *string
	DNSServer1              *string
	DNSServer2              *string
	NTPServer               *string
	FrequencyHz             *int
	DefaultUsername         *string
	EncodedByName           *string
	EncodedAt               *time.Time
	ConfiguredByName        *string
	ConfiguredAt            *time.Time
	QCByName                *string
	QCAt                    *time.Time
	DeviceConfigSignedOffAt *time.Time
	// AxisSettings lists the Stage 1-A dynamic ("Axis Settings") field
	// definitions with their saved value, pre-formatted for display —
	// only populated when HasDeviceConfig.
	AxisSettings []AxisSettingField
	// Accessories holds the fixed accessory fields (mic/IO settings) saved
	// alongside Axis Settings in the same Stage 1-A meta_data blob — nil if
	// none of those fields were ever filled in.
	Accessories *AccessoriesInfo

	HasFirmwareConfig   bool
	FirmwareUpdated     *bool
	FirmwareVersion     *string
	FirmwareSignedOffAt *time.Time

	HasShipment         bool
	SiteName            *string
	SiteLocation        *string
	SiteIP              *string
	SiteSubnet          *string
	SiteGateway         *string
	DeploymentTeam      *string
	TeamLeader          *string
	ShippedVia          *string
	ShippingProvider    *string
	WaybillNumber       *string
	ShipmentDeliveredAt *time.Time

	HasInstallation         bool
	InstalledLocation       *string
	InstalledHeightM        *float64
	DateInstalled           *time.Time
	NetworkAttached         *bool
	DeviceContactable       *bool
	InstallationSignedOffAt *time.Time

	HasAcceptance           bool
	ClientName              *string
	ClientSignedAt          *time.Time
	ClientSignatureData     *string
	BSPAcceptanceBy         *string
	BSPSignedByName         *string
	BSPAcceptanceDate       *time.Time
	BSPSignature            *string
	HeadOfficeName          *string
	HeadOfficeSignedAt      *time.Time
	HeadOfficeSignatureData *string
	AcceptanceComments      *string
	AcceptanceSignedOffAt   *time.Time

	HasDefect               bool
	DefectType              *string
	DefectDescription       *string
	DefectReplacementStatus *string
	DefectDeclaredDate      *time.Time

	// Attributes lists the unit-level ("general" stage) custom field
	// definitions with their saved value from hardware_units.meta_data —
	// what the "Attributes" tab on the unit detail page edits.
	Attributes []AxisSettingField

	// Photos are the installation photos uploaded as proof of installation,
	// printed on their own page(s) at the end of the sheet when present.
	Photos []UnitPhoto
}

// UnitPhoto is one uploaded installation photo, identified by its stored
// file path so the PDF renderer can read it straight off local disk.
type UnitPhoto struct {
	FilePath   string
	UploadedAt time.Time
}

// AxisSettingField is one Stage 1-A dynamic field definition paired with its
// saved value on a unit, already formatted for display in the info sheet.
type AxisSettingField struct {
	Label string
	Value string
}

// AccessoriesInfo is the fixed accessory (mic/IO) configuration captured on
// the Pre-Deployment Config & QC tab's "Accessories" section.
type AccessoriesInfo struct {
	Type                 *string
	InputType            *string
	Model                *string
	PowerType            *string
	AutomaticGainControl *bool
}

type PackingListEntry struct {
	Barcode        string  `json:"barcode"`
	DeviceModel    *string `json:"device_model,omitempty"`
	SerialNumber   *string `json:"serial_number,omitempty"`
	WaybillNumber  *string `json:"waybill_number,omitempty"`
	ShippedVia     *string `json:"shipped_via,omitempty"`
	DeploymentTeam *string `json:"deployment_team,omitempty"`
}

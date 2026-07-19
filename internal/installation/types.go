package installation

import (
	"time"

	"github.com/google/uuid"
)

type SiteInstallation struct {
	ID                  uuid.UUID      `json:"id"`
	HardwareUnitID      uuid.UUID      `json:"hardware_unit_id"`
	ReceivedOnSiteAt    *time.Time     `json:"received_on_site_at,omitempty"`
	ConfirmedCorrect    *bool          `json:"confirmed_correct,omitempty"`
	DiscrepancyNotes    *string        `json:"discrepancy_notes,omitempty"`
	EscalatedToPMPCAt   *time.Time     `json:"escalated_to_pmpc_at,omitempty"`
	DateInstalled       *time.Time     `json:"date_installed,omitempty"`
	InstalledBy         *uuid.UUID     `json:"installed_by,omitempty"`
	InstalledByName     *string        `json:"installed_by_name,omitempty"`
	InstalledAt         *time.Time     `json:"installed_at,omitempty"`
	InspectedBy         *uuid.UUID     `json:"inspected_by,omitempty"`
	InspectedByName     *string        `json:"inspected_by_name,omitempty"`
	InspectedAt         *time.Time     `json:"inspected_at,omitempty"`
	FitFocusBy          *uuid.UUID     `json:"fit_focus_by,omitempty"`
	FitFocusByName      *string        `json:"fit_focus_by_name,omitempty"`
	FitFocusCompletedAt *time.Time     `json:"fit_focus_completed_at,omitempty"`
	NetworkAttached     *bool          `json:"network_attached,omitempty"`
	DeviceContactable   *bool          `json:"device_contactable,omitempty"`
	PingCheckedAt       *time.Time     `json:"ping_checked_at,omitempty"`
	InstalledLocation   *string        `json:"installed_location,omitempty"`
	InstalledHeightM    *float64       `json:"installed_height_m,omitempty"`
	SignedOffAt         *time.Time     `json:"signed_off_at,omitempty"`
	SiteName            *string        `json:"site_name,omitempty"`
	SiteLocation        *string        `json:"site_location,omitempty"`
	SiteIP              *string        `json:"site_ip,omitempty"`
	SiteSubnet          *string        `json:"site_subnet,omitempty"`
	SiteGateway         *string        `json:"site_gateway,omitempty"`
	DeploymentDate      *time.Time     `json:"deployment_date,omitempty"`
	DeploymentTeam      *string        `json:"deployment_team,omitempty"`
	TeamLeader          *string        `json:"team_leader,omitempty"`
	MetaData            map[string]any `json:"meta_data"`
}

type InstallationPhoto struct {
	ID             uuid.UUID  `json:"id"`
	HardwareUnitID uuid.UUID  `json:"hardware_unit_id"`
	UploadedBy     *uuid.UUID `json:"uploaded_by,omitempty"`
	UploadedAt     time.Time  `json:"uploaded_at"`
}

const MaxInstallationPhotos = 3

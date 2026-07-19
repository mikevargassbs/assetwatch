package acceptance

import (
	"time"

	"github.com/google/uuid"
)

// ClientAcceptance covers two sequential sign-off stages after BSP
// acceptance: the Branch Manager signs first (the Client* fields, historic
// naming), then the Head Office / Security Manager signs second (the
// HeadOffice* fields). The unit is only fully signed off once both are
// present — see Service.finalizeIfComplete.
type ClientAcceptance struct {
	ID                               uuid.UUID      `json:"id"`
	HardwareUnitID                   uuid.UUID      `json:"hardware_unit_id"`
	Method                           *string        `json:"method,omitempty"`
	BSPAcceptanceBy                  *uuid.UUID     `json:"bsp_acceptance_by,omitempty"`
	BSPSignedByName                  *string        `json:"bsp_signed_by_name,omitempty"`
	BSPSignature                     *string        `json:"bsp_signature,omitempty"`
	BSPAcceptanceDate                *time.Time     `json:"bsp_acceptance_date,omitempty"`
	ClientSignatureLinkExpiresAt     *time.Time     `json:"client_signature_link_expires_at,omitempty"`
	ClientEmail                      *string        `json:"client_email,omitempty"`
	ClientLinkEmailedAt              *time.Time     `json:"client_link_emailed_at,omitempty"`
	ClientName                       *string        `json:"client_name,omitempty"`
	ClientSignedAt                   *time.Time     `json:"client_signed_at,omitempty"`
	ClientSignatureData              *string        `json:"client_signature_data,omitempty"`
	UploadedDocumentPath             *string        `json:"uploaded_document_path,omitempty"`
	HeadOfficeSignatureLinkExpiresAt *time.Time     `json:"head_office_signature_link_expires_at,omitempty"`
	HeadOfficeEmail                  *string        `json:"head_office_email,omitempty"`
	HeadOfficeLinkEmailedAt          *time.Time     `json:"head_office_link_emailed_at,omitempty"`
	HeadOfficeName                   *string        `json:"head_office_name,omitempty"`
	HeadOfficeSignedAt               *time.Time     `json:"head_office_signed_at,omitempty"`
	HeadOfficeSignatureData          *string        `json:"head_office_signature_data,omitempty"`
	Comments                         *string        `json:"comments,omitempty"`
	SignedOffAt                      *time.Time     `json:"signed_off_at,omitempty"`
	MetaData                         map[string]any `json:"meta_data"`
}

// PublicSigningInfo is what the unauthenticated client-facing signing page
// gets — never the full acceptance record (no internal IDs/user references).
// Stage identifies which of the two sequential signers this token is for.
type PublicSigningInfo struct {
	HardwareBarcode string `json:"hardware_barcode"`
	Stage           string `json:"stage"` // "branch_manager" | "head_office"
	AlreadySigned   bool   `json:"already_signed"`
	Expired         bool   `json:"expired"`
}

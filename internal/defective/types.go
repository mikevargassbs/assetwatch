package defective

import (
	"time"

	"github.com/google/uuid"
)

type DefectReport struct {
	ID                        uuid.UUID      `json:"id"`
	HardwareUnitID            uuid.UUID      `json:"hardware_unit_id"`
	DeclaredBy                *uuid.UUID     `json:"declared_by,omitempty"`
	DeclaredDate              time.Time      `json:"declared_date"`
	DefectType                string         `json:"defect_type"`
	Description               *string        `json:"description,omitempty"`
	ReportGeneratedAt         *time.Time     `json:"report_generated_at,omitempty"`
	EmailedToSupplierAt       *time.Time     `json:"emailed_to_supplier_at,omitempty"`
	SupplierEmail             *string        `json:"supplier_email,omitempty"`
	ReplacementStatus         string         `json:"replacement_status"`
	ReplacementHardwareUnitID *uuid.UUID     `json:"replacement_hardware_unit_id,omitempty"`
	TrackingNumber            *string        `json:"tracking_number,omitempty"`
	Carrier                   *string        `json:"carrier,omitempty"`
	ShippedBackAt             *time.Time     `json:"shipped_back_at,omitempty"`
	DeliveredAt               *time.Time     `json:"delivered_at,omitempty"`
	SupplierReceivedAt        *time.Time     `json:"supplier_received_at,omitempty"`
	MetaData                  map[string]any `json:"meta_data"`
}

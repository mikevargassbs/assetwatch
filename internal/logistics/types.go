package logistics

import (
	"time"

	"github.com/google/uuid"
)

type Docket struct {
	ID                uuid.UUID      `json:"id"`
	DocketNumber      string         `json:"docket_number"`
	WaybillNumber     *string        `json:"waybill_number,omitempty"`
	ShippedVia        *string        `json:"shipped_via,omitempty"`
	ShippingProvider  *string        `json:"shipping_provider,omitempty"`
	DestinationSite   *string        `json:"destination_site,omitempty"`
	Status            string         `json:"status"`
	DispatchedBy      *string        `json:"dispatched_by,omitempty"`
	DispatchedAt      *time.Time     `json:"dispatched_at,omitempty"`
	ReceivedBy        *string        `json:"received_by,omitempty"`
	ReceivedAt        *time.Time     `json:"received_at,omitempty"`
	Notes             *string        `json:"notes,omitempty"`
	MetaData          map[string]any `json:"meta_data"`
	ItemCount         int            `json:"item_count"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type DocketItem struct {
	ID                    uuid.UUID `json:"id"`
	DocketID              uuid.UUID `json:"docket_id"`
	HardwareUnitID        uuid.UUID `json:"hardware_unit_id"`
	Barcode               string    `json:"barcode"`
	SerialNumber          *string   `json:"serial_number,omitempty"`
	DeviceMake            *string   `json:"device_make,omitempty"`
	DeviceModel           *string   `json:"device_model,omitempty"`
	PartNumber            *string   `json:"part_number,omitempty"`
	SerialNumberSnapshot  *string   `json:"serial_number_snapshot,omitempty"`
	AddedAt               time.Time `json:"added_at"`
}

type TrackingEvent struct {
	ID                 uuid.UUID `json:"id"`
	DocketID           uuid.UUID `json:"docket_id"`
	Status             *string   `json:"status,omitempty"`
	Description        string    `json:"description"`
	OccurredAt         time.Time `json:"occurred_at"`
	IsSystemGenerated  bool      `json:"is_system_generated"`
	CreatedAt          time.Time `json:"created_at"`
}

type DocketDetail struct {
	Docket
	Items          []DocketItem    `json:"items"`
	TrackingEvents []TrackingEvent `json:"tracking_events"`
}

// UnitLogisticsView is the read-only summary shown on a hardware unit's
// Logistics tab: the most recent docket that carries this unit, plus its
// full tracking history.
type UnitLogisticsView struct {
	Docket         *Docket         `json:"docket"`
	TrackingEvents []TrackingEvent `json:"tracking_events"`
}

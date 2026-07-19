package logistics

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sbs-bsp-cctv/internal/auth"
	"sbs-bsp-cctv/internal/hardware"
)

type Handlers struct {
	service *Service
}

func NewHandlers(service *Service) *Handlers {
	return &Handlers{service: service}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

func actorFromRequest(r *http.Request) (uuid.UUID, bool) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	return claims.UserID, true
}

func unitIDFromPath(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

func docketIDFromPath(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

func itemIDFromPath(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "itemId"))
}

func errStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrDocketLocked), errors.Is(err, ErrItemDuplicate), errors.Is(err, ErrUnitNotReady):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// ---- Docket CRUD ----

type createDocketRequest struct {
	WaybillNumber    *string        `json:"waybill_number"`
	ShippedVia       *string        `json:"shipped_via"`
	ShippingProvider *string        `json:"shipping_provider"`
	DestinationSite  *string        `json:"destination_site"`
	Notes            *string        `json:"notes"`
	MetaData         map[string]any `json:"meta_data"`
}

func (h *Handlers) CreateDocket(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createDocketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.CreateDocket(r.Context(), actor, CreateDocketInput{
		WaybillNumber: req.WaybillNumber, ShippedVia: req.ShippedVia, ShippingProvider: req.ShippingProvider,
		DestinationSite: req.DestinationSite, Notes: req.Notes, MetaData: req.MetaData,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create docket")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handlers) ListDockets(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListDockets(r.Context(), ListDocketsFilter{Status: r.URL.Query().Get("status")})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list dockets")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) GetDocket(w http.ResponseWriter, r *http.Request) {
	id, err := docketIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := h.service.GetDocket(r.Context(), id)
	if err != nil {
		writeErr(w, errStatus(err), "failed to get docket")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---- Items ----

type addItemRequest struct {
	Identifier string `json:"identifier"`
}

func (h *Handlers) AddItem(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := docketIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Identifier == "" {
		writeErr(w, http.StatusBadRequest, "identifier is required")
		return
	}

	result, err := h.service.AddItem(r.Context(), id, actor, req.Identifier)
	if err != nil {
		if errors.Is(err, hardware.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "no unit found with that barcode or serial number")
			return
		}
		if errors.Is(err, ErrUnitNotReady) {
			writeErr(w, http.StatusConflict, "this unit hasn't reached the Shipment stage yet")
			return
		}
		writeErr(w, errStatus(err), "failed to add item")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handlers) RemoveItem(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	docketID, err := docketIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	itemID, err := itemIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid item id")
		return
	}

	if err := h.service.RemoveItem(r.Context(), docketID, itemID, actor); err != nil {
		writeErr(w, errStatus(err), "failed to remove item")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Sign-off ----

type dispatchRequest struct {
	DispatchedBy     string     `json:"dispatched_by"`
	DispatchedAt     *time.Time `json:"dispatched_at"`
	WaybillNumber    *string    `json:"waybill_number"`
	ShippedVia       *string    `json:"shipped_via"`
	ShippingProvider *string    `json:"shipping_provider"`
}

func (h *Handlers) Dispatch(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := docketIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req dispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DispatchedBy == "" {
		writeErr(w, http.StatusBadRequest, "dispatched_by is required")
		return
	}

	result, err := h.service.RecordDispatch(r.Context(), id, actor, RecordDispatchInput{
		DispatchedBy:     req.DispatchedBy,
		DispatchedAt:     req.DispatchedAt,
		WaybillNumber:    req.WaybillNumber,
		ShippedVia:       req.ShippedVia,
		ShippingProvider: req.ShippingProvider,
	})
	if err != nil {
		writeErr(w, errStatus(err), "failed to dispatch docket")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type receiveRequest struct {
	ReceivedBy string     `json:"received_by"`
	ReceivedAt *time.Time `json:"received_at"`
}

func (h *Handlers) Receive(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := docketIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req receiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ReceivedBy == "" {
		writeErr(w, http.StatusBadRequest, "received_by is required")
		return
	}

	result, err := h.service.RecordReceipt(r.Context(), id, actor, req.ReceivedBy, req.ReceivedAt)
	if err != nil {
		writeErr(w, errStatus(err), "failed to receive docket")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---- Tracking events ----

type addTrackingEventRequest struct {
	Status      *string    `json:"status"`
	Description string     `json:"description"`
	OccurredAt  *time.Time `json:"occurred_at"`
}

func (h *Handlers) AddTrackingEvent(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := docketIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req addTrackingEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		writeErr(w, http.StatusBadRequest, "description is required")
		return
	}

	// A status button click sends a fixed status with a default description;
	// anything without a status is a manual free-text entry.
	systemGenerated := req.Status != nil && *req.Status != ""

	result, err := h.service.AddTrackingEvent(r.Context(), id, actor, req.Status, req.Description, req.OccurredAt, systemGenerated)
	if err != nil {
		writeErr(w, errStatus(err), "failed to add tracking event")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// ---- Waybill PDF ----

func (h *Handlers) PrintWaybill(w http.ResponseWriter, r *http.Request) {
	id, err := docketIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	pdfBytes, err := h.service.GenerateDocketWaybill(r.Context(), id)
	if err != nil {
		writeErr(w, errStatus(err), "failed to generate waybill")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\"waybill.pdf\"")
	_, _ = w.Write(pdfBytes)
}

// ---- Per-unit read-only view ----

func (h *Handlers) GetUnitLogistics(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := h.service.GetUnitLogistics(r.Context(), id)
	if err != nil {
		writeErr(w, errStatus(err), "failed to get unit logistics")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

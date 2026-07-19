package hardware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sbs-bsp-cctv/internal/appsettings"
	"sbs-bsp-cctv/internal/auth"
	"sbs-bsp-cctv/internal/metadata"
)

type Handlers struct {
	service  *Service
	metadata *metadata.Service
}

func NewHandlers(service *Service, metadataSvc *metadata.Service) *Handlers {
	return &Handlers{service: service, metadata: metadataSvc}
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

// ---- units ----

type createUnitRequest struct {
	Alias           *string        `json:"alias"`
	SerialNumber    string         `json:"serial_number"`
	DeviceMake      *string        `json:"device_make"`
	DeviceModel     *string        `json:"device_model"`
	PartNumber      *string        `json:"part_number"`
	Barcode         *string        `json:"barcode"`
	AllocatedBranch *string        `json:"allocated_branch"`
	MetaData        map[string]any `json:"meta_data"`
}

func (h *Handlers) CreateUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createUnitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SerialNumber == "" {
		writeErr(w, http.StatusBadRequest, "serial_number is required")
		return
	}

	unit, err := h.service.CreateUnit(r.Context(), actor, CreateUnitInput{
		Alias: req.Alias, SerialNumber: req.SerialNumber,
		DeviceMake: req.DeviceMake, DeviceModel: req.DeviceModel,
		PartNumber: req.PartNumber, Barcode: req.Barcode,
		AllocatedBranch: req.AllocatedBranch, MetaData: req.MetaData,
	})
	if err != nil {
		if errors.Is(err, ErrSerialNumberInUse) {
			writeErr(w, http.StatusConflict, "serial number is already in use")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to create unit")
		return
	}
	writeJSON(w, http.StatusCreated, unit)
}

func (h *Handlers) ListUnits(w http.ResponseWriter, r *http.Request) {
	includeArchived := r.URL.Query().Get("include_archived") == "true"
	units, err := h.service.ListUnits(r.Context(), includeArchived)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list units")
		return
	}
	writeJSON(w, http.StatusOK, units)
}

// DeleteUnit is registered behind rbac.RequireRole(rbac.Admin). It retires
// the unit (soft delete) rather than removing it, preserving history.
func (h *Handlers) DeleteUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.service.DeleteUnit(r.Context(), actor, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete unit")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RestoreUnit is registered behind rbac.RequireRole(rbac.Admin). It reverses
// a retire, putting the unit back to active status.
func (h *Handlers) RestoreUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.service.RestoreUnit(r.Context(), actor, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to restore unit")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PurgeUnit is registered behind rbac.RequireRole(rbac.Admin). It permanently
// deletes a retired unit and all its related records. Irreversible.
func (h *Handlers) PurgeUnit(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.service.PurgeUnit(r.Context(), actor, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		if errors.Is(err, ErrNotRetired) {
			writeErr(w, http.StatusConflict, "unit must be retired before it can be permanently deleted")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to purge unit")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ListDeviceMakes(w http.ResponseWriter, r *http.Request) {
	values, err := h.service.DistinctDeviceMakes(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list device makes")
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (h *Handlers) ListDeviceModels(w http.ResponseWriter, r *http.Request) {
	values, err := h.service.DistinctDeviceModels(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list device models")
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (h *Handlers) GetUnit(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	unit, err := h.service.GetUnit(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to get unit")
		return
	}
	writeJSON(w, http.StatusOK, unit)
}

type moveBoardColumnRequest struct {
	Column          string `json:"column"`
	ResetAcceptance bool   `json:"reset_acceptance"`
}

// MoveBoardColumn is registered behind rbac.RequireRole(rbac.Admin). It lets
// an admin move a unit to any board column, including backward. Moving
// backward requires ResetAcceptance=true, confirming the unit's acceptance
// sign-off should be cleared (see Service.MoveBoardColumn).
func (h *Handlers) MoveBoardColumn(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req moveBoardColumnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Column == "" {
		writeErr(w, http.StatusBadRequest, "column is required")
		return
	}

	unit, err := h.service.MoveBoardColumn(r.Context(), actor, id, req.Column, req.ResetAcceptance)
	switch {
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, "not found")
		return
	case errors.Is(err, ErrInvalidBoardColumn):
		writeErr(w, http.StatusBadRequest, "invalid column")
		return
	case errors.Is(err, ErrBackwardMoveNotConfirmed):
		writeErr(w, http.StatusConflict, "moving backward requires confirming the acceptance sign-off will be cleared")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "failed to move unit")
		return
	}
	writeJSON(w, http.StatusOK, unit)
}

// SyncBoardColumn is open to any authenticated user. It re-checks whether
// the unit's current column's sign-offs are actually complete and advances
// board_column if so — the forward-drag counterpart to MoveBoardColumn,
// safe for any user since it can't move a unit past sign-offs that weren't
// already done by someone with the right role.
func (h *Handlers) SyncBoardColumn(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	unit, moved, err := h.service.SyncBoardColumn(r.Context(), actor, id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to sync unit")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unit": unit, "moved": moved})
}

type updateUnitMetaDataRequest struct {
	MetaData map[string]any `json:"meta_data"`
}

func (h *Handlers) UpdateUnitMetaData(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req updateUnitMetaDataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	unit, err := h.service.UpdateUnitMetaData(r.Context(), actor, id, req.MetaData)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to update attributes")
		return
	}
	writeJSON(w, http.StatusOK, unit)
}

type updateUnitIdentityRequest struct {
	Alias           *string `json:"alias"`
	SerialNumber    *string `json:"serial_number"`
	DeviceMake      *string `json:"device_make"`
	DeviceModel     *string `json:"device_model"`
	PartNumber      *string `json:"part_number"`
	AllocatedBranch *string `json:"allocated_branch"`
	Barcode         string  `json:"barcode"`
}

func (h *Handlers) UpdateUnitIdentity(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req updateUnitIdentityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Barcode == "" {
		writeErr(w, http.StatusBadRequest, "barcode is required")
		return
	}

	unit, err := h.service.UpdateUnitIdentity(r.Context(), actor, id, UpdateUnitIdentityInput{
		Alias: req.Alias, SerialNumber: req.SerialNumber,
		DeviceMake: req.DeviceMake, DeviceModel: req.DeviceModel, PartNumber: req.PartNumber,
		AllocatedBranch: req.AllocatedBranch, Barcode: req.Barcode,
	})
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if errors.Is(err, ErrSerialNumberInUse) {
		writeErr(w, http.StatusConflict, "serial number is already in use")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to update identity")
		return
	}
	writeJSON(w, http.StatusOK, unit)
}

// ---- stage 0: receiving ----

type recordReceivingRequest struct {
	PoOrWaybillRef   *string `json:"po_or_waybill_ref"`
	ItemsCorrect     bool    `json:"items_correct"`
	DiscrepancyNotes *string `json:"discrepancy_notes"`
}

func (h *Handlers) GetReceiving(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := h.service.GetReceiving(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to get receiving")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) RecordReceiving(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req recordReceivingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.RecordReceiving(r.Context(), id, RecordReceivingInput{
		ReceivedBy:       actor,
		PoOrWaybillRef:   req.PoOrWaybillRef,
		ItemsCorrect:     req.ItemsCorrect,
		DiscrepancyNotes: req.DiscrepancyNotes,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to record receiving")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---- stage 1-A ----

type upsertStage1ARequest struct {
	DeviceMake      *string        `json:"device_make"`
	DeviceModel     *string        `json:"device_model"`
	DeviceNameDNS   *string        `json:"device_name_dns"`
	ClientIPAddress *string        `json:"client_ip_address"`
	SerialNumber    *string        `json:"serial_number"`
	MACAddress      *string        `json:"mac_address"`
	DNSServer1      *string        `json:"dns_server_1"`
	DNSServer2      *string        `json:"dns_server_2"`
	NTPServer       *string        `json:"ntp_server"`
	FrequencyHz     *int           `json:"frequency_hz"`
	DefaultUsername *string        `json:"default_username"`
	DefaultPassword *string        `json:"default_password"`
	MetaData        map[string]any `json:"meta_data"`
}

func (h *Handlers) UpsertStage1A(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req upsertStage1ARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.UpsertStage1A(r.Context(), id, actor, UpsertStage1AInput{
		DeviceMake: req.DeviceMake, DeviceModel: req.DeviceModel, DeviceNameDNS: req.DeviceNameDNS,
		ClientIPAddress: req.ClientIPAddress, SerialNumber: req.SerialNumber, MACAddress: req.MACAddress,
		DNSServer1: req.DNSServer1, DNSServer2: req.DNSServer2, NTPServer: req.NTPServer,
		FrequencyHz: req.FrequencyHz, DefaultUsername: req.DefaultUsername, DefaultPassword: req.DefaultPassword,
		MetaData: req.MetaData,
	})
	if errors.Is(err, ErrClientIPInUse) {
		writeErr(w, http.StatusConflict, "client IP address is already in use at this site")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save stage 1-A configuration")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// NetworkDefaultsForSite returns pre-fill/autocomplete data for a Stage 1-A
// record's network fields, drawn from other units allocated to the given
// site: the last client IP used there (so the encoder can just bump the
// last octet) plus the last value and full suggestion list for each of the
// DNS/NTP fields, which are typically identical for every unit at a site.
func (h *Handlers) NetworkDefaultsForSite(w http.ResponseWriter, r *http.Request) {
	site := r.URL.Query().Get("site")
	if site == "" {
		writeJSON(w, http.StatusOK, &NetworkDefaults{})
		return
	}
	defaults, err := h.service.NetworkDefaultsForSite(r.Context(), site)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to look up network defaults")
		return
	}
	writeJSON(w, http.StatusOK, defaults)
}

func (h *Handlers) GetStage1A(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := h.service.GetStage1A(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to get stage 1-A configuration")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// stage1ARoleForStep maps a sign-off step to the role permitted to perform
// it (Admin is always allowed, checked separately).
var stage1ARoleForStep = map[string]string{
	"encoded":    "encoder",
	"configured": "configurator",
	"qc":         "qc",
	"qc_fail":    "qc",
}

type signOffRequest struct {
	Step        string     `json:"step"`
	PerformedAt *time.Time `json:"performed_at"`
}

func (h *Handlers) SignOffStage1A(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req signOffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	requiredRole, known := stage1ARoleForStep[req.Step]
	if !known {
		writeErr(w, http.StatusBadRequest, "invalid step")
		return
	}
	if !hasRole(claims.Roles, requiredRole) && !hasRole(claims.Roles, "admin") {
		writeErr(w, http.StatusForbidden, "role "+requiredRole+" required for this sign-off step")
		return
	}

	result, err := h.service.SignOffStage1A(r.Context(), id, claims.UserID, req.Step, req.PerformedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to sign off stage 1-A")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) PrintBarcodeSticker(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	pdfBytes, err := h.service.GenerateBarcodeSticker(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to generate barcode sticker")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\"barcode-sticker.pdf\"")
	_, _ = w.Write(pdfBytes)
}

// PreviewBarcodeLabel renders a sample sticker PDF at the requested (possibly
// unsaved) size, so the Admin settings page can preview a label size change
// before it's saved. Registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) PreviewBarcodeLabel(w http.ResponseWriter, r *http.Request) {
	var settings appsettings.BarcodeLabelSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := appsettings.ValidateBarcodeLabelSize(settings); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	pdfBytes, err := PreviewBarcodeSticker(settings)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to generate preview")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\"barcode-label-preview.pdf\"")
	_, _ = w.Write(pdfBytes)
}

// ListBarcodeLabelFields returns the fields selectable for the barcode
// sticker's text lines, so the Admin settings page can render checkboxes
// without duplicating this list on the frontend.
func (h *Handlers) ListBarcodeLabelFields(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, ListBarcodeLabelFields())
}

// ---- stage 1-B ----

type upsertStage1BRequest struct {
	FirmwareUpdated bool           `json:"firmware_updated"`
	FirmwareVersion *string        `json:"firmware_version"`
	MetaData        map[string]any `json:"meta_data"`
}

func (h *Handlers) UpsertStage1B(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req upsertStage1BRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.UpsertStage1B(r.Context(), id, actor, UpsertStage1BInput{
		FirmwareUpdated: req.FirmwareUpdated, FirmwareVersion: req.FirmwareVersion, MetaData: req.MetaData,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save stage 1-B configuration")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) GetStage1B(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := h.service.GetStage1B(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to get stage 1-B configuration")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

var stage1BRoleForStep = map[string]string{
	"configured": "configurator",
	"qc":         "qc",
	"qc_fail":    "qc",
}

func (h *Handlers) SignOffStage1B(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req signOffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	requiredRole, known := stage1BRoleForStep[req.Step]
	if !known {
		writeErr(w, http.StatusBadRequest, "invalid step")
		return
	}
	if !hasRole(claims.Roles, requiredRole) && !hasRole(claims.Roles, "admin") {
		writeErr(w, http.StatusForbidden, "role "+requiredRole+" required for this sign-off step")
		return
	}

	result, err := h.service.SignOffStage1B(r.Context(), id, claims.UserID, req.Step, req.PerformedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to sign off stage 1-B")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---- store custody ----

type storeCustodyRequest struct {
	Action  string  `json:"action"`
	Purpose string  `json:"purpose"`
	Notes   *string `json:"notes"`
}

func (h *Handlers) LogStoreCustody(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req storeCustodyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.LogStoreCustody(r.Context(), id, StoreCustodyInput{
		Action: req.Action, Purpose: req.Purpose, By: actor, Notes: req.Notes,
	}); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- meta-data field definitions ----

func (h *Handlers) ListMetaDataFields(w http.ResponseWriter, r *http.Request) {
	stage := r.URL.Query().Get("stage")
	if stage == "" {
		writeErr(w, http.StatusBadRequest, "stage query param is required")
		return
	}
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	defs, err := h.metadata.ListForStage(r.Context(), stage, includeInactive)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list field definitions")
		return
	}
	writeJSON(w, http.StatusOK, defs)
}

// DeactivateMetaDataField is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) DeactivateMetaDataField(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "fieldID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.metadata.Deactivate(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to deactivate field definition")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReactivateMetaDataField is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) ReactivateMetaDataField(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "fieldID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.metadata.Reactivate(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to reactivate field definition")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type updateMetaDataFieldRequest struct {
	Label      string `json:"label"`
	IsRequired bool   `json:"is_required"`
	SortOrder  int    `json:"sort_order"`
}

// UpdateMetaDataField is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) UpdateMetaDataField(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "fieldID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateMetaDataFieldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Label == "" {
		writeErr(w, http.StatusBadRequest, "label is required")
		return
	}
	def, err := h.metadata.Update(r.Context(), id, metadata.UpdateFieldDefinitionInput{
		Label: req.Label, IsRequired: req.IsRequired, SortOrder: req.SortOrder,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to update field definition")
		return
	}
	writeJSON(w, http.StatusOK, def)
}

type createMetaDataFieldRequest struct {
	Stage      string `json:"stage"`
	FieldKey   string `json:"field_key"`
	Label      string `json:"label"`
	DataType   string `json:"data_type"`
	IsRequired bool   `json:"is_required"`
	SortOrder  int    `json:"sort_order"`
}

// CreateMetaDataField is registered behind rbac.RequireRole(rbac.Admin) —
// only admins manage the dynamic field registry.
func (h *Handlers) CreateMetaDataField(w http.ResponseWriter, r *http.Request) {
	var req createMetaDataFieldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DataType == "" {
		req.DataType = "text"
	}

	def, err := h.metadata.Create(r.Context(), metadata.CreateFieldDefinitionInput{
		Stage: req.Stage, FieldKey: req.FieldKey, Label: req.Label,
		DataType: req.DataType, IsRequired: req.IsRequired, SortOrder: req.SortOrder,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create field definition")
		return
	}
	writeJSON(w, http.StatusCreated, def)
}

func hasRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

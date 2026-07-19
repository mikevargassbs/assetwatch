package defective

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sbs-bsp-cctv/internal/auth"
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

func (h *Handlers) GetDefectReport(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := h.service.GetDefectReport(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to get defect report")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type declareDefectRequest struct {
	DefectType  string  `json:"defect_type"`
	Description *string `json:"description"`
}

func (h *Handlers) DeclareDefect(w http.ResponseWriter, r *http.Request) {
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

	var req declareDefectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.DeclareDefect(r.Context(), id, actor, DeclareDefectInput{
		DefectType: req.DefectType, Description: req.Description,
	})
	if errors.Is(err, ErrInvalidDefectType) {
		writeErr(w, http.StatusBadRequest, "defect_type must be one of defective, damaged, wrong_item")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to declare defect")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) PrintReport(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	pdfBytes, err := h.service.GenerateReport(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "no defect declared for this unit")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to generate report")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\"defect-report.pdf\"")
	_, _ = w.Write(pdfBytes)
}

type emailSupplierRequest struct {
	SupplierEmail string `json:"supplier_email"`
}

func (h *Handlers) EmailToSupplier(w http.ResponseWriter, r *http.Request) {
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

	var req emailSupplierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SupplierEmail == "" {
		writeErr(w, http.StatusBadRequest, "supplier_email is required")
		return
	}

	result, sent, err := h.service.EmailToSupplier(r.Context(), id, actor, req.SupplierEmail)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to email supplier")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"acceptance": result, "sent": sent})
}

type markShippedBackRequest struct {
	TrackingNumber *string `json:"tracking_number"`
	Carrier        *string `json:"carrier"`
}

func (h *Handlers) MarkShippedBack(w http.ResponseWriter, r *http.Request) {
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

	var req markShippedBackRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	result, err := h.service.MarkShippedBack(r.Context(), id, actor, MarkShippedBackInput{
		TrackingNumber: req.TrackingNumber, Carrier: req.Carrier,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to mark shipped back")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) MarkDelivered(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.MarkDelivered(r.Context(), id, actor)
	if errors.Is(err, ErrNotShippedYet) {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to mark delivered")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) MarkSupplierReceived(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.MarkSupplierReceived(r.Context(), id, actor)
	if errors.Is(err, ErrNotShippedYet) {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to mark received by supplier")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type recordReplacementRequest struct {
	ReplacementSerialNumber string `json:"replacement_serial_number"`
}

func (h *Handlers) RecordReplacement(w http.ResponseWriter, r *http.Request) {
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

	var req recordReplacementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ReplacementSerialNumber == "" {
		writeErr(w, http.StatusBadRequest, "replacement_serial_number is required")
		return
	}

	defect, replacementUnit, err := h.service.RecordReplacement(r.Context(), id, actor, req.ReplacementSerialNumber)
	if errors.Is(err, ErrInvalidReplacement) {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to record replacement")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"defect_report": defect, "replacement_unit": replacementUnit})
}

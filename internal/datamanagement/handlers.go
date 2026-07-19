package datamanagement

import (
	"encoding/json"
	"net/http"
	"time"

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

// Export downloads every transaction and audit trail table as a single JSON
// file, for admins to archive before wiping the data.
func (h *Handlers) Export(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.Export(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to export transaction data")
		return
	}
	filename := "transaction-data-export-" + time.Now().Format("20060102-150405") + ".json"
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	writeJSON(w, http.StatusOK, data)
}

type wipeRequest struct {
	Confirm string `json:"confirm"`
}

// WipeAll permanently deletes every transaction and audit trail record. It
// requires the exact confirmation phrase in the request body, in addition to
// whatever confirmation the UI already collected, since this is irreversible.
func (h *Handlers) WipeAll(w http.ResponseWriter, r *http.Request) {
	actor, ok := actorFromRequest(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req wipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Confirm != WipeConfirmationPhrase {
		writeErr(w, http.StatusBadRequest, "confirmation phrase does not match")
		return
	}

	if err := h.service.WipeAll(r.Context(), actor, r.RemoteAddr); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to wipe transaction data")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

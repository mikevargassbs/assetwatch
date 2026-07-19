package appsettings

import (
	"encoding/json"
	"net/http"
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

func (h *Handlers) GetBarcodeLabelSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetBarcodeLabelSettings(r.Context())
	if err != nil {
		http.Error(w, "failed to load barcode label settings", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// UpdateBarcodeLabelSettings is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) UpdateBarcodeLabelSettings(w http.ResponseWriter, r *http.Request) {
	var in BarcodeLabelSettings
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	settings, err := h.service.UpdateBarcodeLabelSettings(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

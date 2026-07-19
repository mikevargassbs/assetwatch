package audit

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Handlers struct {
	service *Service
}

func NewHandlers(service *Service) *Handlers {
	return &Handlers{service: service}
}

// List is registered behind rbac.RequireRole(rbac.Admin). Supports optional
// entity_type / entity_id filters and limit/offset pagination.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	entries, err := h.service.List(r.Context(), ListFilter{
		EntityType: q.Get("entity_type"),
		EntityID:   q.Get("entity_id"),
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		http.Error(w, "failed to list audit trail", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}

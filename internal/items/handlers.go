package items

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
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

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "true"
	list, err := h.service.List(r.Context(), includeInactive)
	if err != nil {
		http.Error(w, "failed to list items", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type upsertRequest struct {
	Make             string  `json:"make"`
	Model            string  `json:"model"`
	Description      *string `json:"description"`
	Qty              int     `json:"qty"`
	SalesOrderNumber *string `json:"sales_order_number"`
}

// Create is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Make == "" || req.Model == "" {
		http.Error(w, "make and model are required", http.StatusBadRequest)
		return
	}

	item, err := h.service.Create(r.Context(), UpsertInput{
		Make: req.Make, Model: req.Model, Description: req.Description,
		Qty: req.Qty, SalesOrderNumber: req.SalesOrderNumber,
	})
	if err != nil {
		http.Error(w, "failed to create item", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Update is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Make == "" || req.Model == "" {
		http.Error(w, "make and model are required", http.StatusBadRequest)
		return
	}
	item, err := h.service.Update(r.Context(), id, UpsertInput{
		Make: req.Make, Model: req.Model, Description: req.Description,
		Qty: req.Qty, SalesOrderNumber: req.SalesOrderNumber,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update item", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// Deactivate is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.service.Deactivate(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to deactivate item", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reactivate is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) Reactivate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.service.Reactivate(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to reactivate item", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

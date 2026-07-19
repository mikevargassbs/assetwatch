package sitelocation

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
	locations, err := h.service.List(r.Context(), includeInactive)
	if err != nil {
		http.Error(w, "failed to list site locations", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, locations)
}

type createRequest struct {
	Name       string  `json:"name"`
	Region     *string `json:"region"`
	IPGateway  *string `json:"ip_gateway"`
	SubnetMask *string `json:"subnet_mask"`
}

// Create is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	location, err := h.service.Create(r.Context(), UpsertInput{
		Name: req.Name, Region: req.Region, IPGateway: req.IPGateway, SubnetMask: req.SubnetMask,
	})
	if err != nil {
		if errors.Is(err, ErrNameInUse) {
			http.Error(w, "that site name is already in use", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create site location", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, location)
}

// Update is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	location, err := h.service.Update(r.Context(), id, UpsertInput{
		Name: req.Name, Region: req.Region, IPGateway: req.IPGateway, SubnetMask: req.SubnetMask,
	})
	if err != nil {
		if errors.Is(err, ErrNameInUse) {
			http.Error(w, "that site name is already in use", http.StatusConflict)
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to update site location", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, location)
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
		http.Error(w, "failed to deactivate site location", http.StatusInternalServerError)
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
		http.Error(w, "failed to reactivate site location", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

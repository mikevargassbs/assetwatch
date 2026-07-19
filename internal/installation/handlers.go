package installation

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

func (h *Handlers) GetInstallation(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := h.service.GetInstallation(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to get installation")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type siteReceiptRequest struct {
	ConfirmedCorrect bool    `json:"confirmed_correct"`
	DiscrepancyNotes *string `json:"discrepancy_notes"`
}

func (h *Handlers) RecordSiteReceipt(w http.ResponseWriter, r *http.Request) {
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

	var req siteReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.RecordSiteReceipt(r.Context(), id, actor, RecordSiteReceiptInput{
		ConfirmedCorrect: req.ConfirmedCorrect, DiscrepancyNotes: req.DiscrepancyNotes,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to record site receipt")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type upsertInstallationRequest struct {
	InstalledLocation *string        `json:"installed_location"`
	InstalledHeightM  *float64       `json:"installed_height_m"`
	NetworkAttached   *bool          `json:"network_attached"`
	DeviceContactable *bool          `json:"device_contactable"`
	SiteName          *string        `json:"site_name"`
	SiteLocation      *string        `json:"site_location"`
	SiteIP            *string        `json:"site_ip"`
	SiteSubnet        *string        `json:"site_subnet"`
	SiteGateway       *string        `json:"site_gateway"`
	DeploymentDate    *time.Time     `json:"deployment_date"`
	DeploymentTeam    *string        `json:"deployment_team"`
	TeamLeader        *string        `json:"team_leader"`
	MetaData          map[string]any `json:"meta_data"`
}

func (h *Handlers) UpsertInstallation(w http.ResponseWriter, r *http.Request) {
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

	var req upsertInstallationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.UpsertInstallation(r.Context(), id, actor, UpsertInstallationInput{
		InstalledLocation: req.InstalledLocation, InstalledHeightM: req.InstalledHeightM,
		NetworkAttached: req.NetworkAttached, DeviceContactable: req.DeviceContactable,
		SiteName: req.SiteName, SiteLocation: req.SiteLocation, SiteIP: req.SiteIP,
		SiteSubnet: req.SiteSubnet, SiteGateway: req.SiteGateway, DeploymentDate: req.DeploymentDate,
		DeploymentTeam: req.DeploymentTeam, TeamLeader: req.TeamLeader, MetaData: req.MetaData,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save installation")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) CheckContactability(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.service.CheckContactability(r.Context(), id, actor)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to check contactability")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type signOffRequest struct {
	Step        string     `json:"step"`
	PerformedAt *time.Time `json:"performed_at"`
}

func (h *Handlers) SignOffInstallation(w http.ResponseWriter, r *http.Request) {
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

	var req signOffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.SignOffInstallation(r.Context(), id, actor, req.Step, req.PerformedAt)
	switch {
	case errors.Is(err, ErrInvalidSignOffStep):
		writeErr(w, http.StatusBadRequest, "invalid step")
		return
	case errors.Is(err, ErrPhotoRequired):
		writeErr(w, http.StatusConflict, "at least one installation photo is required before this step")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "failed to sign off installation")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

const maxPhotoUploadBytes = 20 << 20 // 20MB

func (h *Handlers) ListPhotos(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	photos, err := h.service.ListPhotos(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list photos")
		return
	}
	writeJSON(w, http.StatusOK, photos)
}

func (h *Handlers) UploadPhoto(w http.ResponseWriter, r *http.Request) {
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

	if err := r.ParseMultipartForm(maxPhotoUploadBytes); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart form or file too large")
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "photo file is required")
		return
	}
	defer file.Close()

	path, err := SaveUploadedPhoto(id, header.Filename, file)
	if errors.Is(err, ErrUnsupportedFileType) {
		writeErr(w, http.StatusBadRequest, "unsupported file type; only jpg, png, webp, or heic photos are allowed")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save photo")
		return
	}

	result, err := h.service.AddPhoto(r.Context(), id, actor, path)
	if errors.Is(err, ErrMaxPhotos) {
		writeErr(w, http.StatusConflict, "maximum of 3 photos already uploaded")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to record photo")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) DownloadPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	photoID, err := uuid.Parse(chi.URLParam(r, "photoId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid photo id")
		return
	}
	path, err := h.service.GetPhotoPath(r.Context(), id, photoID)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "photo not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to get photo")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(path)+"\"")
	http.ServeFile(w, r, path)
}

func (h *Handlers) DeletePhoto(w http.ResponseWriter, r *http.Request) {
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
	photoID, err := uuid.Parse(chi.URLParam(r, "photoId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid photo id")
		return
	}

	path, err := h.service.DeletePhoto(r.Context(), id, photoID, actor)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "photo not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete photo")
		return
	}
	_ = os.Remove(path)
	w.WriteHeader(http.StatusNoContent)
}

package acceptance

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"

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

// ---- Protected (Bearer auth) ----

func (h *Handlers) GetAcceptance(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := h.service.GetAcceptance(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to get acceptance record")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type bspSignOffRequest struct {
	SignedByName string         `json:"signed_by_name"`
	Signature    string         `json:"signature"`
	Comments     *string        `json:"comments"`
	MetaData     map[string]any `json:"meta_data"`
}

func (h *Handlers) RecordBSPAcceptance(w http.ResponseWriter, r *http.Request) {
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

	var req bspSignOffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.service.RecordBSPAcceptance(r.Context(), id, actor, BSPSignOffInput{
		SignedByName: req.SignedByName, Signature: req.Signature, Comments: req.Comments,
		MetaData: req.MetaData,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to record BSP acceptance")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) GenerateClientSigningLink(w http.ResponseWriter, r *http.Request) {
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

	token, err := h.service.GenerateClientSigningLink(r.Context(), id, actor)
	switch {
	case errors.Is(err, ErrAlreadyAccepted):
		writeErr(w, http.StatusConflict, "this unit has already been signed off")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "failed to generate signing link")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// requestBaseURL derives the public origin (scheme + host) the client's
// browser used to reach this server, so links embedded in emails resolve
// correctly whether accessed directly or through a TLS-terminating reverse
// proxy (which sets X-Forwarded-Proto; see DEPLOYMENT.md §5).
func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

type emailClientLinkRequest struct {
	ClientEmail string `json:"client_email"`
}

func (h *Handlers) EmailClientSigningLink(w http.ResponseWriter, r *http.Request) {
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

	var req emailClientLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClientEmail == "" {
		writeErr(w, http.StatusBadRequest, "client_email is required")
		return
	}

	acc, sent, err := h.service.EmailClientSigningLink(r.Context(), id, actor, req.ClientEmail, requestBaseURL(r))
	switch {
	case errors.Is(err, ErrAlreadyAccepted):
		writeErr(w, http.StatusConflict, "this unit has already been signed off")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "failed to email signing link")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"acceptance": acc, "sent": sent})
}

func (h *Handlers) GenerateHeadOfficeSigningLink(w http.ResponseWriter, r *http.Request) {
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

	token, err := h.service.GenerateHeadOfficeSigningLink(r.Context(), id, actor)
	switch {
	case errors.Is(err, ErrAlreadyAccepted):
		writeErr(w, http.StatusConflict, "this unit has already been signed off")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "failed to generate signing link")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *Handlers) EmailHeadOfficeSigningLink(w http.ResponseWriter, r *http.Request) {
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

	var req emailClientLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClientEmail == "" {
		writeErr(w, http.StatusBadRequest, "client_email is required")
		return
	}

	acc, sent, err := h.service.EmailHeadOfficeSigningLink(r.Context(), id, actor, req.ClientEmail, requestBaseURL(r))
	switch {
	case errors.Is(err, ErrAlreadyAccepted):
		writeErr(w, http.StatusConflict, "this unit has already been signed off")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "failed to email signing link")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"acceptance": acc, "sent": sent})
}

const maxUploadBytes = 20 << 20 // 20MB

func (h *Handlers) RecordManualUpload(w http.ResponseWriter, r *http.Request) {
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

	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid multipart form or file too large")
		return
	}
	file, header, err := r.FormFile("document")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "document file is required")
		return
	}
	defer file.Close()

	path, err := SaveUploadedDocument(id, header.Filename, file)
	if errors.Is(err, ErrUnsupportedFileType) {
		writeErr(w, http.StatusBadRequest, "unsupported file type; only pdf, jpg, or png documents are allowed")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to save document")
		return
	}

	var clientName *string
	if name := r.FormValue("client_name"); name != "" {
		clientName = &name
	}

	result, err := h.service.RecordManualUpload(r.Context(), id, actor, ManualUploadInput{
		ClientName: clientName, DocumentPath: path,
	})
	switch {
	case errors.Is(err, ErrAlreadyAccepted):
		writeErr(w, http.StatusConflict, "this unit has already been signed off")
		return
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "failed to record manual upload")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) DownloadDocument(w http.ResponseWriter, r *http.Request) {
	id, err := unitIDFromPath(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	acc, err := h.service.GetAcceptance(r.Context(), id)
	if errors.Is(err, ErrNotFound) || acc.UploadedDocumentPath == nil {
		writeErr(w, http.StatusNotFound, "no uploaded document for this unit")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to get acceptance record")
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(*acc.UploadedDocumentPath)+"\"")
	http.ServeFile(w, r, *acc.UploadedDocumentPath)
}

// ---- Public (token-authenticated, no login) ----

func (h *Handlers) GetPublicSigningInfo(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	info, err := h.service.GetPublicSigningInfo(r.Context(), token)
	if errors.Is(err, ErrLinkInvalid) {
		writeErr(w, http.StatusNotFound, "signing link is invalid or expired")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to load signing info")
		return
	}
	writeJSON(w, http.StatusOK, info)
}

type submitSignatureRequest struct {
	ClientName    string `json:"client_name"`
	SignatureData string `json:"signature_data"`
}

func (h *Handlers) SubmitClientSignature(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	var req submitSignatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ClientName == "" || req.SignatureData == "" {
		writeErr(w, http.StatusBadRequest, "client_name and signature_data are required")
		return
	}

	_, err := h.service.SubmitClientSignature(r.Context(), token, ClientSignInput{
		ClientName: req.ClientName, SignatureData: req.SignatureData,
	})
	switch {
	case errors.Is(err, ErrLinkInvalid):
		writeErr(w, http.StatusNotFound, "signing link is invalid or expired")
	case errors.Is(err, ErrAlreadyAccepted):
		writeErr(w, http.StatusConflict, "this unit has already been signed")
	case err != nil:
		writeErr(w, http.StatusInternalServerError, "failed to submit signature")
	default:
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	}
}

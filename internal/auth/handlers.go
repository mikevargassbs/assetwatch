package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"sbs-bsp-cctv/internal/audit"
)

type Handlers struct {
	service *Service
	audit   *audit.Service
}

func NewHandlers(service *Service, auditSvc *audit.Service) *Handlers {
	return &Handlers{service: service, audit: auditSvc}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	Email        string   `json:"email"`
	Roles        []string `json:"roles"`
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrDisallowedDomain) {
			_ = h.audit.Record(r.Context(), audit.Entry{
				EntityType: "user",
				EntityID:   req.Email,
				Action:     "login_failed",
				IPAddress:  r.RemoteAddr,
			})
			http.Error(w, "invalid email or password", http.StatusUnauthorized)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = h.audit.Record(r.Context(), audit.Entry{
		EntityType:  "user",
		EntityID:    result.UserID.String(),
		Action:      "login",
		PerformedBy: &result.UserID,
		IPAddress:   r.RemoteAddr,
	})

	writeJSON(w, http.StatusOK, authResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		Email:        result.Email,
		Roles:        result.Roles,
	})
}

// requestBaseURL derives the public origin (scheme + host) the client's
// browser used to reach this server, so links embedded in emails resolve
// correctly whether accessed directly or through a TLS-terminating reverse
// proxy (which sets X-Forwarded-Proto).
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

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// ForgotPassword always responds 204 regardless of whether the email
// belongs to a known account, so the endpoint can't be used to enumerate
// registered users.
func (h *Handlers) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	if err := h.service.RequestPasswordReset(r.Context(), req.Email, requestBaseURL(r)); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *Handlers) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || len(req.NewPassword) < 8 {
		http.Error(w, "token and a password of at least 8 characters are required", http.StatusBadRequest)
		return
	}

	if err := h.service.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		if errors.Is(err, ErrResetTokenInvalid) {
			http.Error(w, "reset link is invalid or has expired", http.StatusBadRequest)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		Email:        result.Email,
		Roles:        result.Roles,
	})
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.service.Logout(r.Context(), req.RefreshToken); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type createUserRequest struct {
	Email    string   `json:"email"`
	FullName string   `json:"full_name"`
	Password string   `json:"password"`
	Roles    []string `json:"roles"`
}

// CreateUser is registered behind rbac.RequireRole(rbac.Admin) — only
// admins can create accounts; there is no public self-registration route.
func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	userID, err := h.service.CreateUser(r.Context(), req.Email, req.FullName, req.Password, req.Roles)
	if err != nil {
		switch {
		case errors.Is(err, ErrDisallowedDomain):
			http.Error(w, "email domain is not allowed", http.StatusBadRequest)
		case errors.Is(err, ErrEmailInUse):
			http.Error(w, "email already in use", http.StatusConflict)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	actor, _ := ClaimsFromContext(r.Context())
	_ = h.audit.Record(r.Context(), audit.Entry{
		EntityType:  "user",
		EntityID:    userID.String(),
		Action:      "create",
		PerformedBy: &actor.UserID,
		NewValue:    req,
		IPAddress:   r.RemoteAddr,
	})

	writeJSON(w, http.StatusCreated, map[string]string{"id": userID.String()})
}

// ListUsers is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

type setUserRolesRequest struct {
	Roles []string `json:"roles"`
}

// SetUserRoles is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) SetUserRoles(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req setUserRolesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.SetUserRoles(r.Context(), userID, req.Roles); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	actor, _ := ClaimsFromContext(r.Context())
	_ = h.audit.Record(r.Context(), audit.Entry{
		EntityType:  "user",
		EntityID:    userID.String(),
		Action:      "set_roles",
		PerformedBy: &actor.UserID,
		NewValue:    req.Roles,
		IPAddress:   r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}

type updateUserRequest struct {
	FullName string `json:"full_name"`
	IsActive bool   `json:"is_active"`
}

// UpdateUser is registered behind rbac.RequireRole(rbac.Admin).
func (h *Handlers) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FullName == "" {
		http.Error(w, "full_name is required", http.StatusBadRequest)
		return
	}

	if err := h.service.UpdateUser(r.Context(), userID, req.FullName, req.IsActive); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	actor, _ := ClaimsFromContext(r.Context())
	_ = h.audit.Record(r.Context(), audit.Entry{
		EntityType:  "user",
		EntityID:    userID.String(),
		Action:      "update",
		PerformedBy: &actor.UserID,
		NewValue:    req,
		IPAddress:   r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}

type setUserPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// SetUserPassword is registered behind rbac.RequireRole(rbac.Admin) — lets an
// admin set a user's password directly, without the token-based reset flow.
func (h *Handlers) SetUserPassword(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req setUserPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.NewPassword) < 8 {
		http.Error(w, "a password of at least 8 characters is required", http.StatusBadRequest)
		return
	}

	if err := h.service.AdminSetPassword(r.Context(), userID, req.NewPassword); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	actor, _ := ClaimsFromContext(r.Context())
	_ = h.audit.Record(r.Context(), audit.Entry{
		EntityType:  "user",
		EntityID:    userID.String(),
		Action:      "set_password",
		PerformedBy: &actor.UserID,
		IPAddress:   r.RemoteAddr,
	})

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

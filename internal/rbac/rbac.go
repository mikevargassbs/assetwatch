package rbac

import (
	"net/http"

	"sbs-bsp-cctv/internal/auth"
)

const (
	Admin                = "admin"
	PMPC                 = "pm_pc"
	Encoder              = "encoder"
	Configurator         = "configurator"
	QC                   = "qc"
	Logistics            = "logistics"
	FieldTechnician      = "field_technician"
	BSPAcceptanceOfficer = "bsp_acceptance_officer"
	ReportsViewer        = "reports_viewer"
)

// RequireRole returns middleware that allows the request through only if the
// authenticated user (set by auth.Middleware) has at least one of the given
// roles. Admin always passes, regardless of the list.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			for _, role := range claims.Roles {
				if role == Admin {
					next.ServeHTTP(w, r)
					return
				}
				if _, hasRole := allowed[role]; hasRole {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}

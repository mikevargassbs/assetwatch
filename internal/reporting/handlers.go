package reporting

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
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

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parseDateParam(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func filtersFromQuery(r *http.Request) (HardwareFilters, error) {
	q := r.URL.Query()
	from, err := parseDateParam(q.Get("date_from"))
	if err != nil {
		return HardwareFilters{}, err
	}
	to, err := parseDateParam(q.Get("date_to"))
	if err != nil {
		return HardwareFilters{}, err
	}
	return HardwareFilters{
		UnitID:      strPtr(q.Get("unit_id")),
		BoardColumn: strPtr(q.Get("stage")),
		Status:      strPtr(q.Get("status")),
		SiteName:    strPtr(q.Get("site_name")),
		DeviceModel: strPtr(q.Get("device_model")),
		CreatedFrom: from,
		CreatedTo:   to,
	}, nil
}

// GetHardwareSummary serves the "Summary of Hardware" and, with a site_name
// filter, the "Summary of Hardware to a Site" report. format=csv|pdf
// switches to a downloadable export; anything else returns JSON for an
// on-screen table.
func (h *Handlers) GetHardwareSummary(w http.ResponseWriter, r *http.Request) {
	filters, err := filtersFromQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid date_from/date_to (expected YYYY-MM-DD)")
		return
	}

	rows, err := h.service.ListHardwareSummary(r.Context(), filters)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list hardware summary")
		return
	}

	switch r.URL.Query().Get("format") {
	case "csv":
		data, err := HardwareSummaryToCSV(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to render csv")
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=\"hardware-summary.csv\"")
		_, _ = w.Write(data)
	case "pdf":
		data, err := HardwareSummaryToPDF("Summary of Hardware", rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to render pdf")
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "inline; filename=\"hardware-summary.pdf\"")
		_, _ = w.Write(data)
	default:
		writeJSON(w, http.StatusOK, rows)
	}
}

// GetDefectsReport serves the "Defective/Damage/Wrong Item Report", reusing
// the same hardware summary pipeline filtered to units with a declared
// defect.
func (h *Handlers) GetDefectsReport(w http.ResponseWriter, r *http.Request) {
	filters, err := filtersFromQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid date_from/date_to (expected YYYY-MM-DD)")
		return
	}
	filters.OnlyDefects = true

	rows, err := h.service.ListHardwareSummary(r.Context(), filters)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list defects")
		return
	}

	switch r.URL.Query().Get("format") {
	case "csv":
		data, err := HardwareSummaryToCSV(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to render csv")
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=\"defect-report.csv\"")
		_, _ = w.Write(data)
	case "pdf":
		data, err := HardwareSummaryToPDF("Defective / Damage / Wrong Item Report", rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to render pdf")
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "inline; filename=\"defect-report.pdf\"")
		_, _ = w.Write(data)
	default:
		writeJSON(w, http.StatusOK, rows)
	}
}

// GetUnitInfoSheet serves the single-unit "Hardware Information Sheet" PDF
// — the printable report from a unit's detail page, covering identity,
// device/network configuration, and whatever lifecycle stages it has
// reached so far.
func (h *Handlers) GetUnitInfoSheet(w http.ResponseWriter, r *http.Request) {
	unitID := chi.URLParam(r, "id")

	sheet, err := h.service.GetUnitInfoSheet(r.Context(), unitID)
	if errors.Is(err, ErrNotFound) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to load unit info sheet")
		return
	}

	data, err := UnitInfoSheetToPDF(sheet)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to render pdf")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\"hardware-information-sheet.pdf\"")
	_, _ = w.Write(data)
}

// GetPackingListForSite serves the "Packing List Report to each site" —
// always a PDF, reprintable at any time.
func (h *Handlers) GetPackingListForSite(w http.ResponseWriter, r *http.Request) {
	siteName := r.URL.Query().Get("site_name")
	if siteName == "" {
		writeErr(w, http.StatusBadRequest, "site_name query param is required")
		return
	}

	entries, err := h.service.ListPackingListForSite(r.Context(), siteName)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list shipments for site")
		return
	}

	if r.URL.Query().Get("format") == "json" {
		writeJSON(w, http.StatusOK, entries)
		return
	}

	data, err := PackingListToPDF(siteName, entries)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to render packing list pdf")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=\"packing-list.pdf\"")
	_, _ = w.Write(data)
}

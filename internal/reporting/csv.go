package reporting

import (
	"bytes"
	"encoding/csv"
)

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// HardwareSummaryToCSV renders the given rows as CSV for spreadsheet import.
func HardwareSummaryToCSV(rows []HardwareSummaryRow) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	header := []string{
		"Barcode", "Status", "Current Stage", "Board Column", "Exception",
		"Allocated Branch", "Created At", "Device Make", "Device Model", "Serial Number",
		"MAC Address", "Site Name", "Site Location", "Deployment Team", "Waybill Number",
		"Defect Type", "Replacement Status",
	}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, r := range rows {
		exception := "no"
		if r.IsException {
			exception = "yes"
		}
		row := []string{
			r.Barcode, r.Status, r.CurrentStage, r.BoardColumn, exception,
			strOrEmpty(r.AllocatedBranch), r.CreatedAt.Format("2006-01-02 15:04"),
			strOrEmpty(r.DeviceMake), strOrEmpty(r.DeviceModel), strOrEmpty(r.SerialNumber),
			strOrEmpty(r.MACAddress), strOrEmpty(r.SiteName), strOrEmpty(r.SiteLocation),
			strOrEmpty(r.DeploymentTeam), strOrEmpty(r.WaybillNumber),
			strOrEmpty(r.DefectType), strOrEmpty(r.DefectReplacementStatus),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

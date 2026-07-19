package defective

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
)

// tagWidthMM/tagHeightMM size the report as a physical tag meant to travel
// inside the unit's box back to the supplier — a standard 4x6" shipping/
// waybill tag footprint — rather than an A4 office document.
const (
	tagWidthMM  = 101.6
	tagHeightMM = 152.4
)

// renderReportPDF renders the defect report as a printable waybill-style tag:
// a barcode graphic plus the unit's identifying and defect details, sized and
// styled to be printed, cut out, and attached to or placed inside the
// physical unit box when it's shipped back to the supplier.
func (s *Service) renderReportPDF(ctx context.Context, unitID uuid.UUID) ([]byte, error) {
	unit, err := s.hardware.GetUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	d, err := s.GetDefectReport(ctx, unitID)
	if err != nil {
		return nil, err
	}

	code, err := code128.Encode(unit.Barcode)
	if err != nil {
		return nil, fmt.Errorf("encode barcode: %w", err)
	}
	scaled, err := barcode.Scale(code, 300, 80)
	if err != nil {
		return nil, fmt.Errorf("scale barcode: %w", err)
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, scaled); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}

	// OrientationStr must be "P": fpdf swaps Wd/Ht whenever it's "L", which
	// would silently turn this into a landscape tag regardless of the Size
	// given. "P" uses Size as given.
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		Size:           fpdf.SizeType{Wd: tagWidthMM, Ht: tagHeightMM},
	})
	const margin = 5.0
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	const cornerR = 3.0
	const edge = 2.0

	pdf.SetDrawColor(90, 90, 90)
	pdf.SetLineWidth(0.4)
	pdf.RoundedRect(1.0, 1.0, tagWidthMM-2.0, tagHeightMM-2.0, cornerR, "1234", "D")

	headerH := 16.0
	headerW := tagWidthMM - 2*edge
	pdf.SetFillColor(153, 27, 27)
	pdf.RoundedRectExt(edge, edge, headerW, headerH, cornerR, cornerR, 0, 0, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetXY(edge, edge+2.5)
	pdf.CellFormat(headerW, 7, "RETURN TO SUPPLIER", "", 2, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 8.5)
	pdf.CellFormat(headerW, 5, "Defective / Damaged / Wrong Item", "", 0, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	bodyX := margin
	bodyW := tagWidthMM - 2*margin
	bodyY := edge + headerH + 3.0

	// Barcode graphic, preserving the encoder's fixed 300:80 aspect ratio so
	// the bars aren't stretched out of scanning proportion.
	const barcodeAspect = 300.0 / 80.0
	imgW := bodyW * 0.86
	imgH := imgW / barcodeAspect
	imgX := bodyX + (bodyW-imgW)/2
	imgOpts := fpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("barcode", imgOpts, &pngBuf)
	pdf.ImageOptions("barcode", imgX, bodyY, imgW, imgH, false, imgOpts, 0, "")

	humanY := bodyY + imgH + 1.5
	pdf.SetXY(bodyX, humanY)
	pdf.SetFont("Courier", "B", 11)
	pdf.CellFormat(bodyW, 5, unit.Barcode, "", 2, "C", false, 0, "")

	ruleY := humanY + 6.5
	pdf.SetDrawColor(190, 190, 190)
	pdf.SetLineWidth(0.15)
	pdf.Line(bodyX, ruleY, bodyX+bodyW, ruleY)

	type fieldRow struct{ label, value string }
	rows := []fieldRow{
		{"Barcode", unit.Barcode},
	}
	if unit.SerialNumber != nil && *unit.SerialNumber != "" {
		rows = append(rows, fieldRow{"Serial Number", *unit.SerialNumber})
	}
	if unit.PartNumber != nil && *unit.PartNumber != "" {
		rows = append(rows, fieldRow{"Part Number", *unit.PartNumber})
	}
	rows = append(rows,
		fieldRow{"Defect Type", strings.ToUpper(d.DefectType)},
		fieldRow{"Declared Date", d.DeclaredDate.Format("2006-01-02 15:04")},
	)
	if d.Carrier != nil && *d.Carrier != "" {
		rows = append(rows, fieldRow{"Carrier", *d.Carrier})
	}
	if d.TrackingNumber != nil && *d.TrackingNumber != "" {
		rows = append(rows, fieldRow{"Tracking Number", *d.TrackingNumber})
	}
	rows = append(rows, fieldRow{"Replacement Status", strings.ToUpper(strings.ReplaceAll(d.ReplacementStatus, "_", " "))})

	const labelFontSize = 7.5
	const valueFontSize = 8.5
	pdf.SetFont("Helvetica", "B", labelFontSize)
	labelColW := 0.0
	for _, row := range rows {
		if w := pdf.GetStringWidth(strings.ToUpper(row.label)); w > labelColW {
			labelColW = w
		}
	}
	labelColW += 2.0

	rowY := ruleY + 2.0
	const rowH = 6.5
	for _, row := range rows {
		pdf.SetXY(bodyX, rowY)
		pdf.SetFont("Helvetica", "B", labelFontSize)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(labelColW, rowH, strings.ToUpper(row.label), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", valueFontSize)
		pdf.SetTextColor(20, 20, 20)
		pdf.MultiCell(bodyW-labelColW, 4.5, row.value, "", "L", false)
		nextY := pdf.GetY()
		if nextY < rowY+rowH {
			nextY = rowY + rowH
		}
		pdf.SetDrawColor(230, 230, 230)
		pdf.SetLineWidth(0.1)
		pdf.Line(bodyX, nextY, bodyX+bodyW, nextY)
		rowY = nextY + 1.5
	}

	if d.Description != nil && *d.Description != "" {
		pdf.SetXY(bodyX, rowY+1)
		pdf.SetFont("Helvetica", "B", labelFontSize)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(0, 4.5, "DESCRIPTION", "", 2, "L", false, 0, "")
		pdf.SetX(bodyX)
		pdf.SetFont("Helvetica", "", valueFontSize)
		pdf.SetTextColor(20, 20, 20)
		pdf.MultiCell(bodyW, 4.5, *d.Description, "", "L", false)
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("render defect report pdf: %w", err)
	}
	return out.Bytes(), nil
}

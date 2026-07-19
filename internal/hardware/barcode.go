package hardware

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"image/png"
	"slices"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"

	"sbs-bsp-cctv/internal/appsettings"
)

const barcodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ" // no 0/O/1/I to avoid misreads

// generateBarcode produces a short, human-typeable code such as "SBS-7F3KQ2".
// It's independent of any user-entered identifier, since the barcode is a
// physical sticker value and must never collide across units.
func generateBarcode() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate barcode: %w", err)
	}
	code := make([]byte, len(buf))
	for i, b := range buf {
		code[i] = barcodeAlphabet[int(b)%len(barcodeAlphabet)]
	}
	return "SBS-" + string(code), nil
}

// barcodeLabelFieldOrder is the fixed, canonical display order for the
// optional text lines on a barcode sticker. It's the single source of truth
// for both what admins can select in the Admin settings page (via
// ListBarcodeLabelFields) and how selected fields are rendered.
//
// DefaultUsername/DefaultPassword are deliberately never offered here —
// printing device credentials on a sticker anyone can photograph is a
// security risk.
var barcodeLabelFieldOrder = []string{
	"serial_number", "mac", "ip", "model", "make",
	"barcode", "alias", "part_number", "device_name_dns", "allocated_branch",
}

var barcodeLabelFieldLabels = map[string]string{
	"serial_number":    "Serial Number",
	"mac":              "MAC",
	"ip":               "IP",
	"model":            "Model",
	"make":             "Make",
	"barcode":          "Barcode",
	"alias":            "Alias",
	"part_number":      "Part Number",
	"device_name_dns":  "Device Name",
	"allocated_branch": "Allocated Branch",
}

// BarcodeLabelField is the shape returned by ListBarcodeLabelFields so the
// Admin settings page can render checkboxes without duplicating the label
// list on the frontend.
type BarcodeLabelField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ListBarcodeLabelFields returns the selectable fields in canonical order.
func ListBarcodeLabelFields() []BarcodeLabelField {
	fields := make([]BarcodeLabelField, len(barcodeLabelFieldOrder))
	for i, key := range barcodeLabelFieldOrder {
		fields[i] = BarcodeLabelField{Key: key, Label: barcodeLabelFieldLabels[key]}
	}
	return fields
}

// barcodeLabelFieldValues maps each selectable field key to the unit's
// current value. Where a field exists on both Stage1A and Unit (model, make,
// serial number), the Stage1A value wins since it reflects the
// post-configuration value; it falls back to the Unit value if not yet set.
func barcodeLabelFieldValues(unit *Unit, stage1a *Stage1A) map[string]*string {
	pick := func(a, b *string) *string {
		if a != nil {
			return a
		}
		return b
	}
	return map[string]*string{
		"barcode":          &unit.Barcode,
		"mac":              stage1a.MACAddress,
		"ip":               stage1a.ClientIPAddress,
		"model":            pick(stage1a.DeviceModel, unit.DeviceModel),
		"make":             pick(stage1a.DeviceMake, unit.DeviceMake),
		"serial_number":    pick(stage1a.SerialNumber, unit.SerialNumber),
		"alias":            unit.Alias,
		"part_number":      unit.PartNumber,
		"device_name_dns":  stage1a.DeviceNameDNS,
		"allocated_branch": unit.AllocatedBranch,
	}
}

// previewBarcodeValue is the sample barcode graphic value shown in the Admin
// barcode-label settings preview — a real unit's data isn't available since
// the preview reflects settings that may not be saved yet.
const previewBarcodeValue = "SBS-PREVIEW"

// previewFieldValues gives every selectable field a placeholder value, so
// toggling any field in the Admin preview shows what it will look like.
func previewFieldValues() map[string]*string {
	sample := map[string]string{
		"barcode":          previewBarcodeValue,
		"mac":              "AA:BB:CC:DD:EE:FF",
		"ip":               "192.168.1.100",
		"model":            "Sample Camera Model",
		"make":             "Sample Make",
		"serial_number":    "SN-0000001",
		"alias":            "Sample Alias",
		"part_number":      "PN-12345",
		"device_name_dns":  "sample-device.local",
		"allocated_branch": "Sample Branch",
	}
	values := make(map[string]*string, len(sample))
	for k, v := range sample {
		v := v
		values[k] = &v
	}
	return values
}

// GenerateBarcodeSticker renders a printable PDF sticker for the unit's
// Stage 1-A record, and marks it as printed. The sticker's physical size and
// which fields it shows come from the admin-configurable barcode label
// setting.
func (s *Service) GenerateBarcodeSticker(ctx context.Context, unitID uuid.UUID) ([]byte, error) {
	unit, err := s.GetUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	stage1a, err := s.GetStage1A(ctx, unitID)
	if err != nil {
		return nil, err
	}
	settings, err := s.appSettings.GetBarcodeLabelSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load barcode label settings: %w", err)
	}

	out, err := renderBarcodeSticker(settings, unit.Barcode, barcodeLabelFieldValues(unit, stage1a))
	if err != nil {
		return nil, err
	}

	if err := s.MarkBarcodePrinted(ctx, unitID); err != nil {
		return nil, err
	}

	return out, nil
}

// PreviewBarcodeSticker renders the same sticker layout as
// GenerateBarcodeSticker, but with sample data and arbitrary (possibly
// unsaved) settings, so the Admin settings page can show what a sticker will
// actually look like before it's saved.
func PreviewBarcodeSticker(settings appsettings.BarcodeLabelSettings) ([]byte, error) {
	return renderBarcodeSticker(settings, previewBarcodeValue, previewFieldValues())
}

// renderBarcodeSticker draws the barcode graphic plus the selected
// identifying text fields onto a PDF sized to settings. The barcode graphic
// itself is always rendered regardless of field selection — only the text
// lines beneath it are configurable. fieldValues need not contain every key
// in barcodeLabelFieldOrder; missing or nil entries are simply skipped.
func renderBarcodeSticker(settings appsettings.BarcodeLabelSettings, barcodeGraphicValue string, fieldValues map[string]*string) ([]byte, error) {
	code, err := code128.Encode(barcodeGraphicValue)
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

	// OrientationStr must be "P" here: fpdf swaps Wd/Ht whenever it's "L",
	// which silently turns an e.g. 80x45mm wide label into a 45x80mm tall
	// page. "P" uses Size as given, regardless of which side is longer.
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		Size:           fpdf.SizeType{Wd: settings.WidthMM, Ht: settings.HeightMM},
	})

	const margin = 3.0
	// fpdf defaults to 1cm margins and auto page-break, both sized for
	// letter/A4 pages — on a small label those alone can exceed the page
	// height, silently spilling content onto extra pages. Override them so
	// our own margin/height math below is what actually governs the layout.
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()

	const cornerR = 1.6
	const edge = 1.0 // inset of the header bar / body from the outer border

	// A thin rounded frame around the whole label reads as a deliberately
	// designed asset tag rather than loose content on a page.
	pdf.SetDrawColor(90, 90, 90)
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(0.6, 0.6, settings.WidthMM-1.2, settings.HeightMM-1.2, cornerR, "1234", "D")

	// A solid branded header bar reads as a designed asset tag rather than a
	// plain printout, even without a logo asset to draw.
	headerH := settings.HeightMM * 0.24
	headerW := settings.WidthMM - 2*edge
	pdf.SetFillColor(30, 41, 59)
	pdf.RoundedRectExt(edge, edge, headerW, headerH, cornerR, cornerR, 0, 0, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetXY(edge, edge)
	pdf.CellFormat(headerW, headerH, "SBS-BSP CCTV HARDWARE", "", 1, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	bodyX := margin
	bodyW := settings.WidthMM - 2*margin
	bodyY := edge + headerH + 1.5
	bodyBottom := settings.HeightMM - edge
	bodyH := bodyBottom - bodyY

	// The barcode is prominent but not edge-to-edge — a visible quiet zone
	// around it, centered, is what separates a designed tag from a printout
	// with an image slapped on it.
	barcodeH := bodyH * 0.36

	// boombuler always rasterizes onto a fixed 300x80px canvas, so drawing it
	// at arbitrary dimensions verbatim stretches the bars out of proportion.
	// Preserve that 300:80 aspect ratio and center the result.
	const barcodeAspect = 300.0 / 80.0
	maxImgW := bodyW * 0.82
	imgW, imgH := maxImgW, maxImgW/barcodeAspect
	if imgH > barcodeH {
		imgH = barcodeH
		imgW = imgH * barcodeAspect
	}
	imgX := bodyX + (bodyW-imgW)/2
	imgY := bodyY + (barcodeH-imgH)/2

	imgOpts := fpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("barcode", imgOpts, &pngBuf)
	pdf.ImageOptions("barcode", imgX, imgY, imgW, imgH, false, imgOpts, 0, "")

	// A rule under the barcode separates it from the data grid, like a
	// letterhead divider.
	ruleY := bodyY + barcodeH
	pdf.SetDrawColor(190, 190, 190)
	pdf.SetLineWidth(0.15)
	pdf.Line(bodyX, ruleY, bodyX+bodyW, ruleY)

	type fieldRow struct{ label, value string }
	var rows []fieldRow
	for _, key := range barcodeLabelFieldOrder {
		if !slices.Contains(settings.Fields, key) {
			continue
		}
		val := fieldValues[key]
		if val == nil || *val == "" {
			continue
		}
		rows = append(rows, fieldRow{strings.ToUpper(barcodeLabelFieldLabels[key]), *val})
	}

	if len(rows) > 0 {
		gridY := ruleY + 1.2
		gridH := bodyBottom - gridY
		rowH := gridH / float64(len(rows))
		if rowH > 6.5 {
			rowH = 6.5
		}
		if rowH < 3.2 {
			rowH = 3.2
		}

		// A label column sized to the longest selected label — not a flat
		// percentage — keeps every value column starting at the same x
		// without clipping longer labels (e.g. "ALLOCATED BRANCH") into the
		// value with no gap.
		const labelFontSize = 6.5
		pdf.SetFont("Helvetica", "B", labelFontSize)
		labelColW := 0.0
		for _, row := range rows {
			if w := pdf.GetStringWidth(row.label); w > labelColW {
				labelColW = w
			}
		}
		labelColW += 2.5
		if maxLabelColW := bodyW * 0.5; labelColW > maxLabelColW {
			labelColW = maxLabelColW
		}

		for i, row := range rows {
			y := gridY + float64(i)*rowH
			pdf.SetXY(bodyX, y)
			pdf.SetFont("Helvetica", "B", labelFontSize)
			pdf.SetTextColor(90, 90, 90)
			pdf.CellFormat(labelColW, rowH, row.label, "", 0, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 7.5)
			pdf.SetTextColor(20, 20, 20)
			pdf.CellFormat(bodyW-labelColW, rowH, row.value, "", 2, "L", false, 0, "")

			// A faint hairline under each row (except the last) gives the
			// grid a tabular, data-sheet structure instead of loose lines.
			if i < len(rows)-1 {
				lineY := y + rowH
				pdf.SetDrawColor(230, 230, 230)
				pdf.SetLineWidth(0.1)
				pdf.Line(bodyX, lineY, bodyX+bodyW, lineY)
			}
		}
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}

	return out.Bytes(), nil
}

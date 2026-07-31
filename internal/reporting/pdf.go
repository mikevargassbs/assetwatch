package reporting

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"

	"sbs-bsp-cctv/internal/pdfbrand"
)

// HardwareSummaryToPDF renders a condensed tabular report — enough columns
// to be useful on a printed page without wrapping to landscape spreads.
func HardwareSummaryToPDF(title string, rows []HardwareSummaryRow) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	if err := pdfbrand.DrawHeader(pdf); err != nil {
		return nil, err
	}
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, title, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.Ln(2)

	headers := []string{"Barcode", "Status", "Stage", "Site", "Device Model", "Waybill", "Defect"}
	widths := []float64{35.0, 25.0, 35.0, 45.0, 45.0, 35.0, 35.0}

	pdf.SetFont("Helvetica", "B", 9)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "L", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 8)
	for _, r := range rows {
		pdf.CellFormat(widths[0], 6, r.Barcode, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 6, r.Status, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 6, r.BoardColumn, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 6, strOrEmpty(r.SiteName), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[4], 6, strOrEmpty(r.DeviceModel), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[5], 6, strOrEmpty(r.WaybillNumber), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[6], 6, strOrEmpty(r.DefectType), "1", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return out.Bytes(), nil
}

// The core Helvetica/Courier fonts fpdf uses here are cp1252, not UTF-8 — a
// multi-byte character like an em dash renders as garbled bytes ("â€"") when
// passed straight through, so placeholders stick to plain ASCII.
const emptyPlaceholder = "-"

func dash(s *string) string {
	if s == nil || *s == "" {
		return emptyPlaceholder
	}
	return *s
}

// coalesceStr returns the first non-nil, non-empty of the two.
func coalesceStr(primary, fallback *string) *string {
	if primary != nil && *primary != "" {
		return primary
	}
	return fallback
}

func dashTime(t *time.Time) string {
	if t == nil {
		return emptyPlaceholder
	}
	return t.Format("2006-01-02 15:04")
}

// dashBy formats a "who + when" sign-off pair, e.g. "Jane Doe — 2026-07-15 09:30".
func dashBy(name *string, at *time.Time) string {
	if at == nil {
		return emptyPlaceholder
	}
	if name == nil || *name == "" {
		return dashTime(at)
	}
	return *name + " - " + dashTime(at)
}

func dashBool(b *bool) string {
	if b == nil {
		return emptyPlaceholder
	}
	if *b {
		return "Yes"
	}
	return "No"
}

func dashInt(i *int) string {
	if i == nil {
		return emptyPlaceholder
	}
	return strconv.Itoa(*i)
}

func dashFloat(f *float64) string {
	if f == nil {
		return emptyPlaceholder
	}
	return strconv.FormatFloat(*f, 'f', 2, 64)
}

func humanize(s string) string {
	return strings.ReplaceAll(s, "_", " ")
}

// drawSignatureImage decodes a "data:image/png;base64,..." data URL (as
// produced by the SignaturePad canvas) and draws it at (x, y), scaled down
// to fit within maxW x maxH while preserving aspect ratio. Returns the
// drawn height, or 0 if the value isn't a usable data URL — e.g. the BSP
// signature field can hold a plain typed name instead of a drawn signature
// when the pad was left blank, in which case nothing is drawn here and the
// caller falls back to printing the text.
func drawSignatureImage(pdf *fpdf.Fpdf, imgName, dataURL string, x, y, maxW, maxH float64) float64 {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		return 0
	}
	raw, err := base64.StdEncoding.DecodeString(dataURL[len(prefix):])
	if err != nil {
		return 0
	}
	imgOpts := fpdf.ImageOptions{ImageType: "PNG"}
	info := pdf.RegisterImageOptionsReader(imgName, imgOpts, bytes.NewReader(raw))
	if info == nil {
		return 0
	}
	w, h := info.Extent()
	if w == 0 || h == 0 {
		return 0
	}
	scale := maxW / w
	if h*scale > maxH {
		scale = maxH / h
	}
	drawW, drawH := w*scale, h*scale
	pdf.ImageOptions(imgName, x, y, drawW, drawH, false, imgOpts, 0, "")
	return drawH
}

// UnitInfoSheetToPDF renders the single-unit "Hardware Information Sheet" —
// a sectioned label/value layout covering identity and device/network
// configuration, plus shipment, installation, acceptance, and defect
// sections once the unit has actually reached those stages. Sections for
// stages not yet reached are omitted rather than printed full of dashes.
// Fields are packed two-per-line in a compact grid (rather than one per
// line) so the whole sheet fits on a single page even once every stage has
// been filled in.
func UnitInfoSheetToPDF(sheet *UnitInfoSheet) ([]byte, error) {
	const margin = 12.0
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	if err := pdfbrand.DrawHeader(pdf, "Hardware Information Sheet"); err != nil {
		return nil, err
	}

	pageW, _ := pdf.GetPageSize()
	usableW := pageW - 2*margin
	colW := usableW / 2
	const labelW = 32.0
	const rowH = 4.4

	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(0, 3.5, "Generated "+time.Now().Format("2006-01-02 15:04"), "", 1, "R", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	section := func(title string) {
		pdf.Ln(0.8)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(235, 235, 235)
		pdf.CellFormat(0, 4.6, title, "", 1, "L", true, 0, "")
		pdf.Ln(0.3)
	}

	// labelWidth measures how much room label needs (bold, plus a little
	// padding), floored at labelW so short labels still line up.
	labelWidth := func(label string) float64 {
		pdf.SetFont("Helvetica", "B", 8)
		w := pdf.GetStringWidth(label+":") + 2
		if w < labelW {
			w = labelW
		}
		return w
	}

	// field draws one label:value cell within a column of width w, then
	// either moves to the right-hand column (ln=0) or wraps to the next
	// line at the left margin (ln=1). The label column is sized to fit the
	// label itself (capped at 60% of w) rather than a fixed width, since a
	// long label (e.g. an Axis Settings field name) would otherwise run
	// straight into the value — the same overlap problem long values have,
	// just on the other side of the cell.
	field := func(label, value string, w float64, ln int) {
		lw := labelWidth(label)
		if maxLW := w * 0.6; lw > maxLW {
			lw = maxLW
		}
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(lw, rowH, label+":", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.CellFormat(w-lw, rowH, value, "", ln, "L", false, 0, "")
	}

	// fieldWide prints one label:value row spanning the full page width, for
	// values too long to safely share a half-width column (a combined
	// "name — timestamp" sign-off, a full site address, etc).
	fieldWide := func(label, value string) {
		field(label, value, usableW, 1)
	}

	// grid lays out label/value pairs two per row, wrapping onto its own
	// full-width row (instead of overlapping into the next column) any
	// pair whose label or value is too wide to fit a half-width column —
	// this is what actually prevents text overlap, rather than hardcoding
	// which fields run long.
	grid := func(pairs [][2]string) {
		fits := func(label, value string) bool {
			lw := labelWidth(label)
			if maxLW := colW * 0.6; lw > maxLW {
				lw = maxLW
			}
			pdf.SetFont("Helvetica", "", 8)
			return pdf.GetStringWidth(value) <= colW-lw-2
		}
		var pendingLabel, pendingValue string
		hasPending := false
		flush := func() {
			if hasPending {
				field(pendingLabel, pendingValue, colW, 1)
				hasPending = false
			}
		}
		for _, p := range pairs {
			if !fits(p[0], p[1]) {
				flush()
				fieldWide(p[0], p[1])
				continue
			}
			if !hasPending {
				pendingLabel, pendingValue, hasPending = p[0], p[1], true
				continue
			}
			field(pendingLabel, pendingValue, colW, 0)
			field(p[0], p[1], colW, 1)
			hasPending = false
		}
		flush()
	}

	// The unit-level identity fields are set at intake and may be left
	// blank; fall back to the Stage 1-A device configuration (captured
	// during encoding) if so, since both describe the same physical unit.
	effectiveSerial := coalesceStr(sheet.SerialNumber, sheet.ConfigSerialNumber)
	effectiveMake := coalesceStr(sheet.DeviceMake, sheet.ConfigDeviceMake)
	effectiveModel := coalesceStr(sheet.DeviceModel, sheet.ConfigDeviceModel)
	str := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}

	// Small scannable barcodes for the three identifiers printed on a unit's
	// physical label, laid out side by side (rather than stacked) to keep
	// this section compact.
	type bcItem struct{ label, value string }
	var bcItems []bcItem
	if sheet.Barcode != "" {
		bcItems = append(bcItems, bcItem{"Barcode", sheet.Barcode})
	}
	if str(effectiveSerial) != "" {
		bcItems = append(bcItems, bcItem{"Serial Number", *effectiveSerial})
	}
	if str(sheet.PartNumber) != "" {
		bcItems = append(bcItems, bcItem{"Part Number", *sheet.PartNumber})
	}
	if len(bcItems) > 0 {
		section("Barcodes")
		itemW := usableW / float64(len(bcItems))
		const labelH, imgH, valueH = 3.2, 7.0, 3.2
		// moduleScale is an exact multiplier (not a fixed target width) for
		// the barcode's pixel width — boombuler's Scale() truncates width/
		// orgWidth to an integer factor and centers the result, so scaling
		// every barcode to the *same* fixed canvas (e.g. 300px) leaves a
		// different amount of blank padding baked in for each one depending
		// on its content length, which is what made them look raggedly
		// unaligned. Scaling by a clean multiple keeps zero padding, so each
		// barcode's rendered width is simply proportional to its own data —
		// all left- and top-aligned, all the same height.
		const moduleScale = 3
		startY := pdf.GetY()
		for i, it := range bcItems {
			x := margin + float64(i)*itemW
			maxW := itemW - 2
			pdf.SetXY(x, startY)
			pdf.SetFont("Helvetica", "B", 7)
			pdf.CellFormat(maxW, labelH, it.label, "", 2, "L", false, 0, "")
			if code, err := code128.Encode(it.value); err == nil {
				orgWidth := code.Bounds().Max.X - code.Bounds().Min.X
				pxW, pxH := orgWidth*moduleScale, 60
				if scaled, err := barcode.Scale(code, pxW, pxH); err == nil {
					var buf bytes.Buffer
					if png.Encode(&buf, scaled) == nil {
						imgName := fmt.Sprintf("barcode-%d", i)
						imgOpts := fpdf.ImageOptions{ImageType: "PNG"}
						pdf.RegisterImageOptionsReader(imgName, imgOpts, &buf)
						w := imgH * float64(pxW) / float64(pxH)
						if w > maxW {
							w = maxW
						}
						pdf.ImageOptions(imgName, x, pdf.GetY(), w, imgH, false, imgOpts, 0, "")
					}
				}
			}
			pdf.SetXY(x, startY+labelH+imgH)
			pdf.SetFont("Courier", "B", 8)
			pdf.CellFormat(maxW, valueH, it.value, "", 2, "L", false, 0, "")
		}
		pdf.SetXY(margin, startY+labelH+imgH+valueH+0.8)
	}

	section("Identification")
	idFields := [][2]string{
		{"Barcode", sheet.Barcode},
		{"Alias / Name", dash(sheet.Alias)},
		{"Serial Number", dash(effectiveSerial)},
		{"Device Make", dash(effectiveMake)},
		{"Device Model", dash(effectiveModel)},
		{"Part Number", dash(sheet.PartNumber)},
		{"Current Stage", humanize(sheet.BoardColumn)},
		{"Created", sheet.CreatedAt.Format("2006-01-02 15:04")},
	}
	if sheet.IsException {
		idFields = append(idFields, [2]string{"Exception", "Yes"})
	}
	if sheet.AllocatedBranch != nil {
		idFields = append(idFields, [2]string{"Allocated Branch", *sheet.AllocatedBranch})
	}
	grid(idFields)

	if len(sheet.Attributes) > 0 {
		section("Attributes")
		pairs := make([][2]string, len(sheet.Attributes))
		for i, f := range sheet.Attributes {
			pairs[i] = [2]string{f.Label, f.Value}
		}
		grid(pairs)
	}

	section("Device & Network Configuration")
	if sheet.HasDeviceConfig {
		grid([][2]string{
			{"Device Name (DNS)", dash(sheet.DeviceNameDNS)},
			{"Client IP Address", dash(sheet.ClientIPAddress)},
			{"MAC Address", dash(sheet.MACAddress)},
			{"DNS Server 1", dash(sheet.DNSServer1)},
			{"DNS Server 2", dash(sheet.DNSServer2)},
			{"NTP Server", dash(sheet.NTPServer)},
			{"Frequency (Hz)", dashInt(sheet.FrequencyHz)},
			{"Default Username", dash(sheet.DefaultUsername)},
			{"Encoded", dashBy(sheet.EncodedByName, sheet.EncodedAt)},
			{"Configured", dashBy(sheet.ConfiguredByName, sheet.ConfiguredAt)},
			{"QC", dashBy(sheet.QCByName, sheet.QCAt)},
			{"Signed Off", dashTime(sheet.DeviceConfigSignedOffAt)},
		})
	} else {
		grid([][2]string{{"Status", "Not yet configured"}})
	}

	if sheet.HasDeviceConfig && sheet.Accessories != nil {
		section("Accessories")
		grid([][2]string{
			{"Accessories Type", dash(sheet.Accessories.Type)},
			{"Input Type", dash(sheet.Accessories.InputType)},
			{"Model", dash(sheet.Accessories.Model)},
			{"Power Type", dash(sheet.Accessories.PowerType)},
			{"Automatic Gain Control", dashBool(sheet.Accessories.AutomaticGainControl)},
		})
	}

	if sheet.HasDeviceConfig && len(sheet.AxisSettings) > 0 {
		section("Axis Settings")
		pairs := make([][2]string, len(sheet.AxisSettings))
		for i, f := range sheet.AxisSettings {
			pairs[i] = [2]string{f.Label, f.Value}
		}
		grid(pairs)
	}

	section("Firmware")
	if sheet.HasFirmwareConfig {
		grid([][2]string{
			{"Firmware Updated", dashBool(sheet.FirmwareUpdated)},
			{"Firmware Version", dash(sheet.FirmwareVersion)},
			{"Signed Off", dashTime(sheet.FirmwareSignedOffAt)},
		})
	} else {
		grid([][2]string{{"Status", "Not yet configured"}})
	}

	if sheet.HasShipment {
		section("Shipment & Logistics")
		grid([][2]string{
			{"Site Name", dash(sheet.SiteName)},
			{"Site Location", dash(sheet.SiteLocation)},
			{"Site IP", dash(sheet.SiteIP)},
			{"Site Subnet", dash(sheet.SiteSubnet)},
			{"Site Gateway", dash(sheet.SiteGateway)},
			{"Deployment Team", dash(sheet.DeploymentTeam)},
			{"Team Leader", dash(sheet.TeamLeader)},
			{"Shipped Via", dash(sheet.ShippedVia)},
			{"Shipping Provider", dash(sheet.ShippingProvider)},
			{"Waybill Number", dash(sheet.WaybillNumber)},
			{"Delivered", dashTime(sheet.ShipmentDeliveredAt)},
		})
	}

	if sheet.HasInstallation {
		section("Installation")
		grid([][2]string{
			{"Installed Location", dash(sheet.InstalledLocation)},
			{"Installed Height (m)", dashFloat(sheet.InstalledHeightM)},
			{"Date Installed", dashTime(sheet.DateInstalled)},
			{"Network Attached", dashBool(sheet.NetworkAttached)},
			{"Device Contactable", dashBool(sheet.DeviceContactable)},
			{"Signed Off", dashTime(sheet.InstallationSignedOffAt)},
		})
	}

	if sheet.HasAcceptance {
		section("Client Acceptance")

		// Only signers who actually acted are shown — a client_acceptances
		// row can exist (e.g. a signing link was generated) before BSP has
		// accepted, and Branch Manager / Head Office are both optional,
		// whichever-signs-first signers, not sequential requirements. Always
		// printing all three with "-" placeholders reads as "still missing"
		// even when the stage is already fully signed off some other way.
		hasBSP := sheet.BSPAcceptanceDate != nil
		hasBranchManager := sheet.ClientName != nil || sheet.ClientSignedAt != nil
		hasHeadOffice := sheet.HeadOfficeName != nil || sheet.HeadOfficeSignedAt != nil

		type signer struct {
			label, imgName string
			name           *string
			signedAt       *time.Time
			sigData        *string
		}
		var signers []signer
		if hasBSP {
			// BSP Accepted By shows whoever actually signed (the typed
			// "Signed by" name — a Branch Manager or BSP representative on
			// site standing in for BSP), not the logged-in account that may
			// have submitted the form on their behalf; falls back to the
			// account name for older records that predate capturing it.
			signers = append(signers, signer{
				"BSP Accepted By", "bsp-signature",
				coalesceStr(sheet.BSPSignedByName, sheet.BSPAcceptanceBy), sheet.BSPAcceptanceDate, sheet.BSPSignature,
			})
		}
		if hasBranchManager {
			signers = append(signers, signer{
				"Branch Manager", "client-signature",
				sheet.ClientName, sheet.ClientSignedAt, sheet.ClientSignatureData,
			})
		}
		if hasHeadOffice {
			signers = append(signers, signer{
				"Head Office / Security Manager", "head-office-signature",
				sheet.HeadOfficeName, sheet.HeadOfficeSignedAt, sheet.HeadOfficeSignatureData,
			})
		}

		// Each signer gets one column stacking their name, their own signed
		// date, and their signature — rather than splitting the date off
		// into a shared field grid and the signature into a separate row.
		if len(signers) > 0 {
			const sigW, sigH = 50.0, 16.0
			startY := pdf.GetY()
			colWN := usableW / float64(len(signers))
			maxUsedH := 0.0

			// inlineField draws "Label: value" on one line within column
			// width w, sizing the label to fit itself so short labels don't
			// waste space and long ones don't overrun the value.
			inlineField := func(x, w, h float64, label, value string) {
				pdf.SetXY(x, pdf.GetY())
				pdf.SetFont("Helvetica", "B", 7.5)
				lw := pdf.GetStringWidth(label+": ") + 1
				if maxLW := w * 0.55; lw > maxLW {
					lw = maxLW
				}
				pdf.CellFormat(lw, h, label+":", "", 0, "L", false, 0, "")
				pdf.SetFont("Helvetica", "", 7.5)
				pdf.CellFormat(w-lw, h, value, "", 2, "L", false, 0, "")
				pdf.SetX(x)
			}

			for i, sg := range signers {
				x := margin + float64(i)*colWN
				w := colWN - 3

				pdf.SetXY(x, startY)
				inlineField(x, w, 4, sg.label, dash(sg.name))
				inlineField(x, w, 3.8, "Signed Off", dashTime(sg.signedAt))

				pdf.SetFont("Helvetica", "B", 7.5)
				pdf.CellFormat(w, 3.8, "Signature:", "", 2, "L", false, 0, "")
				sigY := pdf.GetY()

				used := 0.0
				if sg.sigData != nil && *sg.sigData != "" {
					if h := drawSignatureImage(pdf, sg.imgName, *sg.sigData, x, sigY, sigW, sigH); h > 0 {
						used = h
					} else {
						pdf.SetXY(x, sigY)
						pdf.SetFont("Helvetica", "", 7.5)
						pdf.CellFormat(w, 3.8, *sg.sigData, "", 2, "L", false, 0, "")
						used = 3.8
					}
				}
				if total := (sigY - startY) + used; total > maxUsedH {
					maxUsedH = total
				}
			}
			pdf.SetXY(margin, startY+maxUsedH+1)
		}

		if sheet.AcceptanceComments != nil && *sheet.AcceptanceComments != "" {
			pdf.SetFont("Helvetica", "B", 7.5)
			pdf.CellFormat(0, 3.8, "Comments:", "", 1, "L", false, 0, "")
			pdf.SetFont("Helvetica", "", 7.5)
			pdf.MultiCell(0, 3.6, *sheet.AcceptanceComments, "", "L", false)
		}
	}

	if sheet.HasDefect {
		section("Defect")
		grid([][2]string{
			{"Type", dash(sheet.DefectType)},
			{"Replacement Status", dash(sheet.DefectReplacementStatus)},
			{"Declared", dashTime(sheet.DefectDeclaredDate)},
		})
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(0, 5, "Description:", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.MultiCell(0, 4.5, dash(sheet.DefectDescription), "", "L", false)
	}

	if len(sheet.Photos) > 0 {
		drawPhotoPages(pdf, sheet.Photos, margin)
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("render unit info sheet pdf: %w", err)
	}
	return out.Bytes(), nil
}

// embeddableImageExts are the raster formats fpdf can decode and place
// directly (see go-pdf/fpdf's RegisterImageOptions). Installation photos may
// also be uploaded as webp/heic (see installation.allowedPhotoExtensions),
// which aren't embeddable — those get a text placeholder instead so a single
// unsupported photo doesn't set fpdf's sticky internal error and blank out
// the rest of the document.
var embeddableImageExts = map[string]string{
	".jpg": "JPG", ".jpeg": "JPG", ".png": "PNG", ".gif": "GIF",
}

// drawPhotoPages appends one page per uploaded installation photo — the
// "Uploaded Images / Proof of Installation" section — each with the photo
// scaled to fit the page and a caption noting its upload time. Called only
// once fpdf's own error state is known clean, so a missing/corrupt file is
// checked for up front rather than let RegisterImageOptions set pdf.err.
func drawPhotoPages(pdf *fpdf.Fpdf, photos []UnitPhoto, margin float64) {
	pageW, pageH := pdf.GetPageSize()
	usableW := pageW - 2*margin

	for i, photo := range photos {
		pdf.AddPage()
		title := fmt.Sprintf("Uploaded Images / Proof of Installation (%d of %d)", i+1, len(photos))
		if err := pdfbrand.DrawHeader(pdf, title); err != nil {
			continue
		}

		top := pdf.GetY() + 2
		const captionH = 5.0
		maxH := pageH - top - margin - captionH

		imgType, ok := embeddableImageExts[strings.ToLower(filepath.Ext(photo.FilePath))]
		if _, statErr := os.Stat(photo.FilePath); statErr != nil {
			ok = false
		}

		drawH := 0.0
		if ok {
			info := pdf.RegisterImageOptions(photo.FilePath, fpdf.ImageOptions{ImageType: imgType, ReadDpi: true})
			if pdf.Err() {
				pdf.ClearError()
				ok = false
			} else if info != nil {
				w, h := info.Extent()
				if w > 0 && h > 0 {
					scale := usableW / w
					if h*scale > maxH {
						scale = maxH / h
					}
					var drawW float64
					drawW, drawH = w*scale, h*scale
					x := margin + (usableW-drawW)/2
					pdf.ImageOptions(photo.FilePath, x, top, drawW, drawH, false, fpdf.ImageOptions{ImageType: imgType}, 0, "")
					if pdf.Err() {
						pdf.ClearError()
					}
				}
			}
		}

		pdf.SetXY(margin, top+drawH+1.5)
		pdf.SetFont("Helvetica", "", 8)
		if !ok {
			pdf.CellFormat(0, captionH, "Photo not embeddable ("+filepath.Base(photo.FilePath)+") - view in app", "", 1, "L", false, 0, "")
		} else {
			pdf.CellFormat(0, captionH, "Uploaded "+photo.UploadedAt.Format("2006-01-02 15:04"), "", 1, "L", false, 0, "")
		}
	}
}

// PackingListToPDF renders a printable packing list for every unit shipped
// to the given site.
func PackingListToPDF(siteName string, entries []PackingListEntry) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	if err := pdfbrand.DrawHeader(pdf); err != nil {
		return nil, err
	}
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "Packing List - "+siteName, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.Ln(2)

	headers := []string{"Barcode", "Device Model", "Serial Number", "Waybill", "Shipped Via", "Team"}
	widths := []float64{35.0, 40.0, 35.0, 30.0, 25.0, 25.0}

	pdf.SetFont("Helvetica", "B", 10)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 8, h, "1", 0, "L", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	for _, e := range entries {
		pdf.CellFormat(widths[0], 7, e.Barcode, "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[1], 7, strOrEmpty(e.DeviceModel), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[2], 7, strOrEmpty(e.SerialNumber), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[3], 7, strOrEmpty(e.WaybillNumber), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[4], 7, strOrEmpty(e.ShippedVia), "1", 0, "L", false, 0, "")
		pdf.CellFormat(widths[5], 7, strOrEmpty(e.DeploymentTeam), "1", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("render packing list pdf: %w", err)
	}
	return out.Bytes(), nil
}

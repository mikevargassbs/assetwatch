// Package pdfbrand draws the SBS AssetWatch logo and wordmark shared across
// every generated PDF report, so printed dockets, sheets, and lists all
// carry consistent branding instead of each report package rolling its own.
package pdfbrand

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/go-pdf/fpdf"
)

//go:embed AssetWatchLogo.png
var logoPNG []byte

const (
	logoSizeMM   = 10.0
	logoGapMM    = 3.0 // horizontal gap between the logo and the wordmark
	ruleGapMM    = 2.0 // vertical gap between the brand row and the divider rule
	contentGapMM = 2.0 // vertical gap between the divider rule and whatever comes next
)

// HeaderHeightMM is the total vertical space DrawHeader consumes, measured
// from the page's top margin down to where it leaves the cursor. Reports
// that reserve a fixed content budget to fit everything on one page (e.g.
// the waybill's item table) should add this to their own calculations.
const HeaderHeightMM = logoSizeMM + ruleGapMM + contentGapMM

// DrawHeader draws the logo + "SBS AssetWatch" wordmark at the page's
// current position (the left/top margin, immediately after AddPage), then a
// thin divider rule, and leaves the cursor positioned for whatever content
// the caller draws next. It must be called once, right after AddPage(),
// before any other content on the page.
//
// An optional report title (e.g. "Hardware Information Sheet") is printed
// inline on the same row, right-aligned — this keeps the header to a single
// compact row instead of the caller drawing its own separate title block
// beneath it, which matters for reports that need to fit on one page.
func DrawHeader(pdf *fpdf.Fpdf, title ...string) error {
	x, y := pdf.GetXY()

	imgOpts := fpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("pdfbrand-logo", imgOpts, bytes.NewReader(logoPNG))
	pdf.ImageOptions("pdfbrand-logo", x, y, logoSizeMM, logoSizeMM, false, imgOpts, 0, "")
	if pdf.Err() {
		return fmt.Errorf("draw pdf brand header: %w", pdf.Error())
	}

	pdf.SetXY(x+logoSizeMM+logoGapMM, y)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(0, logoSizeMM, "SBS AssetWatch", "", 0, "L", false, 0, "")

	pageW, _ := pdf.GetPageSize()
	_, _, rMargin, _ := pdf.GetMargins()
	if len(title) > 0 && title[0] != "" {
		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetXY(x, y)
		pdf.CellFormat(pageW-rMargin-x, logoSizeMM, title[0], "", 0, "R", false, 0, "")
	}

	ruleY := y + logoSizeMM + ruleGapMM
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetLineWidth(0.2)
	pdf.Line(x, ruleY, pageW-rMargin, ruleY)
	pdf.SetDrawColor(0, 0, 0)

	pdf.SetXY(x, ruleY+contentGapMM)
	return nil
}

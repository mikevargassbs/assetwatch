package logistics

import (
	"bytes"
	"context"
	"fmt"
	"image/png"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"

	"sbs-bsp-cctv/internal/pdfbrand"
)

// pageContentHeightMM is the usable vertical space on an A4 page (297mm)
// after default ~10mm top/bottom margins, with a little breathing room.
const pageContentHeightMM = 270.0

// GenerateDocketWaybill renders a printable waybill PDF for a delivery
// docket: its shipping details, the list of hardware units it carries, and a
// signature block — sized to always fit on a single page regardless of item
// count, so it prints as one sheet.
func (s *Service) GenerateDocketWaybill(ctx context.Context, docketID uuid.UUID) ([]byte, error) {
	detail, err := s.GetDocket(ctx, docketID)
	if err != nil {
		return nil, err
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	if err := pdfbrand.DrawHeader(pdf); err != nil {
		return nil, err
	}

	// titleBandHeight is the vertical space the title cell + its trailing gap
	// occupy (10 + 4, set below) — the barcode is sized to fill exactly that
	// same band so its top and bottom line up with the title text next to it.
	const titleBandHeight = 10 + 4
	if err := renderDocketNumberBarcode(pdf, detail.DocketNumber, titleBandHeight); err != nil {
		return nil, err
	}

	// Fixed chrome (brand header, title, docket fields, section titles, table
	// header, and the signature block) plus one row per item must all fit
	// within pageContentHeightMM — shrink the row height to fit as items grow.
	const headerHeight = pdfbrand.HeaderHeightMM + 10 + 4 + 7*7 + 6 + 8 + 7 // title + gap + 7 detail rows + gap + items heading + table header
	const signatureHeight = 6 + 30
	available := pageContentHeightMM - headerHeight - signatureHeight
	rowHeight := 7.0
	if n := len(detail.Items); n > 0 {
		if maxRow := available / float64(n); maxRow < rowHeight {
			rowHeight = maxRow
		}
	}
	if rowHeight < 4 {
		rowHeight = 4
	}
	fontSize := 10.0
	if rowHeight < 6 {
		fontSize = 8
	}

	// Capped at 130mm (rather than the full row width) so it doesn't run
	// under the docket-number barcode in the top-right corner.
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(130, 10, "Waybill / Delivery Docket", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.Ln(4)

	row := func(label, value string) {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(50, 7, label, "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.CellFormat(0, 7, value, "", 1, "L", false, 0, "")
	}

	row("Docket Number:", detail.DocketNumber)
	row("Waybill Number:", strOr(detail.WaybillNumber))
	row("Destination Site:", strOr(detail.DestinationSite))
	row("Shipped Via:", strOr(detail.ShippedVia))
	row("Shipping Provider:", strOr(detail.ShippingProvider))
	row("Dispatched By:", strOr(detail.DispatchedBy))
	row("Received By:", strOr(detail.ReceivedBy))

	pdf.Ln(6)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, fmt.Sprintf("Items (%d)", len(detail.Items)), "", 1, "L", false, 0, "")

	const checkColWidth = 10.0
	colWidths := []float64{checkColWidth, 30, 35, 30, 45, 50} // Check, Barcode, Serial Number, Make, Model, Part Number
	headers := []string{"Check", "Barcode", "Serial Number", "Make", "Model", "Part Number"}

	pdf.SetFont("Helvetica", "B", fontSize)
	for i, h := range headers {
		align := "L"
		border := "B"
		width := colWidths[i]
		if i == len(headers)-1 {
			width = 0
		}
		pdf.CellFormat(width, 7, h, border, boolToLn(i == len(headers)-1), align, false, 0, "")
	}

	// A small hand-tickable box in place of a text cell, so the printed sheet
	// doubles as a physical checklist when loading/counting items on site.
	boxSize := rowHeight - 2
	if boxSize > 4 {
		boxSize = 4
	} else if boxSize < 2 {
		boxSize = 2
	}

	pdf.SetFont("Helvetica", "", fontSize)
	for _, item := range detail.Items {
		x, y := pdf.GetXY()
		pdf.Rect(x+(checkColWidth-boxSize)/2, y+(rowHeight-boxSize)/2, boxSize, boxSize, "D")
		pdf.CellFormat(checkColWidth, rowHeight, "", "", 0, "L", false, 0, "")

		values := []string{item.Barcode, strOr(item.SerialNumber), strOr(item.DeviceMake), strOr(item.DeviceModel), strOr(item.PartNumber)}
		for i, v := range values {
			width := colWidths[i+1]
			if i == len(values)-1 {
				width = 0
			}
			pdf.CellFormat(width, rowHeight, v, "", boolToLn(i == len(values)-1), "L", false, 0, "")
		}
	}

	// Signature block, anchored near the bottom of the page.
	sigY := 297.0 - 10 - signatureHeight
	pdf.SetXY(10, sigY)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Signatures", "", 1, "L", false, 0, "")
	sigY += 8

	signatureBlock := func(x float64, label string) {
		pdf.Line(x, sigY+18, x+80, sigY+18)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetXY(x, sigY+19)
		pdf.CellFormat(80, 5, label, "", 0, "L", false, 0, "")
		pdf.SetXY(x, sigY+24)
		pdf.CellFormat(80, 5, "Printed Name:", "", 0, "L", false, 0, "")
		pdf.SetXY(x, sigY+29)
		pdf.CellFormat(80, 5, "Date:", "", 0, "L", false, 0, "")
	}

	signatureBlock(10, "Dispatched By (Signature)")
	signatureBlock(110, "Received By (Signature)")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("render waybill pdf: %w", err)
	}
	return out.Bytes(), nil
}

// renderDocketNumberBarcode draws a Code 128 barcode of the docket number in
// the top-right corner of the current page, so the printed waybill can be
// scanned directly instead of the number being re-typed at receiving. It's
// sized to exactly fill height (top-aligned with the page margin, same as the
// title text beside it), so the barcode's top and bottom edges line up with
// the header title rather than floating at an unrelated size. The docket
// number is already printed as text in the "Docket Number:" row below, so no
// separate human-readable label is drawn under the bars here.
func renderDocketNumberBarcode(pdf *fpdf.Fpdf, docketNumber string, height float64) error {
	code, err := code128.Encode(docketNumber)
	if err != nil {
		return fmt.Errorf("encode barcode: %w", err)
	}
	scaled, err := barcode.Scale(code, 300, 80)
	if err != nil {
		return fmt.Errorf("scale barcode: %w", err)
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, scaled); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}

	// boombuler always rasterizes onto a fixed 300x80px canvas — preserve
	// that aspect ratio so the bars aren't stretched out of proportion.
	const barcodeAspect = 300.0 / 80.0
	barcodeH := height
	barcodeW := barcodeH * barcodeAspect

	const pageWidthMM = 210.0
	const marginMM = 10.0
	x := pageWidthMM - marginMM - barcodeW
	y := marginMM + pdfbrand.HeaderHeightMM

	imgOpts := fpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("docket-barcode", imgOpts, &pngBuf)
	pdf.ImageOptions("docket-barcode", x, y, barcodeW, barcodeH, false, imgOpts, 0, "")

	return nil
}

func boolToLn(last bool) int {
	if last {
		return 1
	}
	return 0
}

func strOr(s *string) string {
	if s == nil || *s == "" {
		return "—"
	}
	return *s
}

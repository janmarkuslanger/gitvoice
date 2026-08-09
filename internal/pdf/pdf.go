// Package pdf renders an invoice to a print-ready PDF document. It is pure:
// it transforms domain values into bytes in memory and performs no I/O
// (filesystem, network) itself — callers persist or stream the result.
package pdf

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/i18n"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
	"github.com/janmarkuslanger/gitvoice/internal/money"
)

const (
	margin     = 15.0  // page margin in mm
	contentW   = 180.0 // A4 width (210) minus both margins
	bottomEdge = 282.0 // A4 height (297) minus the bottom margin
	lineH      = 6.0   // default line height in mm
	font       = "Helvetica"
)

// item table column widths (sum == contentW).
const (
	colDesc  = 90.0
	colQty   = 25.0
	colUnit  = 30.0
	colTotal = 35.0
)

// Render produces a print-ready PDF of inv issued by comp, with all fixed
// labels translated into lang. The standard Helvetica font is used with a
// cp1252 translator, which covers German umlauts and the euro sign without
// embedding any font files, keeping the binary self-contained.
func Render(inv invoice.Invoice, comp company.Company, lang string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, 297-bottomEdge)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	t := func(key string) string { return tr(i18n.T(lang, key)) }
	pdf.AddPage()

	writeHeader(pdf, tr, t, inv)
	writeParties(pdf, tr, t, lang, inv, comp)
	writeItems(pdf, tr, t, inv)
	writeTotals(pdf, tr, t, inv)
	writeNotes(pdf, tr, t, inv, comp)
	writeFooter(pdf, tr, lang, comp)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render invoice %s to PDF: %w", inv.Number, err)
	}
	return buf.Bytes(), nil
}

type translator = func(string) string

// joinLines joins the non-empty parts with newlines, used to build address
// blocks that skip fields the user left blank.
func joinLines(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n")
}

// writeHeader prints the invoice title and the date / due / time-of-supply block.
func writeHeader(pdf *fpdf.Fpdf, tr, t translator, inv invoice.Invoice) {
	pdf.SetFont(font, "B", 20)
	pdf.CellFormat(contentW, 10, t("view.invoice")+" "+tr(inv.Number), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont(font, "", 10)
	meta := [][2]string{{t("th.date"), tr(inv.Date)}}
	if inv.DueDate != "" {
		meta = append(meta, [2]string{t("view.due"), tr(inv.DueDate)})
	}
	if inv.HasServicePeriod() {
		meta = append(meta, [2]string{t("view.service_period"), tr(inv.ServicePeriodStart + " – " + inv.ServicePeriodEnd)})
	} else if inv.ServiceDate != "" {
		meta = append(meta, [2]string{t("view.service_date"), tr(inv.ServiceDate)})
	}
	for _, row := range meta {
		pdf.SetFont(font, "B", 10)
		pdf.CellFormat(40, lineH, row[0], "", 0, "L", false, 0, "")
		pdf.SetFont(font, "", 10)
		pdf.CellFormat(contentW-40, lineH, row[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)
}

// writeParties prints the issuer (From) and recipient (Billed to) columns
// side by side, advancing to the bottom of the taller of the two.
func writeParties(pdf *fpdf.Fpdf, tr, t translator, lang string, inv invoice.Invoice, comp company.Company) {
	const colW = 85.0
	startY := pdf.GetY()

	from := joinLines(comp.Name, comp.Address, comp.Email, comp.Phone)
	leftY := writeParty(pdf, tr, t, margin, startY, colW, "view.from", from)

	person := ""
	if inv.Customer.Company != "" {
		person = inv.Customer.PersonName()
	}
	vatLine := ""
	if inv.Customer.VATID != "" {
		vatLine = i18n.T(lang, "view.vat_id") + ": " + inv.Customer.VATID
	}
	billed := joinLines(inv.Customer.DisplayName(), person, inv.Customer.Address, inv.Customer.Email, vatLine)
	rightY := writeParty(pdf, tr, t, margin+contentW-colW, startY, colW, "view.billed_to", billed)

	pdf.SetY(max(leftY, rightY) + 4)
}

// writeParty renders one titled address block at (x, y) and returns the Y
// position just below it.
func writeParty(pdf *fpdf.Fpdf, tr, t translator, x, y, w float64, titleKey, body string) float64 {
	pdf.SetXY(x, y)
	pdf.SetFont(font, "B", 11)
	pdf.CellFormat(w, lineH, t(titleKey), "", 2, "L", false, 0, "")
	pdf.SetFont(font, "", 10)
	if body != "" {
		pdf.MultiCell(w, 5, tr(body), "", "L", false)
	}
	return pdf.GetY()
}

// writeItems prints the line-item table, repeating the header row after any
// automatic page break.
func writeItems(pdf *fpdf.Fpdf, tr, t translator, inv invoice.Invoice) {
	header := func() {
		pdf.SetFont(font, "B", 9)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(colDesc, lineH, t("th.description"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(colQty, lineH, t("th.qty"), "1", 0, "R", true, 0, "")
		pdf.CellFormat(colUnit, lineH, t("th.unit_price"), "1", 0, "R", true, 0, "")
		pdf.CellFormat(colTotal, lineH, t("th.total"), "1", 1, "R", true, 0, "")
	}
	header()
	pdf.SetFont(font, "", 9)
	for _, it := range inv.Items {
		desc := tr(it.Description)
		// SplitLines works on the cp1252 bytes the translator produces;
		// SplitText would range over them as runes and panic on high bytes.
		n := len(pdf.SplitLines([]byte(desc), colDesc))
		if n == 0 {
			n = 1
		}
		rowH := 5.0 * float64(n)
		if pdf.GetY()+rowH > bottomEdge {
			pdf.AddPage()
			header()
			pdf.SetFont(font, "", 9)
		}
		x, y := pdf.GetX(), pdf.GetY()
		pdf.MultiCell(colDesc, 5, desc, "1", "L", false)
		pdf.SetXY(x+colDesc, y)
		pdf.CellFormat(colQty, rowH, tr(money.FormatQuantity(it.Quantity)), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colUnit, rowH, tr(money.FormatCents(it.UnitPriceCents)), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colTotal, rowH, tr(money.FormatCents(it.TotalCents())), "1", 1, "R", false, 0, "")
		pdf.SetXY(x, y+rowH)
	}
}

// writeTotals prints the net / VAT / gross breakdown, or a single gross total
// under the small-business rule.
func writeTotals(pdf *fpdf.Fpdf, tr, t translator, inv invoice.Invoice) {
	labelW := colDesc + colQty + colUnit
	row := func(label, value string, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		pdf.SetFont(font, style, 10)
		pdf.CellFormat(labelW, lineH, label, "", 0, "R", false, 0, "")
		pdf.CellFormat(colTotal, lineH, value, "", 1, "R", false, 0, "")
	}
	gross := tr(money.FormatCents(inv.TotalCents()) + " " + inv.Currency)
	if inv.SmallBusiness {
		row(t("th.total"), gross, true)
		return
	}
	row(t("view.net"), tr(money.FormatCents(inv.NetCents())), false)
	row(t("view.vat")+" "+tr(money.FormatQuantity(inv.TaxRatePercent))+"%", tr(money.FormatCents(inv.TaxCents())), false)
	row(t("th.total"), gross, true)
}

// writeNotes prints the small-business note and any free-text notes.
func writeNotes(pdf *fpdf.Fpdf, tr, t translator, inv invoice.Invoice, comp company.Company) {
	if inv.SmallBusiness {
		pdf.Ln(4)
		pdf.SetFont(font, "", 10)
		pdf.MultiCell(contentW, 5, tr(comp.Note()), "", "L", false)
	}
	if inv.Notes != "" {
		pdf.Ln(4)
		pdf.SetFont(font, "B", 11)
		pdf.CellFormat(contentW, lineH, t("view.notes"), "", 1, "L", false, 0, "")
		pdf.SetFont(font, "", 10)
		pdf.MultiCell(contentW, 5, tr(inv.Notes), "", "L", false)
	}
}

// writeFooter prints the issuer's tax and bank details at the bottom.
func writeFooter(pdf *fpdf.Fpdf, tr translator, lang string, comp company.Company) {
	var lines []string
	if comp.TaxNumber != "" {
		lines = append(lines, i18n.T(lang, "view.tax_number")+": "+comp.TaxNumber)
	}
	if comp.VATID != "" {
		lines = append(lines, i18n.T(lang, "view.vat_id")+": "+comp.VATID)
	}
	if comp.IBAN != "" {
		bank := "IBAN: " + comp.IBAN
		if comp.BIC != "" {
			bank += " · BIC: " + comp.BIC
		}
		if comp.BankName != "" {
			bank += " · " + comp.BankName
		}
		lines = append(lines, bank)
	}
	if len(lines) == 0 {
		return
	}
	pdf.Ln(6)
	pdf.SetFont(font, "", 8)
	pdf.SetTextColor(90, 90, 90)
	for _, line := range lines {
		pdf.CellFormat(contentW, 4.5, tr(line), "", 1, "L", false, 0, "")
	}
}

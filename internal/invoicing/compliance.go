package invoicing

import (
	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
)

// Warning identifies a § 14 UStG mandatory detail missing from an invoice.
// Its string value doubles as the i18n key the presentation layer translates,
// so the core stays free of user-facing text. The check never blocks.
type Warning string

const (
	WarnIssuerName       Warning = "compliance.issuer_name"
	WarnIssuerAddress    Warning = "compliance.issuer_address"
	WarnIssuerTaxID      Warning = "compliance.issuer_tax_id"
	WarnRecipientAddress Warning = "compliance.recipient_address"
	WarnServiceDate      Warning = "compliance.service_date"
)

// smallAmountLimitCents is the § 33 UStDV Kleinbetragsrechnung ceiling
// (250 EUR gross). At or below it, the reduced set of mandatory details
// applies: issuer name and address only. The limit is EUR-denominated; for
// other currencies the gross amount is compared as-is.
const smallAmountLimitCents = 25000

// ComplianceWarnings reports the § 14 UStG mandatory details missing from inv
// given the issuer profile comp, newest concern first. It is advisory: the
// caller decides whether and how to surface the result, and nothing here
// prevents saving. Issuer name and address are always required; the extended
// set (issuer tax number/VAT ID, recipient address, time of supply) applies
// only above the § 33 UStDV small-amount threshold.
func ComplianceWarnings(inv invoice.Invoice, comp company.Company) []Warning {
	var w []Warning
	if comp.DisplayName() == "" {
		w = append(w, WarnIssuerName)
	}
	if comp.Address == "" {
		w = append(w, WarnIssuerAddress)
	}
	if inv.TotalCents() <= smallAmountLimitCents {
		return w
	}
	if comp.TaxNumber == "" && comp.VATID == "" {
		w = append(w, WarnIssuerTaxID)
	}
	if inv.Customer.Address == "" {
		w = append(w, WarnRecipientAddress)
	}
	if inv.ServiceDate == "" && !inv.HasServicePeriod() {
		w = append(w, WarnServiceDate)
	}
	return w
}

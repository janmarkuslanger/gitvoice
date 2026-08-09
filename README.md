# gitvoice

Invoices with git. An embeddable Go library that serves a small web UI for
writing and managing invoices, storing everything as plain JSON files inside
your git repository — your repo is the database, `git log` is the audit trail.

## How it works

You don't clone this repo as a template. You create your own (private) repo
for your invoices, add a tiny `main.go` that imports gitvoice, and run it.
Updating gitvoice later is a normal `go get -u` — your data and your repo
stay untouched.

```
your-invoices-repo/
├── go.mod
├── main.go            <- ~10 lines, see below
└── data/
    ├── company.json   <- your issuer profile (who the invoices come from)
    ├── customers/
    │   └── acme.json
    └── invoices/
        ├── 2026-001.json
        └── 2026-002.json
```

## Usage

```go
package main

import (
	"log"

	"github.com/janmarkuslanger/gitvoice"
)

func main() {
	err := gitvoice.Run(gitvoice.Config{
		DataDir: "data",           // JSON files live here — commit them with git
		Addr:    "127.0.0.1:8080", // local only by default
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

```sh
go mod init your-invoices && go get github.com/janmarkuslanger/gitvoice
go run . # open http://127.0.0.1:8080
```

The zero value `gitvoice.Config{}` works too (data in `./data`, UI on
`127.0.0.1:8080`). For mounting into an existing server or adding auth
middleware, use `gitvoice.New(cfg)` and `app.Handler()`.

## Features

- Create, edit, and delete invoices in a plain, dependency-free web UI
  (embedded in the binary — no assets to deploy)
- Company profile (Settings page): name, address, Steuernummer, USt-IdNr.,
  bank account — printed on every invoice as the issuer
- VAT with a configurable rate per invoice (net / VAT / gross breakdown)
- Time of supply (Leistungsdatum or Leistungszeitraum) per invoice, printed
  on the sheet (§ 14 Abs. 4 Nr. 6 UStG)
- Non-blocking § 14 UStG completeness check: the invoice view flags missing
  mandatory details (issuer name/address, tax number or VAT ID, recipient
  address, time of supply), while respecting the reduced set for
  Kleinbetragsrechnungen up to 250 € gross (§ 33 UStDV)
- UI language switch (German / English) with a default in the profile
- Kleinunternehmerregelung (§ 19 UStG) toggle: when enabled, new invoices
  default to no VAT and print the § 19 note (text is configurable). The
  setting is snapshotted per invoice, so flipping it later never changes
  invoices you already issued
- Customer master data: company, first/last name, address, email, phone,
  VAT ID, and internal notes. Manage customers once, then pick them from a
  select when writing an invoice. The invoice stores its own copy of the
  customer data, so editing or deleting a customer never changes existing
  invoices
- Line items with quantity and unit price; totals are computed
- Print-friendly invoice view — use the browser's print dialog for PDFs
- Status tracking: draft, sent, paid, canceled

## Data format

One JSON file per invoice under `<DataDir>/invoices/<number>.json`,
pretty-printed so git diffs stay readable. Amounts are stored as integer
cents (`unit_price_cents`) to avoid floating-point drift.

```json
{
  "number": "2026-042",
  "date": "2026-08-07",
  "status": "sent",
  "currency": "EUR",
  "service_date": "2026-08-05",
  "customer": { "name": "ACME GmbH" },
  "items": [
    { "description": "Consulting", "quantity": 2, "unit_price_cents": 9550 }
  ],
  "small_business": false,
  "tax_rate_percent": 19
}
```

The issuer profile lives in `<DataDir>/company.json` and is edited on the
Settings page. Customers live in `<DataDir>/customers/<id>.json`.

## German legal compliance

gitvoice helps you produce invoices that meet the German § 14 UStG mandatory
details, but it does not decide compliance for you — the completeness check is
advisory and never blocks saving. Known scope and limitations:

- **Retention (GoBD):** JSON files in git are editable, and history can be
  rewritten, so the repository alone is not tamper-proof archival. Keep the
  printed PDFs of issued invoices in your regular, unchangeable archive for the
  full retention period (currently 8 years for invoices).
- **One VAT rate per invoice:** invoices mixing e.g. 19 % and 7 % are not
  supported yet.
- **No e-invoicing:** output is PDF/print, not a structured e-invoice
  (EN 16931 / XRechnung / ZUGFeRD). Domestic B2B issuing of e-invoices becomes
  mandatory in 2027/2028; businesses under the Kleinunternehmerregelung (§ 19)
  are exempt from *issuing* them.

This is guidance, not legal or tax advice — verify your invoices with your tax
advisor.

## Notes

- The UI has no authentication. It binds to localhost by default; put a
  reverse proxy with auth in front before exposing it anywhere else.

## License

Apache-2.0, see [LICENSE](LICENSE).

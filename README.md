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

## Command line

Everything the UI writes can also be written from a terminal or a script —
same data directory, same rules, same JSON files.

```sh
go run github.com/janmarkuslanger/gitvoice/cmd/gitvoice help

# add a customer to the master data
go run github.com/janmarkuslanger/gitvoice/cmd/gitvoice customer add \
  -id acme -company "ACME GmbH" \
  -address 'Musterstraße 1\n12345 Berlin' -vat-id DE123456789

# write an invoice for that customer
go run github.com/janmarkuslanger/gitvoice/cmd/gitvoice invoice add \
  -number 2026-001 -customer acme -due-date 2026-08-23 \
  -item 'Consulting;3;120.00' -item 'Travel;1;49,50'

# serve the web UI
go run github.com/janmarkuslanger/gitvoice/cmd/gitvoice serve
```

- Global flags come before the command: `-data <dir>` (default `data`) and
  `-addr <host:port>` for `serve`.
- `-item` takes `description;quantity;unit price` and can be repeated. The
  quantity and price are read from the end, so a description may contain `;`.
- `-customer <id>` copies the recipient from the master data; the
  `-customer-*` flags fill in or override single fields, as in the web form.
- Date, currency, VAT rate and the small-business rule default exactly like a
  new invoice in the UI: from today's date and your company profile.
- `\n` in `-address` and `-notes` becomes a real line break.
- Exit codes: `0` success, `1` the command failed, `2` wrong invocation.

To get the same commands from your own binary, call
`gitvoice.RunCLI(cfg, args, os.Stdout, os.Stderr)`.

## Features

- Create, edit, and delete invoices in a plain, dependency-free web UI
  (embedded in the binary — no assets to deploy)
- A command line for the same job: `customer add`, `invoice add`, `serve`
  (see [Command line](#command-line))
- Company profile (Settings page): name, address, Steuernummer, USt-IdNr.,
  bank account — printed on every invoice as the issuer
- VAT with a configurable rate per invoice (net / VAT / gross breakdown)
- Time of supply (single date or period) per invoice, printed on the sheet
- Non-blocking completeness check: the invoice view flags missing details
  (issuer name/address, tax number or VAT ID, recipient address, time of
  supply), with a reduced set for small-amount invoices
- UI language switch (German / English) with a default in the profile
- Small-business toggle: when enabled, new invoices default to no VAT and
  print a configurable note. The setting is snapshotted per invoice, so
  flipping it later never changes invoices you already issued
- Customer master data: company, first/last name, address, email, phone,
  VAT ID, and internal notes. Manage customers once, then pick them from a
  select when writing an invoice. The invoice stores its own copy of the
  customer data, so editing or deleting a customer never changes existing
  invoices
- Line items with quantity and unit price; totals are computed
- PDF export: "Save PDF" renders the invoice and files it under
  `<DataDir>/pdfs/<number>.pdf` (and downloads it). A print-friendly view is
  also available via the browser's print dialog
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
Settings page. Customers live in `<DataDir>/customers/<id>.json`. Exported
PDFs are written to `<DataDir>/pdfs/<number>.pdf`.

## Disclaimer

gitvoice is provided "as is", without any warranty or liability whatsoever.
Whether its output is usable for your purpose must always be checked by you
before use. Use at your own risk.

## Notes

- The UI has no authentication. It binds to localhost by default; put a
  reverse proxy with auth in front before exposing it anywhere else.

## License

Apache-2.0, see [LICENSE](LICENSE).

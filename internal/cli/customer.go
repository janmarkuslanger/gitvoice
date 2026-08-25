package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/janmarkuslanger/gitvoice/internal/customer"
)

// customerAdd writes a new entry to the customer master data. Invoices copy
// these fields at creation time. There is no -id flag: the service derives
// the ID from the name and the command reports the one it stored.
func (r Runner) customerAdd(args []string) error {
	fs := r.flagSet("customer add")
	var c customer.Customer
	fs.StringVar(&c.Company, "company", "", "company name")
	fs.StringVar(&c.FirstName, "first-name", "", "given name")
	fs.StringVar(&c.LastName, "last-name", "", "family name")
	fs.StringVar(&c.Address, "address", "", `postal address, \n starts a new line`)
	fs.StringVar(&c.Email, "email", "", "email address")
	fs.StringVar(&c.Phone, "phone", "", "phone number, master data only")
	fs.StringVar(&c.VATID, "vat-id", "", "USt-IdNr., relevant for B2B")
	fs.StringVar(&c.Notes, "notes", "", "internal note, never printed")
	if err := parse(fs, args); err != nil {
		return err
	}
	c.Address = unescapeNewlines(c.Address)
	c.Notes = unescapeNewlines(c.Notes)
	stored, err := r.Svc.CreateCustomer(c)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "customer %s created\n", stored.ID)
	return nil
}

// customerList prints the master data as a table, sorted by name, or as the
// stored records under -json. It is how you look up the IDs that
// "invoice add -customer" expects.
func (r Runner) customerList(args []string) error {
	fs := r.flagSet("customer list")
	asJSON := fs.Bool("json", false, "print the customers as a JSON array")
	if err := parse(fs, args); err != nil {
		return err
	}
	customers, err := r.Svc.Customers()
	if err != nil {
		return err
	}
	if *asJSON {
		if customers == nil {
			customers = []customer.Customer{} // an empty array, not null
		}
		return r.printJSON(customers)
	}
	if len(customers) == 0 {
		fmt.Fprintln(r.Out, "no customers yet")
		return nil
	}
	w := tabwriter.NewWriter(r.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCOMPANY\tNAME\tEMAIL")
	for _, c := range customers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.ID, c.Company, c.PersonName(), c.Email)
	}
	return w.Flush()
}

package store

import (
	"fmt"
	"path/filepath"

	"github.com/janmarkuslanger/gitvoice/internal/invoice"
)

func (s *Store) pdfPath(number string) (string, error) {
	if !invoice.ValidNumber(number) {
		return "", fmt.Errorf("invalid invoice number %q", number)
	}
	return filepath.Join(s.pdfsDir, number+".pdf"), nil
}

// SavePDF writes the rendered PDF for the given invoice number to
// <dir>/pdfs/<number>.pdf, overwriting any previous version atomically.
func (s *Store) SavePDF(number string, data []byte) error {
	p, err := s.pdfPath(number)
	if err != nil {
		return err
	}
	if err := writeAtomic(p, data); err != nil {
		return fmt.Errorf("write PDF %s: %w", number, err)
	}
	return nil
}

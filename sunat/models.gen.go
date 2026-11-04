// Contract models. Regenerate through the pinned models generator.
package sunat

import (
	"fmt"
	"regexp"
	"time"
)

type ValidityQuery struct {
	IssuerRUC    string `json:"issuer_ruc"`
	DocumentType string `json:"document_type"`
	Series       string `json:"series"`
	Number       int64  `json:"number"`
	IssuedOn     string `json:"issued_on"`
	Amount       string `json:"amount"`
}

func (m ValidityQuery) Validate() error {
	if !validityQueryIssuerRUCPattern.MatchString(m.IssuerRUC) {
		return fmt.Errorf("issuer_ruc: invalid format")
	}
	if !validityQueryDocumentTypePattern.MatchString(m.DocumentType) {
		return fmt.Errorf("document_type: invalid format")
	}
	if !validityQuerySeriesPattern.MatchString(m.Series) {
		return fmt.Errorf("series: invalid format")
	}
	if m.Number < 1 {
		return fmt.Errorf("number: out of range")
	}
	if m.Number > 99999999 {
		return fmt.Errorf("number: out of range")
	}
	if t, err := time.Parse("2006-01-02", m.IssuedOn); err != nil || t.Format("2006-01-02") != m.IssuedOn {
		return fmt.Errorf("issued_on: expected YYYY-MM-DD")
	}
	if !validityQueryAmountPattern.MatchString(m.Amount) {
		return fmt.Errorf("amount: invalid format")
	}
	return nil
}

var validityQueryIssuerRUCPattern = regexp.MustCompile("^[0-9]{11}$")
var validityQueryDocumentTypePattern = regexp.MustCompile("^[A-Z0-9]{2}$")
var validityQuerySeriesPattern = regexp.MustCompile("^[A-Z0-9]{4}$")
var validityQueryAmountPattern = regexp.MustCompile("^(0|[1-9][0-9]{0,14})([.][0-9]{1,2})?$")

type ValidityResult struct {
	DocumentStatus      string   `json:"document_status"`
	IssuerStatus        string   `json:"issuer_status"`
	IssuerAddressStatus string   `json:"issuer_address_status"`
	Observations        []string `json:"observations"`
}

func (m ValidityResult) Validate() error {
	if !validityResultDocumentStatusPattern.MatchString(m.DocumentStatus) {
		return fmt.Errorf("document_status: invalid format")
	}
	if m.Observations == nil {
		return fmt.Errorf("observations: expected an array")
	}
	return nil
}

var validityResultDocumentStatusPattern = regexp.MustCompile("^[0-9]+$")

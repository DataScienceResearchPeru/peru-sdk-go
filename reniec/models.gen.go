// Contract models. Regenerate through the pinned models generator.
package reniec

import (
	"fmt"
	"regexp"
)

type IdentityQuery struct {
	DNI string `json:"dni"`
}

func (m IdentityQuery) Validate() error {
	if !identityQueryDNIPattern.MatchString(m.DNI) {
		return fmt.Errorf("dni: invalid format")
	}
	return nil
}

var identityQueryDNIPattern = regexp.MustCompile("^[0-9]{8}$")

type Identity struct {
	DNI           string `json:"dni"`
	GivenNames    string `json:"given_names"`
	FirstSurname  string `json:"first_surname"`
	SecondSurname string `json:"second_surname"`
}

func (m Identity) Validate() error {
	if !identityDNIPattern.MatchString(m.DNI) {
		return fmt.Errorf("dni: invalid format")
	}
	return nil
}

var identityDNIPattern = regexp.MustCompile("^[0-9]{8}$")

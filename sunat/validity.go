// Package sunat provides a native Consulta Integrada client. Live compatibility is not yet certified.
package sunat

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"
)

type ValidityOptions struct {
	RequesterRUC string
	TokenSource  TokenSource
	APIBaseURL   string
	HTTPClient   *http.Client
}

type ValidityClient struct {
	requesterRUC string
	tokens       TokenSource
	baseURL      string
	http         *http.Client
}

func NewValidityClient(options ValidityOptions) (*ValidityClient, error) {
	if !regexp.MustCompile(`^[0-9]{11}$`).MatchString(options.RequesterRUC) || options.TokenSource == nil {
		return nil, &Error{Kind: "configuration", Operation: "validity"}
	}
	base := options.APIBaseURL
	if base == "" {
		base = "https://api.sunat.gob.pe"
	}
	base, err := checkedEndpoint(base)
	if err != nil {
		return nil, err
	}
	return &ValidityClient{requesterRUC: options.RequesterRUC, tokens: options.TokenSource, baseURL: base, http: httpClient(options.HTTPClient)}, nil
}

// Check queries the service. A successful HTTP exchange is not a positive document status.
// There are no implicit retries and no hidden calls other than token acquisition when required.
func (c *ValidityClient) Check(ctx context.Context, query ValidityQuery) (ValidityResult, error) {
	if err := query.Validate(); err != nil {
		return ValidityResult{}, err
	}
	date, _ := time.Parse("2006-01-02", query.IssuedOn)
	request := struct {
		Issuer string      `json:"numRuc"`
		Type   string      `json:"codComp"`
		Series string      `json:"numeroSerie"`
		Number int64       `json:"numero"`
		Date   string      `json:"fechaEmision"`
		Amount json.Number `json:"monto"`
	}{query.IssuerRUC, query.DocumentType, query.Series, query.Number, date.Format("02/01/2006"), json.Number(query.Amount)}
	body, err := json.Marshal(request)
	if err != nil {
		return ValidityResult{}, &Error{Kind: "validation", Operation: "validity"}
	}
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return ValidityResult{}, err
	}
	if !validToken(token) {
		return ValidityResult{}, &Error{Kind: "configuration", Operation: "token"}
	}
	endpoint := c.baseURL + "/v1/contribuyente/contribuyentes/" + c.requesterRUC + "/validarcomprobante"
	data, err := post(ctx, c.http, "validity", endpoint, "application/json", token, body)
	if err != nil {
		return ValidityResult{}, err
	}
	return ParseValidityResponse(data)
}

// ParseValidityResponse accepts numeric or digit-string status codes and preserves unknown codes.
func ParseValidityResponse(body []byte) (ValidityResult, error) {
	failure := func() (ValidityResult, error) {
		return ValidityResult{}, &Error{Kind: "protocol", Operation: "validity"}
	}
	if len(body) > maxResponseBytes || !utf8.Valid(body) {
		return failure()
	}
	var response map[string]json.RawMessage
	if json.Unmarshal(body, &response) != nil || response == nil {
		return failure()
	}
	var success bool
	if string(response["success"]) != "true" && string(response["success"]) != "false" {
		return failure()
	}
	_ = json.Unmarshal(response["success"], &success)
	if !success {
		var code string
		_ = json.Unmarshal(response["errorCode"], &code)
		if !regexp.MustCompile(`^[A-Za-z0-9_.-]{0,64}$`).MatchString(code) {
			code = ""
		}
		return ValidityResult{}, &Error{Kind: "remote", Operation: "validity", Code: code}
	}
	var data map[string]json.RawMessage
	if json.Unmarshal(response["data"], &data) != nil || data == nil {
		return failure()
	}
	readCode := func(name string) (string, bool) {
		raw := data[name]
		var code string
		if json.Unmarshal(raw, &code) == nil {
			return code, regexp.MustCompile(`^[0-9]+$`).MatchString(code)
		}
		return string(raw), regexp.MustCompile(`^[0-9]+$`).Match(raw)
	}
	status, ok := readCode("estadoCp")
	if !ok {
		return failure()
	}
	ruc, ok := readCode("estadoRuc")
	if !ok {
		return failure()
	}
	address, ok := readCode("condDomiRuc")
	if !ok {
		return failure()
	}
	observations := []string{}
	raw := data["observaciones"]
	if len(raw) == 0 {
		raw = data["Observaciones"]
	}
	if len(raw) != 0 && string(raw) != "null" {
		var entries []json.RawMessage
		if json.Unmarshal(raw, &entries) != nil {
			return failure()
		}
		for _, entry := range entries {
			var value string
			if string(entry) == "null" || json.Unmarshal(entry, &value) != nil {
				return failure()
			}
			observations = append(observations, value)
		}
	}
	return ValidityResult{DocumentStatus: status, IssuerStatus: ruc, IssuerAddressStatus: address, Observations: observations}, nil
}

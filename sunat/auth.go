package sunat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const validityScope = "https://api.sunat.gob.pe/v1/contribuyente/contribuyentes"

type TokenSource interface {
	Token(context.Context) (string, error)
}

type staticToken struct{ value string }

func (s *staticToken) String() string   { return "StaticToken([redacted])" }
func (s *staticToken) GoString() string { return s.String() }
func (s *staticToken) Token(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !validToken(s.value) {
		return "", &Error{Kind: "configuration", Operation: "token"}
	}
	return s.value, nil
}

// StaticToken accepts an already obtained access token without network access.
func StaticToken(value string) TokenSource { return &staticToken{value: value} }

type OAuth2Options struct {
	ClientID     string
	ClientSecret string
	// TokenURL overrides the complete token endpoint; HTTPS or literal loopback only.
	TokenURL   string
	HTTPClient *http.Client
}

func (o OAuth2Options) String() string   { return "OAuth2Options([redacted])" }
func (o OAuth2Options) GoString() string { return o.String() }

// OAuth2TokenSource implements the client_credentials flow for Consulta Integrada.
// It is not the SOL/password authentication flow used by other SUNAT services.
type OAuth2TokenSource struct {
	mu                         sync.Mutex
	clientID, secret, endpoint string
	client                     *http.Client
	token                      string
	expires                    time.Time
	now                        func() time.Time
}

func (s *OAuth2TokenSource) String() string   { return "OAuth2TokenSource([redacted])" }
func (s *OAuth2TokenSource) GoString() string { return s.String() }

func NewOAuth2TokenSource(options OAuth2Options) (*OAuth2TokenSource, error) {
	if strings.TrimSpace(options.ClientID) == "" || strings.TrimSpace(options.ClientSecret) == "" {
		return nil, &Error{Kind: "configuration", Operation: "token"}
	}
	endpoint := options.TokenURL
	if endpoint == "" {
		endpoint = "https://api-seguridad.sunat.gob.pe/v1/clientesextranet/" + url.PathEscape(options.ClientID) + "/oauth2/token/"
	}
	if _, err := checkedEndpoint(endpoint); err != nil {
		return nil, err
	}
	return &OAuth2TokenSource{clientID: options.ClientID, secret: options.ClientSecret, endpoint: endpoint, client: httpClient(options.HTTPClient), now: time.Now}, nil
}

func (s *OAuth2TokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.now().Before(s.expires) {
		return s.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "scope": {validityScope}, "client_id": {s.clientID}, "client_secret": {s.secret}}
	started := s.now()
	body, err := post(ctx, s.client, "token", s.endpoint, "application/x-www-form-urlencoded", "", []byte(form.Encode()))
	if err != nil {
		return "", err
	}
	var response struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if json.Unmarshal(body, &response) != nil || !validToken(response.AccessToken) || !strings.EqualFold(response.TokenType, "Bearer") || response.ExpiresIn <= 0 || response.ExpiresIn > 86400*365 {
		return "", &Error{Kind: "protocol", Operation: "token"}
	}
	ttl := time.Duration(response.ExpiresIn) * time.Second
	skew := ttl / 10
	if skew > 30*time.Second {
		skew = 30 * time.Second
	}
	s.token, s.expires = response.AccessToken, started.Add(ttl-skew)
	return s.token, nil
}

package sunat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedConformance(t *testing.T) {
	var cases struct {
		Query   map[string]json.RawMessage `json:"query"`
		Queries []struct {
			ID    string
			Patch map[string]json.RawMessage
			Valid bool
		} `json:"query_cases"`
		Responses []struct {
			ID       string
			Body     json.RawMessage
			RawBody  string `json:"raw_body"`
			Expected ValidityResult
			Error    string
		} `json:"response_cases"`
	}
	data, err := os.ReadFile("../internal/conformance/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases.Queries {
		t.Run("query/"+c.ID, func(t *testing.T) {
			merged := map[string]json.RawMessage{}
			for k, v := range cases.Query {
				merged[k] = v
			}
			for k, v := range c.Patch {
				merged[k] = v
			}
			body, _ := json.Marshal(merged)
			var query ValidityQuery
			err := json.Unmarshal(body, &query)
			if err == nil {
				err = query.Validate()
			}
			if (err == nil) != c.Valid {
				t.Fatalf("valid=%v, error=%v", c.Valid, err)
			}
		})
	}
	for _, c := range cases.Responses {
		t.Run("response/"+c.ID, func(t *testing.T) {
			body := c.Body
			if c.RawBody != "" {
				body = []byte(c.RawBody)
			}
			result, err := ParseValidityResponse(body)
			if c.Error != "" {
				var sdkError *Error
				if !errors.As(err, &sdkError) || sdkError.Kind != c.Error {
					t.Fatalf("expected %s, got %v", c.Error, err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(result, c.Expected) {
				t.Fatalf("got %#v, %v; want %#v", result, err, c.Expected)
			}
		})
	}
}

func query() ValidityQuery {
	return ValidityQuery{IssuerRUC: "20000000001", DocumentType: "01", Series: "F001", Number: 123, IssuedOn: "2026-11-02", Amount: "123456789012345.67"}
}

const accepted = `{"success":true,"data":{"estadoCp":1,"estadoRuc":"00","condDomiRuc":"00"}}`

func TestHTTPExchangeAndTokenCache(t *testing.T) {
	var tokens atomic.Int32
	var queries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Error("expected POST")
		}
		if r.URL.Path == "/token" {
			tokens.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("scope") != validityScope || r.Form.Get("client_id") != "synthetic-client" || r.Form.Get("client_secret") != "synthetic-secret" {
				t.Error("incorrect token request")
			}
			fmt.Fprint(w, `{"access_token":"synthetic-token","token_type":"Bearer","expires_in":100}`)
			return
		}
		queries.Add(1)
		if r.URL.Path != "/v1/contribuyente/contribuyentes/20000000002/validarcomprobante" {
			t.Error("path must use requester RUC")
		}
		if r.Header.Get("Authorization") != "Bearer synthetic-token" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("incorrect headers")
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Error(err)
		}
		if string(payload["monto"]) != "123456789012345.67" || string(payload["fechaEmision"]) != `"02/11/2026"` || string(payload["numRuc"]) != `"20000000001"` || string(payload["numero"]) != "123" {
			t.Errorf("wrong payload: %s", body)
		}
		fmt.Fprint(w, accepted)
	}))
	defer server.Close()
	source, err := NewOAuth2TokenSource(OAuth2Options{ClientID: "synthetic-client", ClientSecret: "synthetic-secret", TokenURL: server.URL + "/token"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	source.now = func() time.Time { return now }
	client, err := NewValidityClient(ValidityOptions{RequesterRUC: "20000000002", TokenSource: source, APIBaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := client.Check(context.Background(), query()); err != nil {
			t.Fatal(err)
		}
	}
	if tokens.Load() != 1 || queries.Load() != 2 {
		t.Fatal("token not reused")
	}
	now = now.Add(91 * time.Second)
	if _, err := client.Check(context.Background(), query()); err != nil {
		t.Fatal(err)
	}
	if tokens.Load() != 2 {
		t.Fatal("expired token not refreshed")
	}
}

func TestConcurrentTokenRefresh(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"access_token":"test-token","token_type":"Bearer","expires_in":100}`)
	}))
	defer server.Close()
	source, _ := NewOAuth2TokenSource(OAuth2Options{ClientID: "test", ClientSecret: "secret", TokenURL: server.URL})
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := source.Token(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("concurrent refresh made duplicate requests")
	}
}

func TestHTTPFailuresAndRedirects(t *testing.T) {
	for _, status := range []int{302, 401, 403, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "http://127.0.0.1:1/leak")
				w.WriteHeader(status)
				fmt.Fprint(w, "synthetic-secret")
			}))
			defer server.Close()
			client, _ := NewValidityClient(ValidityOptions{RequesterRUC: "20000000002", TokenSource: StaticToken("test-token"), APIBaseURL: server.URL})
			_, err := client.Check(context.Background(), query())
			var e *Error
			if !errors.As(err, &e) || e.HTTPStatus != status || calls.Load() != 1 {
				t.Fatalf("wrong error or retry: %v", err)
			}
			if strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("remote body leaked")
			}
		})
	}
}

func TestResponseBoundAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", maxResponseBytes+1)) }))
	defer server.Close()
	client, _ := NewValidityClient(ValidityOptions{RequesterRUC: "20000000002", TokenSource: StaticToken("token"), APIBaseURL: server.URL})
	_, err := client.Check(context.Background(), query())
	var e *Error
	if !errors.As(err, &e) || e.Kind != "protocol" {
		t.Fatalf("expected bounded response, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Check(ctx, query()); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
}

func TestConfigurationAndValidationBeforeNetwork(t *testing.T) {
	for _, endpoint := range []string{"http://example.com", "https://user:password@example.com", "https://example.com?token=secret"} {
		if _, err := NewValidityClient(ValidityOptions{RequesterRUC: "20000000002", TokenSource: StaticToken("token"), APIBaseURL: endpoint}); err == nil {
			t.Fatal("insecure endpoint allowed")
		}
	}
	client, _ := NewValidityClient(ValidityOptions{RequesterRUC: "20000000002", TokenSource: StaticToken("token"), APIBaseURL: "http://127.0.0.1:1"})
	bad := query()
	bad.Amount = "1e2"
	if _, err := client.Check(context.Background(), bad); err == nil || !strings.Contains(err.Error(), "amount") {
		t.Fatalf("validation should happen before I/O: %v", err)
	}
	if strings.Contains(fmt.Sprintf("%#v", StaticToken("do-not-log")), "do-not-log") {
		t.Fatal("token formatting leaked")
	}
}

func TestMalformedTokenResponses(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"access_token":"x","token_type":"Bearer","expires_in":0}`, `{"access_token":"x","token_type":"Bearer","expires_in":true}`, `{"access_token":"x","token_type":"Other","expires_in":10}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			source, _ := NewOAuth2TokenSource(OAuth2Options{ClientID: "id", ClientSecret: "secret", TokenURL: server.URL})
			_, err := source.Token(context.Background())
			var e *Error
			if !errors.As(err, &e) || e.Kind != "protocol" {
				t.Fatalf("bad token response accepted: %v", err)
			}
		})
	}
}

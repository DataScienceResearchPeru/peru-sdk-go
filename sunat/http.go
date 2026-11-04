package sunat

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 1 << 20

func checkedEndpoint(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", &Error{Kind: "configuration", Operation: "endpoint"}
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return "", &Error{Kind: "configuration", Operation: "endpoint"}
	}
	return strings.TrimRight(raw, "/"), nil
}

func httpClient(client *http.Client) *http.Client {
	var copy http.Client
	if client != nil {
		copy = *client
	}
	if copy.Timeout == 0 {
		copy.Timeout = 10 * time.Second
	}
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}

func post(ctx context.Context, client *http.Client, operation, endpoint, contentType, token string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &Error{Kind: "configuration", Operation: operation}
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &Error{Kind: "transport", Operation: operation}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		kind := "http"
		if res.StatusCode == 401 {
			kind = "authentication"
		}
		if res.StatusCode == 403 {
			kind = "authorization"
		}
		return nil, &Error{Kind: kind, Operation: operation, HTTPStatus: res.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil {
		return nil, &Error{Kind: "transport", Operation: operation}
	}
	if len(data) > maxResponseBytes {
		return nil, &Error{Kind: "protocol", Operation: operation}
	}
	return data, nil
}

func validToken(token string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}
	for _, c := range token {
		if c <= 32 || c >= 127 {
			return false
		}
	}
	return true
}

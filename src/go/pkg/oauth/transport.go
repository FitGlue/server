package oauth

import (
	"fmt"
	"log/slog"
	"net/http"
)

// Transport is an http.RoundTripper that authenticates all requests
// using the provided TokenSource.
type Transport struct {
	// Source supplies the token to be used.
	Source TokenSource

	// Base is the base RoundTripper used to make the actual HTTP requests.
	// If nil, http.DefaultTransport is used.
	Base http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 0. Get Base Transport
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	// 1. Get Token (Proactive check happens here)
	ctx := req.Context()
	token, err := t.Source.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("oauth: cannot get token: %w", err)
	}

	// 2. Clone Request and Set Header
	req2 := cloneRequest(req)
	req2.Header.Set("Authorization", "Bearer "+token.AccessToken)

	// 3. Execute Request
	resp, err := base.RoundTrip(req2)
	if err != nil {
		return nil, err
	}

	// 4. Reactive Retry (401)
	if resp.StatusCode == http.StatusUnauthorized {
		// Drain body to allow connection reuse
		resp.Body.Close()

		slog.Warn("Got 401 Unauthorized, attempting force refresh", "url", req.URL.String())

		// Force Refresh
		token, err = t.Source.ForceRefresh(ctx)
		if err != nil {
			return nil, fmt.Errorf("oauth: force refresh failed: %w", err)
		}

		// Update Header
		req2.Header.Set("Authorization", "Bearer "+token.AccessToken)

		// Retry Request
		return base.RoundTrip(req2)
	}

	return resp, nil
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

// NewHTTPClient creates an HTTP client with OAuth transport
func NewHTTPClient(source TokenSource) *http.Client {
	return &http.Client{
		Transport: &Transport{
			Source: source,
		},
	}
}

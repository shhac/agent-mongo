// Package oidctest provides a mock identity provider for exercising the device
// authorization grant.
//
// It is a real HTTP server rather than a stubbed transport, so tests cover the
// actual request encoding, headers and status handling — the parts most likely
// to differ from a real provider. It lives outside _test.go because the flows
// that use it span packages: the protocol client, the credential store's
// refresh, and the CLI all need the same provider.
package oidctest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// IDP is a mock OpenID provider. The exported fields are the knobs a test sets
// before driving it; the counters record what the client actually did.
type IDP struct {
	// PendingPolls is how many times the token endpoint answers
	// authorization_pending before returning a token.
	PendingPolls int
	// SlowDownPolls is how many times it first asks the client to back off.
	SlowDownPolls int
	// TokenError, when set, is returned by the token endpoint instead of a
	// token, for as long as it is set.
	TokenError string
	// OmitDeviceEndpoint drops the device endpoint from discovery, standing in
	// for a provider that does not offer the grant.
	OmitDeviceEndpoint bool
	// AccessToken and RefreshToken are what a successful exchange returns.
	AccessToken  string
	RefreshToken string
	// ExpiresIn is the token lifetime in seconds; zero omits it entirely.
	ExpiresIn int

	// Polls counts token-endpoint requests; Refreshes counts the subset using
	// the refresh grant.
	Polls     int
	Refreshes int
	// DeviceForm and LastForm are the decoded request bodies, kept apart
	// because the token endpoint would otherwise overwrite what the device
	// request carried — the scopes among them.
	DeviceForm url.Values
	LastForm   url.Values

	server *httptest.Server
}

// New starts a mock provider and registers its shutdown with the test.
func New(t *testing.T) *IDP {
	t.Helper()
	idp := &IDP{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresIn:    3600,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		doc := map[string]string{
			"issuer":         idp.server.URL,
			"token_endpoint": idp.server.URL + "/token",
		}
		if !idp.OmitDeviceEndpoint {
			doc["device_authorization_endpoint"] = idp.server.URL + "/device"
		}
		writeJSON(w, http.StatusOK, doc)
	})
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		idp.DeviceForm = readForm(t, r)
		idp.LastForm = idp.DeviceForm
		writeJSON(w, http.StatusOK, map[string]any{
			"device_code":               "dev-code",
			"user_code":                 "WDJB-MJHT",
			"verification_uri":          idp.server.URL + "/activate",
			"verification_uri_complete": idp.server.URL + "/activate?user_code=WDJB-MJHT",
			"expires_in":                600,
			"interval":                  5,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		idp.Polls++
		idp.LastForm = readForm(t, r)
		if idp.LastForm.Get("grant_type") == "refresh_token" {
			idp.Refreshes++
		}

		if idp.TokenError != "" {
			// 400 is what RFC 6749 specifies and what most providers send.
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": idp.TokenError})
			return
		}
		if idp.SlowDownPolls > 0 {
			idp.SlowDownPolls--
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slow_down"})
			return
		}
		if idp.PendingPolls > 0 {
			idp.PendingPolls--
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "authorization_pending"})
			return
		}

		body := map[string]any{"access_token": idp.AccessToken}
		if idp.RefreshToken != "" {
			body["refresh_token"] = idp.RefreshToken
		}
		if idp.ExpiresIn > 0 {
			body["expires_in"] = idp.ExpiresIn
		}
		writeJSON(w, http.StatusOK, body)
	})

	// TLS, because a real issuer is https: a caller has to be handed Client()
	// to trust the certificate, which is the seam this exists to exercise.
	idp.server = httptest.NewTLSServer(mux)
	t.Cleanup(idp.server.Close)
	return idp
}

// Issuer is the provider's issuer URL, for discovery.
func (i *IDP) Issuer() string { return i.server.URL }

// Client is an HTTP client that trusts this provider's certificate.
func (i *IDP) Client() *http.Client { return i.server.Client() }

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func readForm(t *testing.T, r *http.Request) url.Values {
	t.Helper()
	if err := r.ParseForm(); err != nil {
		t.Fatalf("parse form: %v", err)
	}
	return r.PostForm
}

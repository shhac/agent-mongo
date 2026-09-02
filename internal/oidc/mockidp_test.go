package oidc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// mockIDP is an identity provider that speaks just enough of the device
// authorization grant to drive the client.
//
// It is a real HTTP server rather than a stubbed transport so the tests
// exercise the actual request encoding, headers and status handling — the parts
// most likely to be wrong against a real provider.
type mockIDP struct {
	server *httptest.Server

	// pendingPolls is how many times the token endpoint answers
	// authorization_pending before succeeding.
	pendingPolls int
	// slowDownPolls is how many times it asks the client to back off first.
	slowDownPolls int
	// tokenError, when set, is returned by the token endpoint instead of a
	// token, for as long as it is set.
	tokenError string

	// omitDeviceEndpoint drops the device endpoint from discovery, standing in
	// for a provider that does not support the grant.
	omitDeviceEndpoint bool

	// polls counts token-endpoint requests, and refreshes counts the subset
	// that used the refresh grant.
	polls     int
	refreshes int
	// lastForm is the most recent decoded request body, for asserting what the
	// client actually sent.
	lastForm url.Values
}

func newMockIDP(t *testing.T) *mockIDP {
	t.Helper()
	idp := &mockIDP{}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		doc := map[string]string{
			"issuer":         idp.server.URL,
			"token_endpoint": idp.server.URL + "/token",
		}
		if !idp.omitDeviceEndpoint {
			doc["device_authorization_endpoint"] = idp.server.URL + "/device"
		}
		writeJSON(w, http.StatusOK, doc)
	})
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		idp.lastForm = readForm(t, r)
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
		idp.polls++
		idp.lastForm = readForm(t, r)
		if idp.lastForm.Get("grant_type") == "refresh_token" {
			idp.refreshes++
		}

		if idp.tokenError != "" {
			// 400 is what RFC 6749 specifies, and what most providers send.
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": idp.tokenError})
			return
		}
		if idp.slowDownPolls > 0 {
			idp.slowDownPolls--
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slow_down"})
			return
		}
		if idp.pendingPolls > 0 {
			idp.pendingPolls--
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "authorization_pending"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token":  "access-" + idp.lastForm.Get("grant_type"),
			"refresh_token": "refresh-1",
			"expires_in":    3600,
		})
	})

	// TLS, because a real issuer is https and the client has to be handed the
	// server's own client to trust it — the seam this exists to exercise.
	idp.server = httptest.NewTLSServer(mux)
	t.Cleanup(idp.server.Close)
	return idp
}

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

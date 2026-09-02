package oidc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A zero Client is usable: the seams exist for tests, not as required
// configuration.
func TestZeroClientHasWorkingDefaults(t *testing.T) {
	var client Client
	if client.httpClient() != http.DefaultClient {
		t.Error("httpClient() did not fall back to http.DefaultClient")
	}
	if client.now().IsZero() {
		t.Error("now() returned the zero time")
	}
	// Proves sleep() is wired to the real one without waiting on it.
	start := time.Now()
	client.sleep(time.Millisecond)
	if time.Since(start) < time.Millisecond {
		t.Error("sleep() did not sleep")
	}
}

func TestOAuthErrorMessage(t *testing.T) {
	withDescription := &Error{Code: "invalid_grant", Description: "token is expired"}
	if got := withDescription.Error(); got != "invalid_grant: token is expired" {
		t.Errorf("Error() = %q", got)
	}
	bare := &Error{Code: "slow_down"}
	if got := bare.Error(); got != "slow_down" {
		t.Errorf("Error() = %q", got)
	}
}

// serving stands up a provider whose two endpoints return exactly what a test
// dictates, for the responses a well-behaved mock will not produce.
func serving(t *testing.T, device, token string, status int) (*Client, Provider) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/device", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(device))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(token))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := &Client{
		HTTP:  server.Client(),
		Now:   func() time.Time { return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC) },
		Sleep: func(time.Duration) {},
	}
	return client, Provider{
		DeviceAuthorizationEndpoint: server.URL + "/device",
		TokenEndpoint:               server.URL + "/token",
	}
}

func TestStartDeviceAuthRejectsAnIncompleteResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"no device code", `{"user_code":"A","verification_uri":"https://idp/activate"}`},
		{"no user code", `{"device_code":"d","verification_uri":"https://idp/activate"}`},
		{"no verification uri", `{"device_code":"d","user_code":"A"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, provider := serving(t, tt.body, "", http.StatusOK)
			_, err := client.StartDeviceAuth(context.Background(), provider, "client-1", nil)
			if err == nil {
				t.Fatal("an incomplete device authorization response was accepted")
			}
			if !strings.Contains(err.Error(), "code or verification URL") {
				t.Errorf("error = %q, want it to name what was missing", err)
			}
		})
	}
}

// RFC 8628 makes both optional, and a provider that omits them must not produce
// a zero interval (a hot poll loop) or an already-expired code.
func TestStartDeviceAuthDefaultsIntervalAndExpiry(t *testing.T) {
	body := `{"device_code":"d","user_code":"A","verification_uri":"https://idp/activate"}`
	client, provider := serving(t, body, "", http.StatusOK)

	auth, err := client.StartDeviceAuth(context.Background(), provider, "client-1", nil)
	if err != nil {
		t.Fatalf("StartDeviceAuth: %v", err)
	}
	if auth.Interval != 5*time.Second {
		t.Errorf("Interval = %s, want the RFC's 5s default", auth.Interval)
	}
	if want := client.now().Add(10 * time.Minute); !auth.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %s, want a default lifetime", auth.ExpiresAt)
	}
}

func TestExchangeRejectsAResponseWithNoAccessToken(t *testing.T) {
	client, provider := serving(t, "", `{"refresh_token":"r"}`, http.StatusOK)
	if _, err := client.Refresh(context.Background(), provider, "client-1", "r0"); err == nil {
		t.Fatal("a token response with no access token was accepted")
	}
}

// A token with no expires_in leaves ExpiresAt zero rather than inventing one;
// the caller treats an unknown expiry as "ask the server".
func TestExchangeLeavesAnUnknownExpiryZero(t *testing.T) {
	client, provider := serving(t, "", `{"access_token":"a"}`, http.StatusOK)
	token, err := client.Refresh(context.Background(), provider, "client-1", "r0")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !token.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt = %s, want the zero time", token.ExpiresAt)
	}
}

func TestNonJSONResponseIsReported(t *testing.T) {
	client, provider := serving(t, "", "<html>gateway error</html>", http.StatusOK)
	_, err := client.Refresh(context.Background(), provider, "client-1", "r0")
	if err == nil {
		t.Fatal("a non-JSON response was accepted")
	}
	if !strings.Contains(err.Error(), "decoding") {
		t.Errorf("error = %q, want it to say the response could not be decoded", err)
	}
}

// A failing status with no OAuth error body still has to surface, rather than
// being decoded into a zero-valued success.
func TestFailingStatusWithoutAnOAuthBodyIsReported(t *testing.T) {
	client, provider := serving(t, "", `{}`, http.StatusInternalServerError)
	_, err := client.Refresh(context.Background(), provider, "client-1", "r0")
	if err == nil {
		t.Fatal("a 500 was accepted")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to carry the status", err)
	}
}

// Providers differ on the status carrying authorization_pending; the client
// must read the OAuth error whichever it is.
func TestOAuthErrorIsReadFromA200(t *testing.T) {
	client, provider := serving(t, "", `{"error":"access_denied"}`, http.StatusOK)
	auth := DeviceAuth{DeviceCode: "d", Interval: time.Second, ExpiresAt: client.now().Add(time.Minute)}
	if _, err := client.PollForToken(context.Background(), provider, "client-1", auth); !errors.Is(err, ErrDenied) {
		t.Errorf("error = %v, want ErrDenied", err)
	}
}

// An unrecognised OAuth error stops the loop rather than being treated as
// "keep waiting".
func TestUnknownOAuthErrorStopsPolling(t *testing.T) {
	client, provider := serving(t, "", `{"error":"invalid_client","error_description":"unknown client"}`, http.StatusBadRequest)
	auth := DeviceAuth{DeviceCode: "d", Interval: time.Second, ExpiresAt: client.now().Add(time.Minute)}
	_, err := client.PollForToken(context.Background(), provider, "client-1", auth)
	var oauthErr *Error
	if !errors.As(err, &oauthErr) || oauthErr.Code != "invalid_client" {
		t.Fatalf("error = %v, want the provider's invalid_client", err)
	}
	if !strings.Contains(err.Error(), "unknown client") {
		t.Errorf("error = %q, want the provider's description", err)
	}
}

func TestDiscoverReportsAnUnreachableIssuer(t *testing.T) {
	client := &Client{HTTP: &http.Client{Timeout: time.Second}}
	if _, err := client.Discover(context.Background(), "https://127.0.0.1:1/idp"); err == nil {
		t.Fatal("an unreachable issuer was accepted")
	}
}

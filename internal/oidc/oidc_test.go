package oidc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shhac/agent-mongo/internal/oidc/oidctest"
)

// testClient points at the mock provider and records what it would have slept,
// so polling tests cost no wall time and can assert the backoff.
func testClient(t *testing.T, idp *oidctest.IDP, at time.Time) (*Client, *[]time.Duration) {
	t.Helper()
	var slept []time.Duration
	return &Client{
		HTTP:  idp.Client(),
		Now:   func() time.Time { return at },
		Sleep: func(d time.Duration) { slept = append(slept, d) },
	}, &slept
}

func TestDiscover(t *testing.T) {
	idp := oidctest.New(t)
	client, _ := testClient(t, idp, time.Now())

	provider, err := client.Discover(context.Background(), idp.Issuer())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if provider.TokenEndpoint != idp.Issuer()+"/token" {
		t.Errorf("TokenEndpoint = %q", provider.TokenEndpoint)
	}
	if provider.DeviceAuthorizationEndpoint != idp.Issuer()+"/device" {
		t.Errorf("DeviceAuthorizationEndpoint = %q", provider.DeviceAuthorizationEndpoint)
	}
}

// A trailing slash on the issuer must not produce a double slash in the
// discovery URL, which some providers 404.
func TestDiscoverTolatesATrailingSlash(t *testing.T) {
	idp := oidctest.New(t)
	client, _ := testClient(t, idp, time.Now())

	if _, err := client.Discover(context.Background(), idp.Issuer()+"/"); err != nil {
		t.Fatalf("Discover: %v", err)
	}
}

// A provider that does not offer the device grant has to be reported as such,
// rather than failing later with an empty endpoint.
func TestDiscoverRejectsAProviderWithoutTheDeviceGrant(t *testing.T) {
	idp := oidctest.New(t)
	idp.OmitDeviceEndpoint = true
	client, _ := testClient(t, idp, time.Now())

	_, err := client.Discover(context.Background(), idp.Issuer())
	if err == nil {
		t.Fatal("a provider with no device endpoint was accepted")
	}
	if !strings.Contains(err.Error(), "device authorization grant") {
		t.Errorf("error = %q, want it to name what is missing", err)
	}
}

func TestStartDeviceAuth(t *testing.T) {
	idp := oidctest.New(t)
	frozen := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	client, _ := testClient(t, idp, frozen)

	provider, err := client.Discover(context.Background(), idp.Issuer())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	auth, err := client.StartDeviceAuth(context.Background(), provider, "client-1", []string{"openid", "offline_access"})
	if err != nil {
		t.Fatalf("StartDeviceAuth: %v", err)
	}

	if auth.UserCode != "WDJB-MJHT" {
		t.Errorf("UserCode = %q", auth.UserCode)
	}
	if auth.VerificationURIComplete == "" {
		t.Error("VerificationURIComplete was dropped; it is the link a person can just open")
	}
	if auth.Interval != 5*time.Second {
		t.Errorf("Interval = %s, want the provider's 5s", auth.Interval)
	}
	if want := frozen.Add(10 * time.Minute); !auth.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %s, want %s", auth.ExpiresAt, want)
	}
	if got := idp.DeviceForm.Get("scope"); got != "openid offline_access" {
		t.Errorf("scope = %q, want the scopes space-joined", got)
	}
	if got := idp.DeviceForm.Get("client_id"); got != "client-1" {
		t.Errorf("client_id = %q", got)
	}
}

func TestPollForTokenWaitsOutAuthorizationPending(t *testing.T) {
	idp := oidctest.New(t)
	idp.PendingPolls = 2
	client, slept := testClient(t, idp, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	token := pollToken(t, client, idp)
	if token.AccessToken == "" {
		t.Fatal("no access token")
	}
	if idp.Polls != 3 {
		t.Errorf("polls = %d, want 3 (two pending, then the token)", idp.Polls)
	}
	for _, d := range *slept {
		if d != 5*time.Second {
			t.Errorf("slept %s between polls, want the provider's 5s interval", d)
		}
	}
}

// RFC 8628 says add five seconds to the interval each time the provider asks
// the client to slow down.
func TestPollForTokenBacksOffOnSlowDown(t *testing.T) {
	idp := oidctest.New(t)
	idp.SlowDownPolls = 2
	client, slept := testClient(t, idp, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	pollToken(t, client, idp)

	want := []time.Duration{5 * time.Second, 10 * time.Second, 15 * time.Second}
	if len(*slept) != len(want) {
		t.Fatalf("slept %v, want %v", *slept, want)
	}
	for i, d := range want {
		if (*slept)[i] != d {
			t.Errorf("sleep %d = %s, want %s", i, (*slept)[i], d)
		}
	}
}

func TestPollForTokenReportsDenial(t *testing.T) {
	idp := oidctest.New(t)
	idp.TokenError = "access_denied"
	client, _ := testClient(t, idp, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	_, err := pollTokenErr(t, client, idp)
	if !errors.Is(err, ErrDenied) {
		t.Errorf("error = %v, want ErrDenied", err)
	}
}

func TestPollForTokenReportsAnExpiredCode(t *testing.T) {
	idp := oidctest.New(t)
	idp.TokenError = "expired_token"
	client, _ := testClient(t, idp, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	_, err := pollTokenErr(t, client, idp)
	if !errors.Is(err, ErrCodeExpired) {
		t.Errorf("error = %v, want ErrCodeExpired", err)
	}
}

// Polling stops when the code's own lifetime runs out, rather than hammering a
// dead code until something else gives up.
func TestPollForTokenStopsWhenTheCodeExpires(t *testing.T) {
	idp := oidctest.New(t)
	idp.PendingPolls = 1000
	frozen := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

	var slept []time.Duration
	clock := frozen
	client := &Client{
		HTTP: idp.Client(),
		Now:  func() time.Time { return clock },
		// Advancing the clock as the client sleeps is what makes the code
		// expire without the test taking ten real minutes.
		Sleep: func(d time.Duration) { slept = append(slept, d); clock = clock.Add(d) },
	}

	_, err := pollTokenErr(t, client, idp)
	if !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("error = %v, want ErrCodeExpired", err)
	}
	if idp.Polls > 130 {
		t.Errorf("polled %d times; the client should stop at the code's expiry", idp.Polls)
	}
}

func TestPollForTokenHonoursContextCancellation(t *testing.T) {
	idp := oidctest.New(t)
	idp.PendingPolls = 1000
	client, _ := testClient(t, idp, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC))

	ctx, cancel := context.WithCancel(context.Background())
	provider, auth := startAuth(t, client, idp)
	cancel()

	if _, err := client.PollForToken(ctx, provider, "client-1", auth); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestRefresh(t *testing.T) {
	idp := oidctest.New(t)
	frozen := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	client, _ := testClient(t, idp, frozen)

	provider, err := client.Discover(context.Background(), idp.Issuer())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	token, err := client.Refresh(context.Background(), provider, "client-1", "refresh-0")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if idp.Refreshes != 1 {
		t.Errorf("refreshes = %d, want 1", idp.Refreshes)
	}
	if got := idp.LastForm.Get("refresh_token"); got != "refresh-0" {
		t.Errorf("refresh_token sent = %q", got)
	}
	if token.AccessToken == "" {
		t.Error("no access token")
	}
	if want := frozen.Add(time.Hour); !token.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %s, want %s", token.ExpiresAt, want)
	}
}

// A rejected refresh token is the one case that needs a human back at a
// terminal, so it gets its own error rather than a generic OAuth failure.
func TestRefreshReportsARejectedToken(t *testing.T) {
	idp := oidctest.New(t)
	idp.TokenError = "invalid_grant"
	client, _ := testClient(t, idp, time.Now())

	provider, err := client.Discover(context.Background(), idp.Issuer())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if _, err := client.Refresh(context.Background(), provider, "client-1", "stale"); !errors.Is(err, ErrRefreshRejected) {
		t.Errorf("error = %v, want ErrRefreshRejected", err)
	}
}

func startAuth(t *testing.T, client *Client, idp *oidctest.IDP) (Provider, DeviceAuth) {
	t.Helper()
	provider, err := client.Discover(context.Background(), idp.Issuer())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	auth, err := client.StartDeviceAuth(context.Background(), provider, "client-1", nil)
	if err != nil {
		t.Fatalf("StartDeviceAuth: %v", err)
	}
	return provider, auth
}

func pollToken(t *testing.T, client *Client, idp *oidctest.IDP) Token {
	t.Helper()
	token, err := pollTokenErr(t, client, idp)
	if err != nil {
		t.Fatalf("PollForToken: %v", err)
	}
	return token
}

func pollTokenErr(t *testing.T, client *Client, idp *oidctest.IDP) (Token, error) {
	t.Helper()
	provider, auth := startAuth(t, client, idp)
	return client.PollForToken(context.Background(), provider, "client-1", auth)
}

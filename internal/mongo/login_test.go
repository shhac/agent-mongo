package mongo

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/oidc"
	"github.com/shhac/agent-mongo/internal/oidc/oidctest"
)

// The conversion between the driver's OIDC types and agent-mongo's is the one
// part of a login that never runs outside a live OIDC-configured deployment, so
// it is exercised directly rather than left to integration.
func TestDeviceLoginCallbackConvertsTheDriversTypes(t *testing.T) {
	idp := useMockIDP(t, oidctest.New(t))

	var prompted credential.DevicePrompt
	login := &deviceLogin{
		host:   "c0.abc.mongodb.net",
		prompt: func(p credential.DevicePrompt) { prompted = p },
	}

	cred, err := login.callback(context.Background(), &options.OIDCArgs{
		IDPInfo: &options.IDPInfo{
			Issuer:        idp.Issuer(),
			ClientID:      "client-1",
			RequestScopes: []string{"openid", "offline_access"},
		},
	})
	if err != nil {
		t.Fatalf("callback: %v", err)
	}

	if prompted.UserCode == "" {
		t.Error("the person was never given a code")
	}
	if cred.AccessToken != "access-token" {
		t.Errorf("AccessToken = %q", cred.AccessToken)
	}
	// The driver wants pointers for the optional fields, and a nil is how "not
	// supplied" is expressed.
	if cred.RefreshToken == nil || *cred.RefreshToken != "refresh-token" {
		t.Errorf("RefreshToken = %v, want the provider's", cred.RefreshToken)
	}
	if cred.ExpiresAt == nil {
		t.Fatal("ExpiresAt was dropped")
	}
	// The session is what gets stored, and it must carry what a refresh needs.
	if login.session.Issuer != idp.Issuer() || login.session.ClientID != "client-1" {
		t.Errorf("session = %+v, want the issuer and client recorded", login.session)
	}
	if login.session.Host != "c0.abc.mongodb.net" {
		t.Errorf("Host = %q, want the deployment it was obtained for", login.session.Host)
	}
	if got := idp.DeviceForm.Get("scope"); got != "openid offline_access" {
		t.Errorf("scope = %q, want the server's scopes forwarded", got)
	}
}

// A provider that returns no refresh token or no expiry must leave those nil
// rather than sending the driver a pointer to an empty value.
func TestDeviceLoginCallbackOmitsAbsentOptionalFields(t *testing.T) {
	provider := oidctest.New(t)
	provider.RefreshToken = ""
	provider.ExpiresIn = 0
	idp := useMockIDP(t, provider)

	login := &deviceLogin{host: "c0.abc.mongodb.net", prompt: func(credential.DevicePrompt) {}}
	cred, err := login.callback(context.Background(), &options.OIDCArgs{
		IDPInfo: &options.IDPInfo{Issuer: idp.Issuer(), ClientID: "client-1"},
	})
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	if cred.RefreshToken != nil {
		t.Errorf("RefreshToken = %v, want nil when the provider sent none", *cred.RefreshToken)
	}
	if cred.ExpiresAt != nil {
		t.Errorf("ExpiresAt = %v, want nil when the provider stated none", *cred.ExpiresAt)
	}
}

// A deployment that does not send IdP metadata leaves nothing to discover, and
// that has to be an error rather than a nil dereference.
func TestDeviceLoginCallbackHandlesMissingIDPInfo(t *testing.T) {
	useMockIDP(t, oidctest.New(t))

	login := &deviceLogin{host: "c0.abc.mongodb.net", prompt: func(credential.DevicePrompt) {}}
	for _, args := range []*options.OIDCArgs{nil, {}} {
		if _, err := login.callback(context.Background(), args); err == nil {
			t.Errorf("callback(%v) succeeded with no identity provider to talk to", args)
		}
	}
}

func TestDeviceLoginCallbackReportsDenial(t *testing.T) {
	provider := oidctest.New(t)
	provider.TokenError = "access_denied"
	idp := useMockIDP(t, provider)

	login := &deviceLogin{host: "c0.abc.mongodb.net", prompt: func(credential.DevicePrompt) {}}
	_, err := login.callback(context.Background(), &options.OIDCArgs{
		IDPInfo: &options.IDPInfo{Issuer: idp.Issuer(), ClientID: "client-1"},
	})
	if err == nil {
		t.Fatal("a declined login succeeded")
	}
	if !strings.Contains(err.Error(), "declined") {
		t.Errorf("error = %q, want it to say the request was declined", err)
	}
	if login.session.AccessToken != "" {
		t.Error("a session was recorded despite the failure")
	}
}

// The login path uses the same pool policy as every other connection, rather
// than restating the numbers.
func TestBaseClientOptionsAreShared(t *testing.T) {
	opts := baseClientOptions("mongodb://localhost:27017/app")
	if opts.MaxPoolSize == nil || *opts.MaxPoolSize != 1 {
		t.Errorf("MaxPoolSize = %v, want 1", opts.MaxPoolSize)
	}
	if opts.MinPoolSize == nil || *opts.MinPoolSize != 0 {
		t.Errorf("MinPoolSize = %v, want 0", opts.MinPoolSize)
	}
	if opts.ServerSelectionTimeout == nil || *opts.ServerSelectionTimeout != 10*time.Second {
		t.Errorf("ServerSelectionTimeout = %v, want 10s", opts.ServerSelectionTimeout)
	}
}

// useMockIDP points the credential package's device flow at a mock provider
// for the duration of a test.
func useMockIDP(t *testing.T, idp *oidctest.IDP) *oidctest.IDP {
	t.Helper()
	at := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	restore := credential.UseIdentityProvider(&oidc.Client{
		HTTP:  idp.Client(),
		Now:   func() time.Time { return at },
		Sleep: func(time.Duration) {},
	}, func() time.Time { return at })
	t.Cleanup(restore)
	return idp
}

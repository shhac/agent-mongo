package credential

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/oidc"
	"github.com/shhac/agent-mongo/internal/oidc/oidctest"
	"github.com/shhac/agent-mongo/internal/testutil"
)

const testHost = "c0.abc.mongodb.net"

// useMockIDP points the device flow at a mock provider for the duration of a
// test, and pins the clock so expiry is decided rather than raced.
func useMockIDP(t *testing.T, at time.Time) *oidctest.IDP {
	t.Helper()
	idp := oidctest.New(t)
	fixedClock(t, at)

	t.Cleanup(UseIdentityProvider(&oidc.Client{
		HTTP:  idp.Client(),
		Now:   func() time.Time { return at },
		Sleep: func(time.Duration) {},
	}, func() time.Time { return at }))
	return idp
}

func deviceCredential(t *testing.T, session Session) config.Credential {
	t.Helper()
	encoded, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	return config.Credential{
		Kind:    config.KindOIDC,
		Flow:    &config.Flow{Type: config.FlowDevice},
		Session: string(encoded),
	}
}

func deviceResolution(t *testing.T, session Session) Resolution {
	t.Helper()
	return Resolution{Alias: "corp", Kind: config.KindOIDC, Credential: deviceCredential(t, session)}
}

func liveSession(issuer string) Session {
	return Session{
		AccessToken:  "live-token",
		RefreshToken: "refresh-0",
		ExpiresAt:    time.Date(2026, 9, 2, 13, 0, 0, 0, time.UTC),
		Issuer:       issuer,
		ClientID:     "client-1",
		Host:         testHost,
	}
}

var frozen = time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

func TestDeviceFlowUsesAValidSessionWithoutTalkingToTheProvider(t *testing.T) {
	idp := useMockIDP(t, frozen)
	res := deviceResolution(t, liveSession(idp.Issuer()))

	token, err := res.AccessToken(context.Background(), testHost)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token != "live-token" {
		t.Errorf("token = %q, want the stored one", token)
	}
	if idp.Polls != 0 {
		t.Errorf("polled the provider %d times for a session that was still good", idp.Polls)
	}
}

// The driver never hands a per-process CLI its refresh token and never looks at
// expiry, so renewal is entirely agent-mongo's job.
func TestDeviceFlowRefreshesAnExpiredSession(t *testing.T) {
	testutil.IsolateConfig(t)
	idp := useMockIDP(t, frozen)
	idp.AccessToken = "renewed-token"
	idp.RefreshToken = "refresh-1"

	session := liveSession(idp.Issuer())
	session.ExpiresAt = frozen.Add(-time.Minute)
	cred := deviceCredential(t, session)
	testutil.StageCredential(t, "corp", cred)

	res := Resolution{Alias: "corp", Kind: config.KindOIDC, Credential: cred}
	token, err := res.AccessToken(context.Background(), testHost)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token != "renewed-token" {
		t.Errorf("token = %q, want the renewed one", token)
	}
	if idp.Refreshes != 1 {
		t.Errorf("refreshes = %d, want 1", idp.Refreshes)
	}

	// Persisted, or the next command refreshes again.
	stored, err := Resolve("corp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	saved, err := decodeSession("corp", stored.Credential.Session)
	if err != nil {
		t.Fatalf("decodeSession: %v", err)
	}
	if saved.AccessToken != "renewed-token" || saved.RefreshToken != "refresh-1" {
		t.Errorf("stored session = %+v, want the renewed tokens", saved)
	}
}

// A token about to expire is refreshed early: one that passes the check can
// still die between the callback returning and the server reading it.
func TestDeviceFlowRefreshesWithinTheLeeway(t *testing.T) {
	testutil.IsolateConfig(t)
	idp := useMockIDP(t, frozen)

	session := liveSession(idp.Issuer())
	session.ExpiresAt = frozen.Add(refreshLeeway / 2)
	cred := deviceCredential(t, session)
	testutil.StageCredential(t, "corp", cred)

	res := Resolution{Alias: "corp", Kind: config.KindOIDC, Credential: cred}
	if _, err := res.AccessToken(context.Background(), testHost); err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if idp.Refreshes != 1 {
		t.Errorf("refreshes = %d, want the token renewed before it expired", idp.Refreshes)
	}
}

// A provider that returns no new refresh token means the old one still stands.
// Dropping it would end the session at the next renewal.
func TestDeviceFlowKeepsTheOldRefreshToken(t *testing.T) {
	testutil.IsolateConfig(t)
	idp := useMockIDP(t, frozen)
	idp.RefreshToken = ""

	session := liveSession(idp.Issuer())
	session.ExpiresAt = frozen.Add(-time.Minute)
	cred := deviceCredential(t, session)
	testutil.StageCredential(t, "corp", cred)

	res := Resolution{Alias: "corp", Kind: config.KindOIDC, Credential: cred}
	if _, err := res.AccessToken(context.Background(), testHost); err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	stored, _ := Resolve("corp")
	saved, _ := decodeSession("corp", stored.Credential.Session)
	if saved.RefreshToken != "refresh-0" {
		t.Errorf("RefreshToken = %q, want the original kept", saved.RefreshToken)
	}
}

// A refused refresh token is the one case needing a person back at a terminal,
// so it says so rather than reporting a transient failure.
func TestDeviceFlowReportsARejectedRefresh(t *testing.T) {
	idp := useMockIDP(t, frozen)
	idp.TokenError = "invalid_grant"

	session := liveSession(idp.Issuer())
	session.ExpiresAt = frozen.Add(-time.Minute)
	res := deviceResolution(t, session)

	_, err := res.AccessToken(context.Background(), testHost)
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("error = %v, want ErrSessionExpired", err)
	}
	if !strings.Contains(err.Error(), "corp") {
		t.Errorf("error = %q, want it to name the credential", err)
	}
}

// A provider that is merely unreachable is worth retrying, and must not be
// reported as a session that needs a fresh login.
func TestDeviceFlowReportsAnUnreachableProviderAsRetryable(t *testing.T) {
	fixedClock(t, frozen)
	t.Cleanup(UseIdentityProvider(&oidc.Client{Now: func() time.Time { return frozen }}, nil))

	session := liveSession("https://127.0.0.1:1/idp")
	session.ExpiresAt = frozen.Add(-time.Minute)
	res := deviceResolution(t, session)

	_, err := res.AccessToken(context.Background(), testHost)
	if !errors.Is(err, ErrRefreshFailed) {
		t.Fatalf("error = %v, want ErrRefreshFailed", err)
	}
	if errors.Is(err, ErrSessionExpired) {
		t.Error("an unreachable provider was reported as an expired session")
	}
}

func TestDeviceFlowRequiresALogin(t *testing.T) {
	fixedClock(t, frozen)
	res := Resolution{
		Alias:      "corp",
		Kind:       config.KindOIDC,
		Credential: config.Credential{Flow: &config.Flow{Type: config.FlowDevice}},
	}
	err := getToken(res, testHost)
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("error = %v, want ErrNotLoggedIn", err)
	}
	if !strings.Contains(errHint(t, err), "credential login corp") {
		t.Errorf("hint = %q, want the login command", errHint(t, err))
	}
}

// An expired session with no refresh token at all needs a person, and says so
// differently from one that was never logged in.
func TestDeviceFlowReportsAnUnrenewableSession(t *testing.T) {
	fixedClock(t, frozen)
	session := liveSession("https://idp.example.com")
	session.ExpiresAt = frozen.Add(-time.Minute)
	session.RefreshToken = ""

	if err := getToken(deviceResolution(t, session), testHost); !errors.Is(err, ErrSessionExpired) {
		t.Errorf("error = %v, want ErrSessionExpired", err)
	}
}

func TestDeviceFlowRejectsACorruptSession(t *testing.T) {
	fixedClock(t, frozen)
	res := Resolution{
		Alias: "corp",
		Kind:  config.KindOIDC,
		Credential: config.Credential{
			Flow:    &config.Flow{Type: config.FlowDevice},
			Session: "not json",
		},
	}
	if err := getToken(res, testHost); !errors.Is(err, ErrSessionExpired) {
		t.Errorf("error = %v, want a corrupt session to read as needing a fresh login", err)
	}
}

// The driver binds a token to nothing and an agent can point a connection
// wherever it likes, so this is the check that makes keeping a session safe.
func TestDeviceFlowRefusesToSendASessionToAnotherHost(t *testing.T) {
	idp := useMockIDP(t, frozen)
	res := deviceResolution(t, liveSession(idp.Issuer()))

	err := getToken(res, "evil.example.com")
	if !errors.Is(err, ErrSessionHostMismatch) {
		t.Fatalf("error = %v, want ErrSessionHostMismatch", err)
	}
	for _, want := range []string{testHost, "evil.example.com"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %q", err, want)
		}
	}
	if idp.Polls != 0 {
		t.Error("the provider was contacted despite the host mismatch")
	}
}

// Host comparison is case-insensitive, because DNS is.
func TestDeviceFlowHostBindingIgnoresCase(t *testing.T) {
	idp := useMockIDP(t, frozen)
	res := deviceResolution(t, liveSession(idp.Issuer()))

	if err := getToken(res, strings.ToUpper(testHost)); err != nil {
		t.Errorf("AccessToken = %v, want the same host in another case to match", err)
	}
}

func getToken(res Resolution, host string) error {
	_, err := res.AccessToken(context.Background(), host)
	return err
}

// errHint reads the actionable half of the family error contract, which is
// where the next command lives rather than in the message.
func errHint(t *testing.T, err error) string {
	t.Helper()
	var oerr *out.Error
	if !out.As(err, &oerr) {
		t.Fatalf("error = %v, want the family error contract", err)
	}
	return oerr.Hint
}

// The binding fails closed. A URI whose host cannot be read, or a session that
// was never tied to a deployment, must not be a way around the check.
func TestDeviceFlowHostBindingFailsClosed(t *testing.T) {
	idp := useMockIDP(t, frozen)

	tests := []struct {
		name        string
		sessionHost string
		target      string
	}{
		{name: "unreadable target host", sessionHost: testHost, target: ""},
		{name: "session bound to nothing", sessionHost: "", target: testHost},
		{name: "neither known", sessionHost: "", target: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := liveSession(idp.Issuer())
			session.Host = tt.sessionHost
			if err := getToken(deviceResolution(t, session), tt.target); !errors.Is(err, ErrSessionHostMismatch) {
				t.Errorf("error = %v, want ErrSessionHostMismatch", err)
			}
		})
	}
}

// A failed renewal must leave the stored session alone: overwriting it with an
// empty one would turn a transient provider outage into a forced re-login.
func TestFailedRefreshLeavesTheStoredSessionIntact(t *testing.T) {
	testutil.IsolateConfig(t)
	idp := useMockIDP(t, frozen)
	idp.TokenError = "temporarily_unavailable"

	session := liveSession(idp.Issuer())
	session.ExpiresAt = frozen.Add(-time.Minute)
	cred := deviceCredential(t, session)
	testutil.StageCredential(t, "corp", cred)

	res := Resolution{Alias: "corp", Kind: config.KindOIDC, Credential: cred}
	if _, err := res.AccessToken(context.Background(), testHost); err == nil {
		t.Fatal("a failing refresh reported success")
	}

	stored, err := Resolve("corp")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	saved, err := decodeSession("corp", stored.Credential.Session)
	if err != nil {
		t.Fatalf("decodeSession: %v", err)
	}
	if saved.RefreshToken != "refresh-0" {
		t.Errorf("stored refresh token = %q, want the original left untouched", saved.RefreshToken)
	}
}

// A provider that supplies no expiry leaves the server as the authority: the
// token is used, and a rejection comes back through here as a reauth.
func TestSessionWithNoExpiryIsUsed(t *testing.T) {
	idp := useMockIDP(t, frozen)
	session := liveSession(idp.Issuer())
	session.ExpiresAt = time.Time{}

	token, err := deviceResolution(t, session).AccessToken(context.Background(), testHost)
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token != "live-token" {
		t.Errorf("token = %q, want the stored one used", token)
	}
	if idp.Refreshes != 0 {
		t.Error("a session with no stated expiry was refreshed anyway")
	}
}

// The token is fetched inside the driver's callback, which only runs after a
// connection exists — so without this check a credential nobody has logged in
// with surfaces as a connection or DNS failure, hiding the one error that says
// what to do.
func TestRequireSession(t *testing.T) {
	fixedClock(t, frozen)

	t.Run("device flow with no session", func(t *testing.T) {
		res := Resolution{
			Alias:      "corp",
			Kind:       config.KindOIDC,
			Credential: config.Credential{Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice}},
		}
		if err := res.RequireSession(); !errors.Is(err, ErrNotLoggedIn) {
			t.Errorf("error = %v, want ErrNotLoggedIn", err)
		}
	})

	t.Run("device flow with a session", func(t *testing.T) {
		res := deviceResolution(t, liveSession("https://idp.example.com"))
		res.Credential.Kind = config.KindOIDC
		if err := res.RequireSession(); err != nil {
			t.Errorf("RequireSession = %v, want nil", err)
		}
	})

	// An expired session may still be renewable, and finding out is the
	// callback's job — this check must not pre-empt it.
	t.Run("an expired session is not refused here", func(t *testing.T) {
		session := liveSession("https://idp.example.com")
		session.ExpiresAt = frozen.Add(-time.Hour)
		res := deviceResolution(t, session)
		res.Credential.Kind = config.KindOIDC
		if err := res.RequireSession(); err != nil {
			t.Errorf("RequireSession = %v, want the refresh left to the callback", err)
		}
	})

	t.Run("flows that keep no session pass", func(t *testing.T) {
		for _, cred := range []config.Credential{
			{Username: "u", Password: "p"},
			testutil.OIDCCredential(config.EnvironmentK8s),
			{Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowFile, Path: "/t"}},
		} {
			res := Resolution{Alias: "c", Kind: cred.ResolvedKind(), Credential: cred}
			if err := res.RequireSession(); err != nil {
				t.Errorf("RequireSession(%v) = %v, want nil", cred.Flow, err)
			}
		}
	})
}

package credential

import (
	"errors"
	"strings"
	"testing"
	"time"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/oidc"
	"github.com/shhac/agent-mongo/internal/testutil"
)

// DescribeSession answers a listing rather than an authentication, so every
// failure reads as "not logged in" rather than propagating.
func TestDescribeSession(t *testing.T) {
	tests := []struct {
		name     string
		stage    func(t *testing.T)
		loggedIn bool
	}{
		{
			name: "device flow, never logged in",
			stage: func(t *testing.T) {
				testutil.StageCredential(t, "corp", config.Credential{
					Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
				})
			},
		},
		{
			name: "device flow with a session",
			stage: func(t *testing.T) {
				testutil.StageCredential(t, "corp", config.Credential{
					Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
				})
				if err := SaveSession("corp", liveSession("https://idp.example.com")); err != nil {
					t.Fatalf("SaveSession: %v", err)
				}
			},
			loggedIn: true,
		},
		{
			name: "a session that will not parse",
			stage: func(t *testing.T) {
				testutil.StageCredential(t, "corp", config.Credential{
					Kind:    config.KindOIDC,
					Flow:    &config.Flow{Type: config.FlowDevice},
					Session: "not json",
				})
			},
		},
		{
			name: "an environment flow keeps none",
			stage: func(t *testing.T) {
				testutil.StageCredential(t, "corp", testutil.OIDCCredential(config.EnvironmentK8s))
			},
		},
		{
			name: "scram keeps none",
			stage: func(t *testing.T) {
				testutil.StageCredential(t, "corp", config.Credential{Username: "u", Password: "p"})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.IsolateConfig(t)
			tt.stage(t)

			info := DescribeSession("corp", All()["corp"])
			if info.LoggedIn != tt.loggedIn {
				t.Errorf("LoggedIn = %v, want %v", info.LoggedIn, tt.loggedIn)
			}
			if !tt.loggedIn && !info.ExpiresAt.IsZero() {
				t.Errorf("ExpiresAt = %s, want the zero time", info.ExpiresAt)
			}
		})
	}
}

// The keychain path has to be followed, or a listing reports "not logged in"
// for a credential that authenticates perfectly well.
func TestDescribeSessionFollowsTheKeychain(t *testing.T) {
	isolateConfig(t)
	swapKeychain(t, newFakeKeychain())
	testutil.StageCredential(t, "corp", config.Credential{
		Kind: config.KindOIDC, Flow: &config.Flow{Type: config.FlowDevice},
	})
	if err := SaveSession("corp", liveSession("https://idp.example.com")); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	entry := All()["corp"]
	if entry.Session != Sentinel {
		t.Fatalf("precondition: session should be keychain-backed, got %q", entry.Session)
	}
	info := DescribeSession("corp", entry)
	if !info.LoggedIn {
		t.Error("a keychain-backed session was reported as not logged in")
	}
	if info.Host != testHost {
		t.Errorf("Host = %q, want %q", info.Host, testHost)
	}
}

func TestSessionExpired(t *testing.T) {
	fixedClock(t, frozen)
	tests := []struct {
		name string
		info SessionInfo
		want bool
	}{
		{"not logged in", SessionInfo{}, false},
		{"no stated expiry", SessionInfo{LoggedIn: true}, false},
		{"still valid", SessionInfo{LoggedIn: true, ExpiresAt: frozen.Add(time.Hour)}, false},
		{"expired", SessionInfo{LoggedIn: true, ExpiresAt: frozen.Add(-time.Hour)}, true},
		{"expiring exactly now", SessionInfo{LoggedIn: true, ExpiresAt: frozen}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.SessionExpired(); got != tt.want {
				t.Errorf("SessionExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveSessionRejectsAnUnknownCredential(t *testing.T) {
	testutil.IsolateConfig(t)
	if err := SaveSession("nope", liveSession("https://idp.example.com")); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// A session with no access token is not usable however fresh its expiry looks.
func TestSessionWithNoTokenIsNotUsable(t *testing.T) {
	fixedClock(t, frozen)
	session := Session{ExpiresAt: frozen.Add(time.Hour)}
	if session.usable() {
		t.Error("a session with no access token reported itself usable")
	}
}

// A deployment that authenticates without ever invoking the login callback is
// not configured for the flow, and says so rather than reporting success.
func TestLoginNotAttemptedError(t *testing.T) {
	err := LoginNotAttemptedError()
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("error = %v, want ErrNotLoggedIn", err)
	}
	if !strings.Contains(err.Error(), "did not ask") {
		t.Errorf("error = %q, want it to say the deployment never asked", err)
	}
}

// An expired code is worth retrying; a declined one is not. They are classified
// differently because that is what an agent branches on.
func TestLoginFailedErrorClassifiesByCause(t *testing.T) {
	expired := LoginFailedError(oidc.ErrCodeExpired)
	if fixableBy(t, expired) != "retry" {
		t.Errorf("expired code fixable_by = %q, want retry", fixableBy(t, expired))
	}
	denied := LoginFailedError(oidc.ErrDenied)
	if fixableBy(t, denied) != "human" {
		t.Errorf("denied fixable_by = %q, want human", fixableBy(t, denied))
	}
}

func fixableBy(t *testing.T, err error) string {
	t.Helper()
	var oerr *out.Error
	if !out.As(err, &oerr) {
		t.Fatalf("error = %v, want the family error contract", err)
	}
	return string(oerr.FixableBy)
}

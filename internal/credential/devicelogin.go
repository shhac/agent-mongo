package credential

import (
	"context"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
)

// DevicePrompt is what a person needs in order to finish a login: a short code
// and where to type it.
type DevicePrompt struct {
	UserCode string
	// VerificationURI is where the code is entered.
	// VerificationURIComplete, when the provider supplies one, already carries
	// the code, so the person only has to open a link.
	VerificationURI         string
	VerificationURIComplete string
	ExpiresAt               time.Time
}

// IDPInfo is what the MongoDB server says about its identity provider.
//
// It comes from the server rather than from config on purpose: the deployment
// is the authority on which provider guards it, so a hand-edited config cannot
// point a login somewhere else and collect the code.
type IDPInfo struct {
	Issuer   string
	ClientID string
	Scopes   []string
}

// RunDeviceLogin performs the interactive half of the device flow: start the
// grant, hand the code to the person through prompt, and wait.
//
// host is recorded on the session so the token is never presented to another
// deployment.
func RunDeviceLogin(
	ctx context.Context, idp IDPInfo, host string, prompt func(DevicePrompt),
) (Session, error) {
	provider, err := identityProvider.Discover(ctx, idp.Issuer)
	if err != nil {
		return Session{}, LoginFailedError(err)
	}

	auth, err := identityProvider.StartDeviceAuth(ctx, provider, idp.ClientID, idp.Scopes)
	if err != nil {
		return Session{}, LoginFailedError(err)
	}
	prompt(DevicePrompt{
		UserCode:                auth.UserCode,
		VerificationURI:         auth.VerificationURI,
		VerificationURIComplete: auth.VerificationURIComplete,
		ExpiresAt:               auth.ExpiresAt,
	})

	token, err := identityProvider.PollForToken(ctx, provider, idp.ClientID, auth)
	if err != nil {
		return Session{}, LoginFailedError(err)
	}

	return Session{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.ExpiresAt,
		Issuer:       idp.Issuer,
		ClientID:     idp.ClientID,
		Host:         host,
	}, nil
}

// SaveSession stores a completed login against a credential, replacing whatever
// session it had.
func SaveSession(alias string, session Session) error {
	entry, ok := config.Read().Credentials[alias]
	if !ok {
		return NotFoundError(alias)
	}
	return storeSession(alias, entry, session)
}

// ClearSession ends a session without removing the credential, which is the
// operation an administrator asks for: stop the access, keep the configuration.
func ClearSession(alias string) error {
	entry, ok := config.Read().Credentials[alias]
	if !ok {
		return NotFoundError(alias)
	}
	if !IsDeviceFlow(entry) {
		return NoSessionToClearError(alias)
	}

	_ = keychain.Delete(sessionAccount(alias))
	entry.Session = ""
	return config.Update(func(cfg *config.Config) error {
		cfg.SetCredential(alias, entry)
		return nil
	})
}

// IsDeviceFlow reports whether a credential keeps a session.
//
// Asked in three places — logging out, describing a session, and rendering a
// row — and previously written at three different strengths, so logging out an
// OIDC credential on a platform-identity flow reported success and told the
// person to log in again, for a credential that can never be logged in.
func IsDeviceFlow(entry config.Credential) bool {
	return entry.ResolvedKind() == config.KindOIDC &&
		entry.Flow != nil && entry.Flow.Type == config.FlowDevice
}

// SessionInfo is what may safely be shown about a stored session: whether there
// is one and when it runs out. Never the tokens.
type SessionInfo struct {
	LoggedIn  bool
	ExpiresAt time.Time
	Host      string
	Issuer    string
}

// DescribeSession reports a credential's session state for display.
//
// It resolves through the keychain but never fails: a credential with no
// session, an unreadable one, or a kind that keeps none all read as "not logged
// in", because this answers a listing rather than an authentication.
func DescribeSession(alias string, entry config.Credential) SessionInfo {
	if !IsDeviceFlow(entry) {
		return SessionInfo{}
	}

	raw := entry.Session
	if raw == Sentinel {
		stored, found := keychain.Get(sessionAccount(alias))
		if !found {
			return SessionInfo{}
		}
		raw = stored
	}
	session, err := decodeSession(alias, raw)
	if err != nil {
		return SessionInfo{}
	}
	return SessionInfo{
		LoggedIn:  session.AccessToken != "",
		ExpiresAt: session.ExpiresAt,
		Host:      session.Host,
		Issuer:    session.Issuer,
	}
}

// SessionExpired reports whether a session has run out, for a caller deciding
// what to tell a person rather than whether to refresh.
func (s SessionInfo) SessionExpired() bool {
	return s.LoggedIn && !s.ExpiresAt.IsZero() && !now().Before(s.ExpiresAt)
}

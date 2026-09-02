package credential

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/oidc"
)

// identityProvider is the seam over the OIDC client, so tests point the device
// flow at a mock provider without reaching the network.
var identityProvider = &oidc.Client{}

// refreshLeeway is how long before expiry a token is treated as spent.
//
// Without it a token that passes the check here can still expire between the
// callback returning and the server reading it, which surfaces as an
// authentication failure with nothing to act on.
const refreshLeeway = time.Minute

// sessionAccount is the keychain account holding a credential's session.
func sessionAccount(alias string) string { return "session:" + alias }

// oidcFields declares the one field an OIDC credential may keep in the
// keychain. Only the device flow fills it; for the others it stays empty and
// the generic storage skips it.
var oidcFields = []secretField{
	{
		account: sessionAccount,
		value:   func(c *config.Credential) *string { return &c.Session },
		missing: NotLoggedInError,
	},
}

// Session is what a completed device login leaves behind.
//
// Issuer and ClientID are recorded because a refresh needs them and the flow
// stores neither: they come from the server at login time, which is what keeps
// a hand-edited config from pointing the login at a different provider. Host is
// the deployment the session was obtained for, and the token is not presented
// anywhere else.
type Session struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Issuer       string    `json:"issuer"`
	ClientID     string    `json:"client_id"`
	Host         string    `json:"host"`
}

// usable reports whether the access token can still be presented, leaving room
// for the request it is about to be used on.
func (s Session) usable() bool {
	if s.AccessToken == "" {
		return false
	}
	if s.ExpiresAt.IsZero() {
		// No expiry from the provider: the server is the authority, and a
		// rejection triggers a reauth that comes back through here.
		return true
	}
	return now().Add(refreshLeeway).Before(s.ExpiresAt)
}

func encodeSession(session Session) (string, error) {
	encoded, err := json.Marshal(session)
	return string(encoded), err
}

func decodeSession(alias, raw string) (Session, error) {
	if raw == "" {
		return Session{}, NotLoggedInError(alias)
	}
	var session Session
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return Session{}, CorruptSessionError(alias)
	}
	return session, nil
}

// storeSession writes a session back, preserving the rest of the credential.
func storeSession(alias string, cred config.Credential, session Session) error {
	encoded, err := encodeSession(session)
	if err != nil {
		return err
	}
	cred.Session = encoded
	_, err = Store(alias, cred)
	return err
}

// deviceFlowToken is the query-path half of the device flow: read the session,
// refresh it if it has gone stale, and never prompt.
//
// The driver may call this twice in one authentication attempt, and an
// interactive prompt here would fire on both. Logging in is a separate command
// for that reason, and a session that cannot be renewed without a person is an
// ordinary self-correcting error.
func deviceFlowToken(
	ctx context.Context, alias string, cred config.Credential, host string,
) (string, error) {
	session, err := decodeSession(alias, cred.Session)
	if err != nil {
		return "", err
	}

	// The token was issued for one deployment and is not shown to another. The
	// driver binds nothing, and an agent can point a connection wherever it
	// likes, so this is the check that makes a stored session safe to keep.
	//
	// It fails closed. An empty target is a URI whose host could not be read,
	// and an empty binding is a session that was never tied to a deployment;
	// neither is a reason to hand over a token, and skipping the check for
	// either would make an unreadable connection string the way around it.
	if host == "" || session.Host == "" || !strings.EqualFold(session.Host, host) {
		return "", SessionHostMismatchError(alias, session.Host, host)
	}

	if session.usable() {
		return session.AccessToken, nil
	}
	if session.RefreshToken == "" {
		return "", SessionExpiredError(alias)
	}

	refreshed, err := refreshSession(ctx, alias, session)
	if err != nil {
		return "", err
	}
	if err := storeSession(alias, cred, refreshed); err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}

// refreshSession renews the access token against the provider that issued it.
//
// The driver never does this for agent-mongo: it hands a callback the refresh
// token only when an earlier callback in the same client returned one, and a
// CLI is a fresh process every time. Expiry is ours to track and refresh is
// ours to perform.
func refreshSession(ctx context.Context, alias string, session Session) (Session, error) {
	provider, err := identityProvider.Discover(ctx, session.Issuer)
	if err != nil {
		return Session{}, RefreshFailedError(alias, err)
	}
	token, err := identityProvider.Refresh(ctx, provider, session.ClientID, session.RefreshToken)
	if err != nil {
		if errors.Is(err, oidc.ErrRefreshRejected) {
			return Session{}, SessionExpiredError(alias)
		}
		return Session{}, RefreshFailedError(alias, err)
	}

	renewed := session
	renewed.AccessToken = token.AccessToken
	renewed.ExpiresAt = token.ExpiresAt
	// A provider that returns no new refresh token is saying the old one still
	// stands; dropping it here would end the session at the next refresh.
	if token.RefreshToken != "" {
		renewed.RefreshToken = token.RefreshToken
	}
	return renewed, nil
}

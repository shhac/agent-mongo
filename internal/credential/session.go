package credential

// What a session is, and how it is kept: the type, its storage in the OS
// keychain, and the safe-to-display view of it. What the device flow *does*
// with one — logging in, presenting a token, renewing it — lives in
// devicelogin.go, so a later kind that also keeps a session reuses this file
// untouched.

import (
	"encoding/json"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
)

// sessionAccount is the keychain account holding a credential's session.
func sessionAccount(alias string) string { return "session:" + alias }

// sessionField is the one field an OIDC credential may keep in the keychain.
// Only the device flow fills it; for the others it stays empty and the generic
// storage skips it.
//
// Named rather than declared inline because two paths read it: authentication,
// which fails when it is missing, and the listing, which reports "not logged
// in" instead. Both go through this declaration so they cannot drift.
var sessionField = secretField{
	account: sessionAccount,
	value:   func(c *config.Credential) *string { return &c.Session },
	missing: NotLoggedInError,
}

var oidcFields = []secretField{sessionField}

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
//
// It goes straight to storeCredential with the OIDC kind's own fields rather
// than through Store. Store re-dispatches on kind, so a refresh — reached from
// inside the OIDC kind's own flow — would re-enter the kinds table, and that
// loop is what made the two tables an initialization cycle. It also re-ran the
// flow validation, on a recipe that had just been driven successfully.
func storeSession(alias string, cred config.Credential, session Session) error {
	encoded, err := encodeSession(session)
	if err != nil {
		return err
	}
	cred.Session = encoded
	_, err = storeCredential(kindHandler{fields: oidcFields}, alias, cred)
	return err
}

// RequireSession reports whether a credential is in a state to authenticate at
// all, without touching the network.
//
// The token itself is fetched inside the driver's callback, which only runs
// once a connection has been established — so a credential nobody has logged in
// with would otherwise surface as a connection or DNS failure, hiding the one
// error that says what to do. This is checked before connecting so "run
// credential login" arrives first.
//
// It deliberately does not refresh: an expired session may still be renewable,
// and finding out is the callback's job.
func (r Resolution) RequireSession() error {
	if !IsDeviceFlow(r.Credential) {
		return nil
	}
	session, err := decodeSession(r.Alias, r.Credential.Session)
	if err != nil {
		return err
	}
	if session.AccessToken == "" {
		return NotLoggedInError(r.Alias)
	}
	return nil
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
	}
}

// SessionExpired reports whether a session has run out, for a caller deciding
// what to tell a person rather than whether to refresh.
func (s SessionInfo) SessionExpired() bool {
	return s.LoggedIn && !s.ExpiresAt.IsZero() && !now().Before(s.ExpiresAt)
}

package credential

// What the device flow does: the interactive login, and the query-path token
// that renews itself. The session it produces, and how that is stored, live in
// session.go.

import (
	"context"
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

// UseIdentityProvider replaces the OIDC client and the clock, returning a
// function that restores them.
//
// Exported because the driver-facing half of a login lives in internal/mongo
// and needs the same seam; the alternative is that package reaching into this
// one's state, or its conversion logic going untested. It takes no *testing.T
// so this file does not drag the testing package into the shipped binary.
func UseIdentityProvider(client *oidc.Client, clock func() time.Time) (restore func()) {
	prevClient, prevNow := identityProvider, now
	identityProvider = client
	if clock != nil {
		now = clock
	}
	return func() { identityProvider, now = prevClient, prevNow }
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

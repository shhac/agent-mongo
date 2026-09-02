package credential

import (
	"errors"
	"fmt"
	"time"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/oidc"
)

// The failure modes a caller may need to tell apart. They are discriminated
// with errors.Is rather than by a parallel status field, so there is one
// channel to keep honest instead of two.
var (
	// ErrNotFound: no credential is stored under the alias.
	ErrNotFound = errors.New("credential not found")
	// ErrUnresolvable: the entry exists but its secret could not be read back.
	ErrUnresolvable = errors.New("credential secret unreadable")
	// ErrUnsupportedKind: the entry names a kind this build does not implement.
	ErrUnsupportedKind = errors.New("unsupported credential kind")
	// ErrInvalidFlow: an OIDC credential's flow recipe is missing or unusable.
	ErrInvalidFlow = errors.New("invalid credential flow")
	// ErrInsecureConnection: the endpoint would carry a token in cleartext.
	ErrInsecureConnection = errors.New("insecure connection for token auth")
	// ErrHostNotAllowed: the endpoint is outside the credential's allowlist.
	ErrHostNotAllowed = errors.New("host not allowed for this credential")
	// ErrTokenUnreadable: a token could not be read, or is not a JWT.
	ErrTokenUnreadable = errors.New("oidc token unreadable")
	// ErrTokenExpired: a token was read but has already expired.
	ErrTokenExpired = errors.New("oidc token expired")
	// ErrNotLoggedIn: a device credential has no session yet.
	ErrNotLoggedIn = errors.New("no session for this credential")
	// ErrSessionExpired: the session cannot be renewed without a person.
	ErrSessionExpired = errors.New("session expired")
	// ErrSessionHostMismatch: the session belongs to a different deployment.
	ErrSessionHostMismatch = errors.New("session bound to a different host")
	// ErrRefreshFailed: renewal failed for a reason a retry might fix.
	ErrRefreshFailed = errors.New("session refresh failed")
)

// The family error contract puts the next command in `hint`, not in the
// message, and classifies who can act on it. Every fix here is one a person has
// to carry out at a terminal except NotFoundError, which an agent can resolve
// by picking one of the aliases the message already lists.
//
// The sentinel goes on via WithCause rather than %w: wrapping would append the
// sentinel's own text to the message an agent reads, and the message is the
// self-correcting half of the contract.

// NotFoundError is the shared self-correcting error for a missing credential
// reference (connection add/update validation, connect, and remove).
func NotFoundError(alias string) error {
	return notFoundError(alias, Aliases())
}

// notFoundError takes the alias list explicitly so a caller already holding the
// config lock can answer from its own snapshot rather than re-reading the file.
func notFoundError(alias string, available []string) error {
	return out.New(
		fmt.Sprintf("Credential %q not found. Available: %s",
			alias, config.JoinOrNone(available)),
		out.FixableByAgent,
	).WithCause(ErrNotFound).WithHint(
		"Pick one of the available aliases, or create it: agent-mongo credential add <alias> --form (or --username <user> --password <pass>)")
}

// UnresolvableError covers an entry whose keychain secret has gone missing —
// distinct from "not found", and fixed by re-adding rather than by picking a
// different alias. Human, because --form opens an OS dialog someone has to type
// into.
func UnresolvableError(alias string) error {
	return out.New(
		fmt.Sprintf("Credential %q is keychain-backed but its secret could not be read", alias),
		out.FixableByHuman,
	).WithCause(ErrUnresolvable).WithHint(
		fmt.Sprintf("Re-add it: agent-mongo credential add %s --form", alias))
}

// UnsupportedKindError names a kind config.json asks for that this build does
// not implement — most likely a config written by a newer agent-mongo. Human,
// because the fix is installing a different binary.
func UnsupportedKindError(alias string, kind config.Kind) error {
	return out.New(
		fmt.Sprintf("Credential %q has unsupported kind %q. Supported: %s",
			alias, kind, config.JoinOrNone(SupportedKinds())),
		out.FixableByHuman,
	).WithCause(ErrUnsupportedKind).WithHint(
		"Upgrade agent-mongo to a build that implements this kind, or re-add the credential with a supported one.")
}

// MissingFlowError covers an OIDC credential with no flow at all — the whole of
// what such a credential is.
func MissingFlowError(alias string) error {
	return out.New(
		fmt.Sprintf("Credential %q has no flow: an oidc credential needs to say how it obtains a session", alias),
		out.FixableByHuman,
	).WithCause(ErrInvalidFlow).WithHint(
		"Re-add it with a flow: agent-mongo credential add " + alias + " --oidc --environment k8s")
}

// UnsupportedFlowError names a flow this build does not implement — most likely
// a config written by a newer agent-mongo.
func UnsupportedFlowError(alias string, flowType config.FlowType) error {
	return out.New(
		fmt.Sprintf("Credential %q has unsupported flow type %q. Supported: %s",
			alias, flowType, config.JoinOrNone(SupportedFlowTypes())),
		out.FixableByHuman,
	).WithCause(ErrInvalidFlow).WithHint(
		"Upgrade agent-mongo to a build that implements this flow, or re-add the credential with a supported one.")
}

// UnknownEnvironmentError names a platform identity provider the driver has no
// implementation for.
func UnknownEnvironmentError(alias, environment string) error {
	return out.New(
		fmt.Sprintf("Credential %q names unknown environment %q. Valid: %s",
			alias, environment, config.JoinOrNone(environmentNames())),
		out.FixableByHuman,
	).WithCause(ErrInvalidFlow).WithHint(
		"Re-add it with a valid environment: agent-mongo credential add " + alias + " --oidc --environment k8s")
}

// MissingTokenResourceError covers an environment whose provider mints tokens
// for a named audience and was not given one.
func MissingTokenResourceError(alias, environment string) error {
	return out.New(
		fmt.Sprintf("Credential %q needs a token resource: the %s environment mints tokens for a named audience",
			alias, environment),
		out.FixableByHuman,
	).WithCause(ErrInvalidFlow).WithHint(fmt.Sprintf(
		"Re-add it with the audience: agent-mongo credential add %s --oidc --environment %s --token-resource <audience>",
		alias, environment))
}

// InsecureConnectionError covers an endpoint that would carry a bearer token in
// cleartext.
func InsecureConnectionError() error {
	return out.New(
		"OIDC authentication requires TLS: a bearer token on a plaintext connection is a credential on the wire",
		out.FixableByHuman,
	).WithCause(ErrInsecureConnection).WithHint(
		"Use a mongodb+srv:// URI, or add tls=true to the connection string.")
}

// HostNotAllowedError covers an endpoint outside the credential's allowlist.
func HostNotAllowedError(host string, allowed []string) error {
	return out.New(
		fmt.Sprintf("Host %q is not in this credential's allowed hosts: %s",
			host, config.JoinOrNone(allowed)),
		out.FixableByHuman,
	).WithCause(ErrHostNotAllowed).WithHint(
		"Point the connection at an allowed host, or widen the credential deliberately: agent-mongo credential add <alias> --oidc --allowed-hosts <pattern>,<pattern>")
}

// MissingTokenPathError covers a file flow with no path to read.
func MissingTokenPathError(alias string) error {
	return out.New(
		fmt.Sprintf("Credential %q has no token path: a file flow needs the file holding the token", alias),
		out.FixableByHuman,
	).WithCause(ErrInvalidFlow).WithHint(
		"Re-add it with the path: agent-mongo credential add " + alias + " --oidc --token-file /path/to/token")
}

// RelativeTokenPathError rejects a path that would resolve differently
// depending on where the command was run from.
func RelativeTokenPathError(alias, path string) error {
	return out.New(
		fmt.Sprintf("Credential %q has a relative token path %q: a saved credential is used from wherever the command runs", alias, path),
		out.FixableByHuman,
	).WithCause(ErrInvalidFlow).WithHint(
		"Re-add it with an absolute path: agent-mongo credential add " + alias + " --oidc --token-file /path/to/token")
}

// TokenFileError covers a token file that could not be read at authentication
// time — deleted, rotated away, or unreadable by this process.
func TokenFileError(path string, cause error) error {
	return out.New(
		fmt.Sprintf("Could not read the OIDC token file %q: %v", path, cause),
		out.FixableByHuman,
	).WithCause(ErrTokenUnreadable).WithHint(
		"Check the file exists and this process can read it; whatever issues the token may need to run again.")
}

// MalformedTokenError covers a token that is not a JWT, which is worth
// catching here because the server's rejection says only that authentication
// failed. source names where it came from, so the message reads correctly for a
// file and for a stored session alike.
func MalformedTokenError(source string) error {
	return out.New(
		fmt.Sprintf("The OIDC token in %s is not a JWT", source),
		out.FixableByHuman,
	).WithCause(ErrTokenUnreadable).WithHint(
		"It should be a single JSON Web Token: three dot-separated segments, nothing else.")
}

// ExpiredTokenError reports a token that has already expired, so the caller is
// told to refresh it rather than left to read a generic authentication failure.
func ExpiredTokenError(source string, expiry time.Time) error {
	return out.New(
		fmt.Sprintf("The OIDC token in %s expired at %s", source, expiry.UTC().Format(time.RFC3339)),
		out.FixableByHuman,
	).WithCause(ErrTokenExpired).WithHint(
		"Whatever issues this token needs to run again to refresh it.")
}

// FlowSuppliesNoTokenError covers asking for a token from a flow where the
// driver fetches it directly. It is a programming error rather than something a
// user can hit, and says so.
func FlowSuppliesNoTokenError(alias string, flowType config.FlowType) error {
	return out.New(
		fmt.Sprintf("Credential %q uses the %q flow, where the driver obtains the token itself", alias, flowType),
		out.FixableByAgent,
	).WithCause(ErrInvalidFlow).WithHint(
		"This flow needs no token from agent-mongo; nothing should be asking for one.")
}

// The device flow's failures. Each is fixable_by human because each ends at the
// same place: a person completing a login at a terminal. That classification is
// the signal an agent branches on — it can stop and say so rather than retrying
// something no retry will fix.

// NotLoggedInError covers a device credential that has never been logged in.
func NotLoggedInError(alias string) error {
	return out.New(
		fmt.Sprintf("Credential %q has no session: nobody has logged in with it yet", alias),
		out.FixableByHuman,
	).WithCause(ErrNotLoggedIn).WithHint(
		"Log in: agent-mongo credential login " + alias)
}

// SessionExpiredError covers a session whose refresh token is gone or refused.
func SessionExpiredError(alias string) error {
	return out.New(
		fmt.Sprintf("The session for credential %q has expired and cannot be renewed", alias),
		out.FixableByHuman,
	).WithCause(ErrSessionExpired).WithHint(
		"Log in again: agent-mongo credential login " + alias)
}

// CorruptSessionError covers a stored session that will not parse — a
// hand-edited config, or a keychain entry from a different tool.
func CorruptSessionError(alias string) error {
	return out.New(
		fmt.Sprintf("The stored session for credential %q could not be read", alias),
		out.FixableByHuman,
	).WithCause(ErrSessionExpired).WithHint(
		"Log in again to replace it: agent-mongo credential login " + alias)
}

// SessionHostMismatchError refuses to present a session to a deployment it was
// not obtained for.
//
// The driver binds a token to nothing, and an agent can point a connection
// wherever it likes, so this is what makes keeping a session safe.
func SessionHostMismatchError(alias, boundTo, target string) error {
	return out.New(
		fmt.Sprintf("The session for credential %q was obtained for %q and will not be sent to %q",
			alias, boundTo, target),
		out.FixableByHuman,
	).WithCause(ErrSessionHostMismatch).WithHint(fmt.Sprintf(
		"Point the connection at %s, or log in again for this host: agent-mongo credential login %s --connection <alias>",
		boundTo, alias))
}

// RefreshFailedError covers a renewal that failed for a reason other than the
// refresh token being refused — the provider was unreachable, or answered with
// something unexpected.
func RefreshFailedError(alias string, cause error) error {
	return out.New(
		fmt.Sprintf("Could not renew the session for credential %q: %v", alias, cause),
		out.FixableByRetry,
	).WithCause(ErrRefreshFailed).WithHint(
		"The identity provider could not be reached or refused the renewal. Retry; if it persists, log in again: agent-mongo credential login " + alias)
}

// LoginFailedError wraps whatever went wrong during an interactive login,
// classified by whether trying again could help.
func LoginFailedError(cause error) error {
	switch {
	case errors.Is(cause, oidc.ErrDenied):
		return out.New(
			"The login was declined at the identity provider",
			out.FixableByHuman,
		).WithCause(cause).WithHint("Run the login again and approve the request.")
	case errors.Is(cause, oidc.ErrCodeExpired):
		return out.New(
			"The login code expired before it was entered",
			out.FixableByRetry,
		).WithCause(cause).WithHint("Run the login again and enter the code before it expires.")
	default:
		return out.New(
			fmt.Sprintf("The login could not be completed: %v", cause),
			out.FixableByRetry,
		).WithCause(cause).WithHint(
			"The identity provider could not be reached or refused the request. Check network access to it and try again.")
	}
}

// NoSessionToClearError covers logging out a credential that keeps no session.
func NoSessionToClearError(alias string) error {
	return out.New(
		fmt.Sprintf("Credential %q keeps no session, so there is nothing to log out of", alias),
		out.FixableByAgent,
	).WithCause(ErrNotLoggedIn).WithHint(
		"Only an oidc credential on the device flow holds a session. See: agent-mongo credential list")
}

// LoginNotAttemptedError covers a login that connected and authenticated
// without the callback ever running — a deployment not configured for OIDC, or
// one that accepted the connection some other way.
func LoginNotAttemptedError() error {
	return out.New(
		"The deployment did not ask for an identity-provider login",
		out.FixableByHuman,
	).WithCause(ErrNotLoggedIn).WithHint(
		"Check the cluster has workforce identity federation configured, and that the connection points at it. OIDC needs MongoDB 7.0+ Enterprise or Atlas M10+.")
}

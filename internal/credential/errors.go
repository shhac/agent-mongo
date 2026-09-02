package credential

import (
	"fmt"
	"time"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
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

// MalformedTokenError covers a token file whose contents are not a JWT, which
// is worth catching here because the server's rejection says only that
// authentication failed.
func MalformedTokenError(path string) error {
	return out.New(
		fmt.Sprintf("The OIDC token file %q does not contain a JWT", path),
		out.FixableByHuman,
	).WithCause(ErrTokenUnreadable).WithHint(
		"The file should hold a single JSON Web Token: three dot-separated segments, nothing else.")
}

// ExpiredTokenError reports a token that has already expired, so the caller is
// told to refresh it rather than left to read a generic authentication failure.
func ExpiredTokenError(path string, expiry time.Time) error {
	return out.New(
		fmt.Sprintf("The OIDC token in %q expired at %s", path, expiry.UTC().Format(time.RFC3339)),
		out.FixableByHuman,
	).WithCause(ErrTokenExpired).WithHint(
		"Whatever issues this token needs to run again to refresh the file.")
}

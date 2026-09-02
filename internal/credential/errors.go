package credential

import (
	"fmt"

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

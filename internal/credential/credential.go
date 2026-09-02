// Package credential resolves credential aliases to the auth material the
// MongoDB driver needs, and persists it.
//
// SCRAM credentials keep their username and password in the OS keychain when
// one is available, falling back to plaintext in config.json. A keychain-backed
// field holds the Sentinel in config.json; the real value lives in the keychain
// service "app.paulie.agent-mongo" under the accounts "username:<alias>" and
// "password:<alias>".
//
// Every path here dispatches on config.Kind. A kind this build does not
// understand resolves to a named error rather than being guessed at as SCRAM,
// because guessing would run the plaintext-upgrade path over a credential whose
// empty username and password are correct rather than missing.
package credential

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/shhac/lib-agent-cli/creds"
	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
)

const (
	// Sentinel marks a config.json field whose real value is in the keychain.
	Sentinel = "__KEYCHAIN__"
	// Service is the reverse-DNS keychain service id.
	Service = "app.paulie.agent-mongo"

	StorageKeychain = "keychain"
	StorageConfig   = "config"
)

// State reports whether a stored credential can authenticate right now.
type State string

const (
	// StateReady: the credential resolved to usable auth material.
	StateReady State = "ready"
	// StateMissing: no credential is stored under the alias.
	StateMissing State = "missing"
	// StateUnresolvable: the entry exists but its secret could not be read
	// back out of the keychain.
	StateUnresolvable State = "unresolvable"
	// StateUnsupported: the entry names a kind this build does not implement.
	StateUnsupported State = "unsupported"
)

// Resolution is the outcome of resolving an alias to auth material. Alias,
// Kind and State are always set; Credential carries resolved secrets only when
// the State is StateReady.
type Resolution struct {
	Alias      string
	Kind       config.Kind
	State      State
	Credential config.Credential
}

func (r Resolution) Ready() bool { return r.State == StateReady }

// keychainStore is the seam over the OS keychain — satisfied by
// *keyring.Keyring in production and by a fake in tests.
type keychainStore interface {
	Available() bool
	Get(account string) (string, bool)
	Set(account, secret string) error
	Delete(account string) error
}

var keychain keychainStore = creds.NewKeychain(Service)

func usernameAccount(alias string) string { return "username:" + alias }
func passwordAccount(alias string) string { return "password:" + alias }

// Resolve turns an alias into usable auth material, or returns a
// self-correcting error naming the fix.
func Resolve(alias string) (Resolution, error) {
	entry, ok := config.Read().Credentials[alias]
	if !ok {
		return Resolution{Alias: alias, State: StateMissing}, NotFoundError(alias)
	}

	res := Resolution{Alias: alias, Kind: entry.ResolvedKind(), Credential: entry}
	switch res.Kind {
	case config.KindSCRAM:
		return resolveSCRAM(res)
	default:
		res.State = StateUnsupported
		return res, UnsupportedKindError(alias, res.Kind)
	}
}

// resolveSCRAM fills in whichever of username/password is keychain-backed.
func resolveSCRAM(res Resolution) (Resolution, error) {
	cred := res.Credential
	if cred.Username != Sentinel && cred.Password != Sentinel {
		maybeUpgrade(res.Alias, cred)
		res.State = StateReady
		return res, nil
	}
	if cred.Username == Sentinel {
		v, found := keychain.Get(usernameAccount(res.Alias))
		if !found || v == "" {
			res.State = StateUnresolvable
			return res, UnresolvableError(res.Alias)
		}
		cred.Username = v
	}
	if cred.Password == Sentinel {
		v, found := keychain.Get(passwordAccount(res.Alias))
		if !found || v == "" {
			res.State = StateUnresolvable
			return res, UnresolvableError(res.Alias)
		}
		cred.Password = v
	}
	res.Credential = cred
	res.State = StateReady
	return res, nil
}

// maybeUpgrade migrates a plaintext SCRAM credential (created before keychain
// support, or stored on a host without one) into the OS keychain the first
// time it is read on a host with a usable keychain. Any failure leaves the
// plaintext entry untouched.
//
// It is reachable only from resolveSCRAM: running it over another kind would
// write empty-string secrets into the keychain and stamp sentinels over an
// entry whose real material is not a username and password.
func maybeUpgrade(alias string, cred config.Credential) {
	if !keychain.Available() {
		return
	}
	if storage, err := Store(alias, cred); err == nil && storage == StorageKeychain {
		out.WriteNotice(os.Stderr,
			fmt.Sprintf("credential %q upgraded from plaintext config to keychain storage", alias), "")
	}
}

// All returns the raw stored entries (sentinels intact, secrets unresolved).
func All() map[string]config.Credential {
	entries := config.Read().Credentials
	if entries == nil {
		return map[string]config.Credential{}
	}
	return entries
}

func Aliases() []string { return aliasesOf(All()) }

// aliasesOf is the sorted-alias logic split out from Aliases so a caller
// already holding the config lock can answer from its own snapshot instead of
// re-reading the file.
func aliasesOf(entries map[string]config.Credential) []string {
	aliases := make([]string, 0, len(entries))
	for alias := range entries {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

// Store persists a credential, dispatching on its kind. Returns which storage
// held the secret ("keychain" or "config").
func Store(alias string, cred config.Credential) (string, error) {
	switch cred.ResolvedKind() {
	case config.KindSCRAM:
		return storeSCRAM(alias, cred)
	default:
		return "", UnsupportedKindError(alias, cred.ResolvedKind())
	}
}

// storeSCRAM writes the username/password pair, preferring the OS keychain.
//
// The keychain writes happen inside config.Update's critical section rather
// than before it, because the keychain and config.json have to agree: the
// sentinel is only written when the secret really landed in the keychain, and
// the partial-write cleanup only runs when the config entry is about to say
// "plaintext". Splitting them would let a concurrent writer interleave between
// the two halves.
func storeSCRAM(alias string, cred config.Credential) (string, error) {
	storage := StorageConfig
	err := config.Update(func(cfg *config.Config) error {
		if cfg.Credentials == nil {
			cfg.Credentials = map[string]config.Credential{}
		}

		if keychain.Available() {
			userErr := keychain.Set(usernameAccount(alias), cred.Username)
			passErr := keychain.Set(passwordAccount(alias), cred.Password)
			if userErr == nil && passErr == nil {
				// Copy and overwrite rather than construct: every field this
				// kind does not own has to survive the round trip.
				stored := cred
				stored.Username = Sentinel
				stored.Password = Sentinel
				cfg.Credentials[alias] = stored
				storage = StorageKeychain
				return nil
			}
		}

		// Fallback: plaintext in config.json; clean up any partial keychain entries.
		_ = keychain.Delete(usernameAccount(alias))
		_ = keychain.Delete(passwordAccount(alias))
		cfg.Credentials[alias] = cred
		storage = StorageConfig
		return nil
	})
	return storage, err
}

// ConnectionsUsing lists connection aliases that reference this credential.
func ConnectionsUsing(credentialAlias string) []string {
	return connectionsUsing(config.Connections(), credentialAlias)
}

// connectionsUsing is ConnectionsUsing over a caller-supplied snapshot, so the
// check can run against the config the lock just loaded.
func connectionsUsing(conns map[string]config.Connection, credentialAlias string) []string {
	// Stays nil when nothing matches: `credential list` marshals this straight
	// to JSON, where nil is null and an empty slice would be [].
	var used []string
	for alias, conn := range conns {
		if conn.Credential == credentialAlias {
			used = append(used, alias)
		}
	}
	sort.Strings(used)
	return used
}

// RequireExists checks only that the alias names a stored credential.
//
// This is the check `connection add` and `connection update` need. A
// connection may legitimately reference a credential that cannot authenticate
// at the moment it is wired up, so requiring a full Resolve here would reject
// valid configuration.
func RequireExists(alias string) error {
	if _, ok := config.Read().Credentials[alias]; !ok {
		return NotFoundError(alias)
	}
	return nil
}

// NotFoundError is the shared self-correcting error for a missing credential
// reference (used by connection add/update validation and connect).
func NotFoundError(alias string) error {
	return fmt.Errorf(
		"Credential %q not found. Available: %s. Run: agent-mongo credential add <alias> --form (or --username <user> --password <pass>)",
		alias, config.JoinOrNone(Aliases()))
}

// UnresolvableError covers an entry whose keychain secret has gone missing —
// distinct from "not found", and fixed by re-adding rather than by picking a
// different alias.
func UnresolvableError(alias string) error {
	return fmt.Errorf(
		"Credential %q is keychain-backed but its secret could not be read. Re-add it: agent-mongo credential add %s --form",
		alias, alias)
}

// UnsupportedKindError names a kind config.json asks for that this build does
// not implement — most likely a config written by a newer agent-mongo.
func UnsupportedKindError(alias string, kind config.Kind) error {
	return fmt.Errorf(
		"Credential %q has unsupported kind %q. Supported: %s. Upgrade agent-mongo, or re-add the credential.",
		alias, kind, config.JoinOrNone(config.SupportedKinds()))
}

// Remove deletes the keychain secrets inside the same critical section that
// drops the config entry, so the two cannot be separated by a concurrent
// writer re-adding the alias between them.
func Remove(alias string) error {
	return config.Update(func(cfg *config.Config) error {
		if _, ok := cfg.Credentials[alias]; !ok {
			return fmt.Errorf("Unknown credential: %q. Valid: %s", alias, config.JoinOrNone(aliasesOf(cfg.Credentials)))
		}
		if used := connectionsUsing(cfg.Connections, alias); len(used) > 0 {
			return fmt.Errorf(
				"Credential %q is used by connections: %s. Remove or update those connections first.",
				alias, strings.Join(used, ", "))
		}
		_ = keychain.Delete(usernameAccount(alias))
		_ = keychain.Delete(passwordAccount(alias))
		delete(cfg.Credentials, alias)
		return nil
	})
}

// StorageType reports where a credential's secret lives.
func StorageType(alias string) string {
	entry, ok := config.Read().Credentials[alias]
	if !ok {
		return StorageConfig
	}
	if entry.ResolvedKind() == config.KindSCRAM &&
		entry.Username == Sentinel && entry.Password == Sentinel {
		return StorageKeychain
	}
	return StorageConfig
}

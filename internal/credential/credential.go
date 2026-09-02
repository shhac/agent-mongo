// Package credential resolves credential aliases to the auth material the
// MongoDB driver needs, and persists it.
//
// SCRAM credentials keep their username and password in the OS keychain when
// one is available, falling back to plaintext in config.json. A keychain-backed
// field holds the Sentinel in config.json; the real value lives in the keychain
// service "app.paulie.agent-mongo" under the accounts "username:<alias>" and
// "password:<alias>".
//
// Every kind-sensitive operation dispatches through the kinds table rather than
// testing for a kind inline, so registering a kind is one table entry instead of
// a hunt for the places that branch. A kind this build does not implement is a
// named error rather than a guess at SCRAM: guessing would run the
// plaintext-upgrade path over a credential whose empty username and password
// are correct rather than missing.
package credential

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

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
)

// Resolution is a credential resolved to usable auth material. It is only
// meaningful alongside a nil error; Resolve returns the zero value on failure
// so a caller cannot mistake an unresolved sentinel for a password.
type Resolution struct {
	Alias      string
	Kind       config.Kind
	Credential config.Credential
}

// kindHandler is everything a credential kind has to supply. One entry in the
// kinds table is the single place a kind is registered: an incomplete entry
// fails to compile rather than falling through a switch nobody updated.
type kindHandler struct {
	// read fills in the entry's secrets from wherever this kind keeps them.
	// It is pure: no writes, so an inspection cannot mutate what it inspects.
	read func(alias string, entry config.Credential) (config.Credential, error)
	// migrate opportunistically moves an entry to better storage after a
	// successful read, and is called only on the authenticate path. Nil for a
	// kind with nothing to migrate.
	migrate func(alias string, entry config.Credential)
	// store persists the credential, reporting which storage took the secret.
	store func(alias string, cred config.Credential) (string, error)
	// keychainAccounts names every keychain account the kind owns for an alias,
	// so Remove can erase them without knowing what they hold.
	keychainAccounts func(alias string) []string
	// storageType reports where this entry's secret actually lives.
	storageType func(entry config.Credential) string
}

var kinds = map[config.Kind]kindHandler{
	config.KindSCRAM: {
		read:             readSCRAM,
		migrate:          maybeUpgradeSCRAM,
		store:            storeSCRAM,
		keychainAccounts: scramAccounts,
		storageType:      scramStorageType,
	},
}

func handlerFor(kind config.Kind) (kindHandler, bool) {
	h, ok := kinds[kind]
	return h, ok
}

// SupportedKinds lists the kinds this build implements, derived from the
// dispatch table so it cannot advertise a kind nothing drives. A hand-written
// list fails by telling the reader to choose from a set that is not real.
func SupportedKinds() []string {
	names := make([]string, 0, len(kinds))
	for kind := range kinds {
		names = append(names, string(kind))
	}
	sort.Strings(names)
	return names
}

// Resolve turns an alias into usable auth material, or returns a
// self-correcting error naming the fix. This is the authenticate path, and the
// only one that may migrate an entry to better storage as a side effect.
func Resolve(alias string) (Resolution, error) {
	r, err := read(alias)
	if err != nil {
		return Resolution{}, err
	}
	if r.handler.migrate != nil {
		r.handler.migrate(alias, r.entry)
	}
	return r.Resolution, nil
}

// Inspect resolves an alias exactly as Resolve does but writes nothing.
//
// Callers that want to know what is stored rather than to authenticate with it
// use this. Going through Resolve would migrate a plaintext credential into the
// keychain as a side effect of reading it, which is wrong on a path that may be
// about to refuse the operation it was checking for.
func Inspect(alias string) (Resolution, error) {
	r, err := read(alias)
	return r.Resolution, err
}

// reading is what the shared read produces: the resolution a caller wants, plus
// the two things only the migration step needs — the entry as it is stored
// (migration decides from that, not from what was resolved) and the handler
// already looked up, so Resolve does not repeat the lookup.
type reading struct {
	Resolution
	entry   config.Credential
	handler kindHandler
}

// read is the pure half both entry points share: look the entry up, dispatch to
// its kind, resolve its secrets. It writes nothing.
func read(alias string) (reading, error) {
	entry, ok := config.Read().Credentials[alias]
	if !ok {
		return reading{}, NotFoundError(alias)
	}

	kind := entry.ResolvedKind()
	h, ok := handlerFor(kind)
	if !ok {
		return reading{}, UnsupportedKindError(alias, kind)
	}
	cred, err := h.read(alias, entry)
	if err != nil {
		return reading{}, err
	}
	return reading{
		Resolution: Resolution{Alias: alias, Kind: kind, Credential: cred},
		entry:      entry,
		handler:    h,
	}, nil
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
	kind := cred.ResolvedKind()
	h, ok := handlerFor(kind)
	if !ok {
		return "", UnsupportedKindError(alias, kind)
	}
	return h.store(alias, cred)
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

// Remove deletes the kind's keychain secrets inside the same critical section
// that drops the config entry, so the two cannot be separated by a concurrent
// writer re-adding the alias between them.
func Remove(alias string) error {
	return config.Update(func(cfg *config.Config) error {
		entry, ok := cfg.Credentials[alias]
		if !ok {
			return notFoundError(alias, aliasesOf(cfg.Credentials))
		}
		if used := connectionsUsing(cfg.Connections, alias); len(used) > 0 {
			return fmt.Errorf(
				"Credential %q is used by connections: %s. Remove or update those connections first.",
				alias, strings.Join(used, ", "))
		}

		// Ask the kind which accounts it owns rather than naming SCRAM's pair:
		// a kind whose secret is not a username and password would otherwise
		// keep live material in the keychain with nothing left to reference it.
		if h, ok := handlerFor(entry.ResolvedKind()); ok {
			for _, account := range h.keychainAccounts(alias) {
				_ = keychain.Delete(account)
			}
		} else {
			// An entry written by a newer build. Removing it is still the
			// user's call — refusing would make the entry unremovable — but say
			// plainly that its secret may outlive it.
			out.WriteNotice(os.Stderr, fmt.Sprintf(
				"credential %q has unsupported kind %q; any keychain secret it owns was left in place",
				alias, entry.ResolvedKind()), "")
		}

		delete(cfg.Credentials, alias)
		return nil
	})
}

// StorageType reports where an entry's secret lives. It takes the entry rather
// than an alias so a caller rendering a list computes every column from one
// read of config.json instead of one read per row.
func StorageType(entry config.Credential) string {
	h, ok := handlerFor(entry.ResolvedKind())
	if !ok {
		return StorageConfig
	}
	return h.storageType(entry)
}

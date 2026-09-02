package credential

import (
	"fmt"
	"os"
	"sort"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
)

// secretField is one field of a credential whose value may live in the OS
// keychain, with the Sentinel standing in for it in config.json.
//
// Declaring the fields, rather than writing a reader and a writer per kind, is
// what makes storage generic: resolution, storage, migration, cleanup on
// removal and the storage-type report are all one implementation over this
// list. A kind with no secret of its own declares none and gets all of that
// behaviour for free.
type secretField struct {
	// account names the keychain account holding this field for an alias.
	account func(alias string) string
	// value points at the field inside a credential, for read and write.
	value func(*config.Credential) *string
	// missing is the error when the sentinel has no keychain value behind it.
	// It is per field because the fix differs: a vanished password means
	// re-add the credential, where a vanished session means log in again.
	missing func(alias string) error
}

// kindHandler is what a credential kind supplies. One entry in the kinds table
// is the single place a kind is registered.
type kindHandler struct {
	// fields are the kind's keychain-backed fields, in resolution order.
	fields []secretField
	// validate rejects a stored entry this kind cannot drive. Nil when any
	// entry of this kind is usable.
	validate func(alias string, entry config.Credential) error
	// checkConnection rejects an endpoint this kind must not be used with.
	// Nil when the kind places no constraints on where it authenticates.
	checkConnection func(entry config.Credential, uri string) error
}

var kinds = map[config.Kind]kindHandler{
	config.KindSCRAM: {fields: scramFields},
	config.KindOIDC: {
		fields:          oidcFields,
		validate:        validateOIDC,
		checkConnection: checkOIDCConnection,
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

// lookupField reads one field's value, following the sentinel into the keychain.
// It reports whether a value was found rather than producing an error, so a
// caller that is describing rather than authenticating can use the same
// declaration of where the value lives.
func lookupField(field secretField, alias string, entry config.Credential) (string, bool) {
	value := *field.value(&entry)
	if value != Sentinel {
		return value, value != ""
	}
	stored, found := keychain.Get(field.account(alias))
	return stored, found && stored != ""
}

// resolveFields fills in every keychain-backed field that holds the sentinel.
// It only reads: the plaintext-to-keychain migration is a separate step so that
// inspecting a credential cannot rewrite it.
func resolveFields(fields []secretField, alias string, entry config.Credential) (config.Credential, error) {
	resolved := entry
	for _, field := range fields {
		if *field.value(&resolved) != Sentinel {
			continue
		}
		stored, found := lookupField(field, alias, entry)
		if !found {
			return config.Credential{}, field.missing(alias)
		}
		*field.value(&resolved) = stored
	}
	return resolved, nil
}

// storeCredential writes an entry, preferring the OS keychain for whichever
// fields the kind declares.
//
// The keychain writes happen inside config.Update's critical section rather
// than before it, because the keychain and config.json have to agree: a
// sentinel is only written when the secret really landed in the keychain, and
// the partial-write cleanup only runs when the config entry is about to say
// "plaintext". Splitting them would let a concurrent writer interleave between
// the two halves.
func storeCredential(h kindHandler, alias string, cred config.Credential) (string, error) {
	if h.validate != nil {
		if err := h.validate(alias, cred); err != nil {
			return "", err
		}
	}

	storage := StorageConfig
	err := config.Update(func(cfg *config.Config) error {
		if len(h.fields) > 0 && keychain.Available() {
			if stored, ok := writeFieldsToKeychain(h.fields, alias, cred); ok {
				cfg.SetCredential(alias, stored)
				storage = StorageKeychain
				return nil
			}
		}

		// Fallback: plaintext in config.json; clean up any partial keychain
		// entries, since a half-written set resolves to nothing usable.
		for _, field := range h.fields {
			_ = keychain.Delete(field.account(alias))
		}
		cfg.SetCredential(alias, cred)
		storage = StorageConfig
		return nil
	})
	return storage, err
}

// writeFieldsToKeychain writes every declared field and returns the entry with
// sentinels in their place. Reports false if any write failed: a partially
// written set is not usable, so the caller falls back to plaintext.
func writeFieldsToKeychain(
	fields []secretField, alias string, cred config.Credential,
) (config.Credential, bool) {
	stored := cred
	for _, field := range fields {
		value := *field.value(&cred)
		if value == "" {
			// A field this credential does not use: an OIDC credential on a
			// platform-identity flow holds no session. Nothing to store, and
			// stamping a sentinel over it would invent one.
			continue
		}
		if err := keychain.Set(field.account(alias), value); err != nil {
			return config.Credential{}, false
		}
		*field.value(&stored) = Sentinel
	}
	return stored, true
}

// migrateToKeychain moves a plaintext entry (created before keychain support,
// or stored on a host without one) into the OS keychain the first time it is
// authenticated with on a host that has one. Any failure leaves the plaintext
// entry untouched.
//
// It runs only on the authenticate path, and decides for itself whether there
// is anything to do, so that path can call it unconditionally.
func migrateToKeychain(h kindHandler, alias string, entry config.Credential) {
	if !keychain.Available() {
		return
	}
	migratable := false
	for _, field := range h.fields {
		switch *field.value(&entry) {
		case Sentinel:
			return // already keychain-backed
		case "":
			continue // nothing stored in this field
		default:
			migratable = true
		}
	}
	if !migratable {
		return
	}
	if storage, err := storeCredential(h, alias, entry); err == nil && storage == StorageKeychain {
		out.WriteNotice(os.Stderr,
			fmt.Sprintf("credential %q upgraded from plaintext config to keychain storage", alias), "")
	}
}

// keychainAccountsFor names every keychain account a kind owns for an alias, so
// Remove can erase them without knowing what they hold.
func keychainAccountsFor(fields []secretField, alias string) []string {
	accounts := make([]string, 0, len(fields))
	for _, field := range fields {
		accounts = append(accounts, field.account(alias))
	}
	return accounts
}

// storageTypeFor reports keychain only when every declared field is actually
// backed by it. A kind that declares none has nothing in the keychain to
// report, however the entry is otherwise shaped.
func storageTypeFor(fields []secretField, entry config.Credential) string {
	keychainBacked := false
	for _, field := range fields {
		switch *field.value(&entry) {
		case "":
			continue // a field this credential does not use
		case Sentinel:
			keychainBacked = true
		default:
			return StorageConfig
		}
	}
	if !keychainBacked {
		return StorageConfig
	}
	return StorageKeychain
}

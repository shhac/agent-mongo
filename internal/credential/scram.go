package credential

import (
	"fmt"
	"os"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/config"
)

func usernameAccount(alias string) string { return "username:" + alias }
func passwordAccount(alias string) string { return "password:" + alias }

// scramAccounts names every keychain account a SCRAM credential owns, so the
// pair is spelled once rather than at each site that has to erase it.
func scramAccounts(alias string) []string {
	return []string{usernameAccount(alias), passwordAccount(alias)}
}

// readSCRAM fills in whichever of username/password is keychain-backed. It only
// reads: the plaintext-to-keychain migration is a separate step so that
// inspecting a credential cannot rewrite it.
func readSCRAM(alias string, entry config.Credential) (config.Credential, error) {
	username, err := resolveSentinel(alias, entry.Username, usernameAccount)
	if err != nil {
		return config.Credential{}, err
	}
	password, err := resolveSentinel(alias, entry.Password, passwordAccount)
	if err != nil {
		return config.Credential{}, err
	}
	entry.Username, entry.Password = username, password
	return entry, nil
}

// resolveSentinel returns the stored value unchanged unless it is the sentinel,
// in which case the real value comes from the keychain.
func resolveSentinel(alias, stored string, account func(string) string) (string, error) {
	if stored != Sentinel {
		return stored, nil
	}
	v, found := keychain.Get(account(alias))
	if !found || v == "" {
		return "", UnresolvableError(alias)
	}
	return v, nil
}

// maybeUpgradeSCRAM migrates a plaintext credential (created before keychain
// support, or stored on a host without one) into the OS keychain the first time
// it is authenticated with on a host that has one. Any failure leaves the
// plaintext entry untouched.
//
// It takes the stored entry and decides for itself whether there is anything to
// do, so the authenticate path can call it unconditionally. It calls storeSCRAM
// rather than the public Store so the kind cannot be re-dispatched: routing
// back through the table would make another kind's store reachable from the
// SCRAM read path.
func maybeUpgradeSCRAM(alias string, entry config.Credential) {
	if entry.Username == Sentinel || entry.Password == Sentinel {
		return // already keychain-backed
	}
	if !keychain.Available() {
		return
	}
	if storage, err := storeSCRAM(alias, entry); err == nil && storage == StorageKeychain {
		out.WriteNotice(os.Stderr,
			fmt.Sprintf("credential %q upgraded from plaintext config to keychain storage", alias), "")
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
		for _, account := range scramAccounts(alias) {
			_ = keychain.Delete(account)
		}
		cfg.Credentials[alias] = cred
		storage = StorageConfig
		return nil
	})
	return storage, err
}

func scramStorageType(entry config.Credential) string {
	if entry.Username == Sentinel && entry.Password == Sentinel {
		return StorageKeychain
	}
	return StorageConfig
}

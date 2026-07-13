// Package credential stores username/password pairs in the OS keychain when
// available, falling back to plaintext in config.json. Keychain-backed entries
// hold the "__KEYCHAIN__" sentinel in config.json; the real values live in the
// keychain service "app.paulie.agent-mongo" under the accounts
// "username:<alias>" and "password:<alias>".
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
	sentinel = "__KEYCHAIN__"
	// Service is the reverse-DNS keychain service id.
	Service = "app.paulie.agent-mongo"

	StorageKeychain = "keychain"
	StorageConfig   = "config"
)

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

// Get resolves a credential, reading keychain-backed fields through the OS
// keychain. Returns false when the credential is unknown or unresolvable.
func Get(alias string) (config.Credential, bool) {
	cred, ok := config.Read().Credentials[alias]
	if !ok {
		return config.Credential{}, false
	}
	if cred.Username != sentinel && cred.Password != sentinel {
		maybeUpgrade(alias, cred)
		return cred, true
	}
	if cred.Username == sentinel {
		v, found := keychain.Get(usernameAccount(alias))
		if !found || v == "" {
			return config.Credential{}, false
		}
		cred.Username = v
	}
	if cred.Password == sentinel {
		v, found := keychain.Get(passwordAccount(alias))
		if !found || v == "" {
			return config.Credential{}, false
		}
		cred.Password = v
	}
	return cred, true
}

// maybeUpgrade migrates a plaintext credential (created before keychain
// support, or stored on a host without one) into the OS keychain the first
// time it is read on a host with a usable keychain. Any failure leaves the
// plaintext entry untouched.
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

func Aliases() []string {
	aliases := make([]string, 0, len(All()))
	for alias := range All() {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

// Store persists a credential, preferring the OS keychain. Returns which
// storage held the secret ("keychain" or "config").
func Store(alias string, cred config.Credential) (string, error) {
	cfg := config.Read()
	if cfg.Credentials == nil {
		cfg.Credentials = map[string]config.Credential{}
	}

	if keychain.Available() {
		userErr := keychain.Set(usernameAccount(alias), cred.Username)
		passErr := keychain.Set(passwordAccount(alias), cred.Password)
		if userErr == nil && passErr == nil {
			cfg.Credentials[alias] = config.Credential{Username: sentinel, Password: sentinel}
			return StorageKeychain, config.Write(cfg)
		}
	}

	// Fallback: plaintext in config.json; clean up any partial keychain entries.
	_ = keychain.Delete(usernameAccount(alias))
	_ = keychain.Delete(passwordAccount(alias))
	cfg.Credentials[alias] = cred
	return StorageConfig, config.Write(cfg)
}

// ConnectionsUsing lists connection aliases that reference this credential.
func ConnectionsUsing(credentialAlias string) []string {
	var used []string
	for alias, conn := range config.Connections() {
		if conn.Credential == credentialAlias {
			used = append(used, alias)
		}
	}
	sort.Strings(used)
	return used
}

// Require verifies that a credential alias resolves, returning the shared
// self-correcting NotFoundError when it doesn't.
func Require(alias string) error {
	if _, ok := Get(alias); !ok {
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

func Remove(alias string) error {
	cfg := config.Read()
	if _, ok := cfg.Credentials[alias]; !ok {
		return fmt.Errorf("Unknown credential: %q. Valid: %s", alias, config.JoinOrNone(Aliases()))
	}
	if used := ConnectionsUsing(alias); len(used) > 0 {
		return fmt.Errorf(
			"Credential %q is used by connections: %s. Remove or update those connections first.",
			alias, strings.Join(used, ", "))
	}
	_ = keychain.Delete(usernameAccount(alias))
	_ = keychain.Delete(passwordAccount(alias))
	delete(cfg.Credentials, alias)
	return config.Write(cfg)
}

// StorageType reports where a credential's secret lives.
func StorageType(alias string) string {
	cred, ok := config.Read().Credentials[alias]
	if ok && cred.Username == sentinel && cred.Password == sentinel {
		return StorageKeychain
	}
	return StorageConfig
}

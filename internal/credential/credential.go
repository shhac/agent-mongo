// Package credential stores username/password pairs in the OS keychain when
// available, falling back to plaintext in config.json. The on-disk layout is
// byte-compatible with the TypeScript implementation: keychain-backed entries
// hold the "__KEYCHAIN__" sentinel in config.json, and the real values live in
// the keychain service "app.paulie.agent-mongo" under the accounts
// "username:<alias>" and "password:<alias>".
package credential

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shhac/lib-agent-cli/creds"

	"github.com/shhac/agent-mongo/internal/config"
)

const (
	sentinel = "__KEYCHAIN__"
	// Service is the reverse-DNS keychain service id shared with the TS CLI.
	Service = "app.paulie.agent-mongo"

	StorageKeychain = "keychain"
	StorageConfig   = "config"
)

var keychain = creds.NewKeychain(Service)

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

func Remove(alias string) error {
	cfg := config.Read()
	if _, ok := cfg.Credentials[alias]; !ok {
		valid := strings.Join(Aliases(), ", ")
		if valid == "" {
			valid = "(none)"
		}
		return fmt.Errorf("Unknown credential: %q. Valid: %s", alias, valid)
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

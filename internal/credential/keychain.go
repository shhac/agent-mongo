package credential

import "github.com/shhac/lib-agent-cli/creds"

// keychainStore is the seam over the OS keychain — satisfied by
// *keyring.Keyring in production and by a fake in tests. Every kind that keeps
// a secret in the keychain goes through this one variable, so a test swaps a
// single package-level value rather than threading a store through each call.
type keychainStore interface {
	Available() bool
	Get(account string) (string, bool)
	Set(account, secret string) error
	Delete(account string) error
}

var keychain keychainStore = creds.NewKeychain(Service)

package credential

import (
	"fmt"
	"sync"
	"testing"

	"github.com/shhac/agent-mongo/internal/config"
)

// concurrentWriters is sized to make the lost-update window easy to hit rather
// than to model real usage. Before the config lock these tests left a single
// survivor — and a credential entry lost that way strands its secret in the
// keychain, referenced by nothing and no longer removable through the CLI.
const concurrentWriters = 20

func TestStoreConcurrentWritersAllSurvive(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)

	var wg sync.WaitGroup
	errs := make([]error, concurrentWriters)
	for i := range concurrentWriters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = Store(fmt.Sprintf("cred-%02d", i), config.Credential{
				Username: fmt.Sprintf("svc-user-%02d", i),
				Password: fmt.Sprintf("pw-%02d-zdMk3q", i),
			})
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Store(cred-%02d): %v", i, err)
		}
	}

	entries := config.Read().Credentials
	if len(entries) != concurrentWriters {
		t.Fatalf("got %d credential entries, want %d — concurrent writes were lost", len(entries), concurrentWriters)
	}
	for i := range concurrentWriters {
		alias := fmt.Sprintf("cred-%02d", i)
		if entries[alias].Password != Sentinel {
			t.Errorf("%s should hold the keychain sentinel, got %+v", alias, entries[alias])
		}
		cred, ok := resolves(alias)
		if !ok {
			t.Errorf("%s no longer resolves — its keychain secret is stranded", alias)
			continue
		}
		if want := fmt.Sprintf("pw-%02d-zdMk3q", i); cred.Password != want {
			t.Errorf("%s password = %q, want %q", alias, cred.Password, want)
		}
	}
}

// Store and Remove both touch the keychain inside the config lock; running
// them together checks the two halves never interleave.
func TestConcurrentStoreAndRemoveLeaveConsistentIndex(t *testing.T) {
	isolateConfig(t)
	fake := newFakeKeychain()
	swapKeychain(t, fake)

	// Half the aliases are pre-seeded so their goroutine removes them; the rest
	// are added concurrently.
	for i := range concurrentWriters / 2 {
		alias := fmt.Sprintf("seed-%02d", i)
		if _, err := Store(alias, config.Credential{Username: "seed", Password: "pw-seed-Wq7t"}); err != nil {
			t.Fatalf("seed %s: %v", alias, err)
		}
	}

	var wg sync.WaitGroup
	removeErrs := make([]error, concurrentWriters/2)
	addErrs := make([]error, concurrentWriters/2)
	for i := range concurrentWriters / 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			removeErrs[i] = Remove(fmt.Sprintf("seed-%02d", i))
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, addErrs[i] = Store(fmt.Sprintf("added-%02d", i), config.Credential{
				Username: "added", Password: fmt.Sprintf("pw-add-%02d-Ln4v", i),
			})
		}()
	}
	wg.Wait()

	for i := range concurrentWriters / 2 {
		if removeErrs[i] != nil {
			t.Fatalf("Remove(seed-%02d): %v", i, removeErrs[i])
		}
		if addErrs[i] != nil {
			t.Fatalf("Store(added-%02d): %v", i, addErrs[i])
		}
	}

	entries := config.Read().Credentials
	if len(entries) != concurrentWriters/2 {
		t.Fatalf("got %d entries, want %d: %v", len(entries), concurrentWriters/2, aliasesOf(entries))
	}
	for i := range concurrentWriters / 2 {
		if _, ok := entries[fmt.Sprintf("seed-%02d", i)]; ok {
			t.Errorf("seed-%02d should have been removed", i)
		}
		if _, ok := entries[fmt.Sprintf("added-%02d", i)]; !ok {
			t.Errorf("added-%02d was lost to a concurrent Remove", i)
		}
	}
}

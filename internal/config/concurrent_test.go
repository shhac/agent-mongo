package config

import (
	"fmt"
	"sync"
	"testing"
)

// concurrentWriters is deliberately well above the number of CLI invocations a
// human would ever overlap — it is sized to make the lost-update window easy to
// hit, not to model real usage. Without the lock around read-modify-write these
// tests leave a single survivor.
const concurrentWriters = 20

func TestStoreConnectionConcurrentWritersAllSurvive(t *testing.T) {
	isolate(t)

	var wg sync.WaitGroup
	errs := make([]error, concurrentWriters)
	for i := range concurrentWriters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			alias := fmt.Sprintf("conn-%02d", i)
			errs[i] = StoreConnection(alias, Connection{
				ConnectionString: fmt.Sprintf("mongodb://db%02d.example.invalid:27017", i),
			})
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("StoreConnection(conn-%02d): %v", i, err)
		}
	}

	conns := Read().Connections
	if len(conns) != concurrentWriters {
		t.Fatalf("got %d connections, want %d — concurrent writes were lost", len(conns), concurrentWriters)
	}
	for i := range concurrentWriters {
		alias := fmt.Sprintf("conn-%02d", i)
		want := fmt.Sprintf("mongodb://db%02d.example.invalid:27017", i)
		if got := conns[alias].ConnectionString; got != want {
			t.Errorf("%s connection_string = %q, want %q", alias, got, want)
		}
	}
}

// Connections and settings share one config.json, so a settings write and a
// connection write are the same read-modify-write racing over one file.
func TestSettingsAndConnectionWritesDoNotClobberEachOther(t *testing.T) {
	isolate(t)

	keys := []string{"defaults.limit", "defaults.sampleSize", "query.timeout", "truncation.maxLength"}
	values := map[string]int{
		"defaults.limit":       50,
		"defaults.sampleSize":  25,
		"query.timeout":        9000,
		"truncation.maxLength": 500,
	}

	var wg sync.WaitGroup
	settingErrs := make([]error, len(keys))
	connErrs := make([]error, concurrentWriters)

	for i, key := range keys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			settingErrs[i] = UpdateSetting(key, values[key])
		}()
	}
	for i := range concurrentWriters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			connErrs[i] = StoreConnection(fmt.Sprintf("mixed-%02d", i), Connection{
				ConnectionString: "mongodb://mixed.example.invalid:27017",
			})
		}()
	}
	wg.Wait()

	for i, err := range settingErrs {
		if err != nil {
			t.Fatalf("UpdateSetting(%s): %v", keys[i], err)
		}
	}
	for i, err := range connErrs {
		if err != nil {
			t.Fatalf("StoreConnection(mixed-%02d): %v", i, err)
		}
	}

	if got := len(Read().Connections); got != concurrentWriters {
		t.Errorf("got %d connections, want %d — connection writes lost to settings writes", got, concurrentWriters)
	}
	for _, key := range keys {
		got, ok := GetSetting(key)
		if !ok || got != values[key] {
			t.Errorf("setting %s = %d (set=%v), want %d — settings write lost", key, got, ok, values[key])
		}
	}
}

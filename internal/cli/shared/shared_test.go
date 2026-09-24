package shared

import (
	"testing"
	"time"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/testutil"
)

func set(t *testing.T, setting *config.SettingDef, value int) {
	t.Helper()
	if err := setting.Set(value); err != nil {
		t.Fatal(err)
	}
}

func TestTimeoutPrecedence(t *testing.T) {
	testutil.IsolateConfig(t)
	cases := []struct {
		flag int
		want time.Duration
	}{
		{0, 30 * time.Second},
		{-5, 30 * time.Second}, // not a timeout anyone meant
		{1500, 1500 * time.Millisecond},
	}
	for _, tc := range cases {
		if got := (&GlobalFlags{TimeoutMS: tc.flag}).Timeout(); got != tc.want {
			t.Errorf("-t %d: %v, want %v", tc.flag, got, tc.want)
		}
	}

	set(t, config.QueryTimeout, 5000)
	if got := (&GlobalFlags{}).Timeout(); got != 5*time.Second {
		t.Errorf("configured: %v, want 5s", got)
	}
	if got := (&GlobalFlags{TimeoutMS: 2000}).Timeout(); got != 2*time.Second {
		t.Errorf("-t beats config: %v, want 2s", got)
	}
}

// Every count an agent asks for is held to query.maxDocuments, whether it came
// from a flag or a configured default.
func TestCappedCount(t *testing.T) {
	testutil.IsolateConfig(t)
	set(t, config.MaxDocuments, 50)

	if got := CappedCount(0, config.DefaultLimit); got != 20 {
		t.Errorf("default: %d, want 20", got)
	}
	if got := CappedCount(500, config.DefaultLimit); got != 50 {
		t.Errorf("flag over the cap: %d, want 50", got)
	}
	set(t, config.DefaultSampleSize, 80)
	if got := CappedCount(0, config.DefaultSampleSize); got != 50 {
		t.Errorf("configured default over the cap: %d, want 50", got)
	}
	if got := SettingDefault(0, config.DefaultSchemaSampleSize); got != 100 {
		t.Errorf("uncapped default: %d, want 100", got)
	}
	if got := SettingDefault(700, config.DefaultSchemaSampleSize); got != 700 {
		t.Errorf("uncapped flag: %d, want 700", got)
	}
}

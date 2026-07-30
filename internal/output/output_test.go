package output

import (
	"strings"
	"testing"

	_ "github.com/shhac/lib-agent-cli/yaml" // registers the --format yaml encoder

	"github.com/shhac/agent-mongo/internal/serialize"
	"github.com/shhac/agent-mongo/internal/testutil"
)

// captureStdout runs fn with stdout captured, returning what it wrote.
func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	buf, restore := testutil.CaptureStdout(t)
	err := fn()
	restore()
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	return buf.String()
}

// spec stands in for an index spec: a compound key whose real order is the
// reverse of alphabetical, and a null-valued field. Both are load-bearing.
func spec() serialize.Ordered {
	return serialize.Ordered{
		{Key: "name", Value: "status_1_expiryDate_1"},
		{Key: "key", Value: serialize.Ordered{
			{Key: "status", Value: 1},
			{Key: "expiryDate", Value: 1},
		}},
		{Key: "deletedAt", Value: nil},
	}
}

// The regression this guards: routing `collection indexes` back through
// PrintList would alphabetize the compound key (making it disagree with the
// index name) and drop the null clause. Every format must hold the line, since
// NDJSON and the json/yaml envelope are different branches in lib-agent-output.
func TestPrintListVerbatimPreservesOrderAndNullsInEveryFormat(t *testing.T) {
	tests := []struct {
		format string
		want   []string
	}{
		{"jsonl", []string{`{"name":"status_1_expiryDate_1","key":{"status":1,"expiryDate":1},"deletedAt":null}`}},
		{"json", []string{`"status": 1`, `"expiryDate": 1`, `"deletedAt": null`}},
		{"yaml", []string{"status: 1\n", "expiryDate: 1\n", "deletedAt: null\n"}},
	}

	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			ConfigureFormat(tc.format)
			t.Cleanup(func() { ConfigureFormat("") })

			got := captureStdout(t, func() error {
				return PrintListVerbatim([]serialize.Ordered{spec()}, nil)
			})
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in %s output:\n%s", want, tc.format, got)
				}
			}
			if statusAt, expiryAt := strings.Index(got, "status"), strings.Index(got, "expiryDate"); statusAt > expiryAt {
				t.Errorf("compound key was reordered in %s output:\n%s", tc.format, got)
			}
		})
	}
}

// The other half of the contract: document output is still normalized. If this
// ever matches the verbatim expectations above, the two paths have collapsed
// and index specs are no longer distinguishable from documents.
func TestPrintListStillNormalizes(t *testing.T) {
	ConfigureFormat("jsonl")
	t.Cleanup(func() { ConfigureFormat("") })

	got := captureStdout(t, func() error {
		return PrintList([]map[string]any{{
			"name":      "status_1_expiryDate_1",
			"key":       map[string]any{"status": 1, "expiryDate": 1},
			"deletedAt": nil,
		}}, nil)
	})
	want := `{"key":{"expiryDate":1,"status":1},"name":"status_1_expiryDate_1"}`
	if strings.TrimSpace(got) != want {
		t.Errorf("PrintList should sort keys and prune nulls\n got: %s\nwant: %s", strings.TrimSpace(got), want)
	}
}

// A verbatim record must never be silently normalized. The normalizing
// printers reject it outright, so mis-wiring a command is a loud error rather
// than plausible-looking output with the wrong key order.
func TestNormalizingPrintersRejectVerbatimRecords(t *testing.T) {
	ConfigureFormat("jsonl")
	t.Cleanup(func() { ConfigureFormat("") })

	tests := map[string]func() error{
		"PrintList":   func() error { return PrintList([]serialize.Ordered{spec()}, nil) },
		"PrintResult": func() error { return PrintResult(spec()) },
		"PrintRaw":    func() error { return PrintRaw(spec()) },
		// An empty slice still carries the element type, so the call site is
		// wrong even when there is nothing to print.
		"PrintList empty": func() error { return PrintList([]serialize.Ordered{}, nil) },
	}

	for name, print := range tests {
		t.Run(name, func(t *testing.T) {
			buf, restore := testutil.CaptureStdout(t)
			err := print()
			restore()

			if err == nil {
				t.Fatalf("%s accepted a verbatim record; output was:\n%s", name, buf.String())
			}
			if !strings.Contains(err.Error(), "PrintListVerbatim") {
				t.Errorf("error should name the right printer, got: %v", err)
			}
			if buf.String() != "" {
				t.Errorf("rejected record still wrote to stdout:\n%s", buf.String())
			}
		})
	}
}

// The guard must not fire on ordinary records, or every other command breaks.
func TestNormalizingPrintersAcceptOrdinaryRecords(t *testing.T) {
	ConfigureFormat("jsonl")
	t.Cleanup(func() { ConfigureFormat("") })

	got := captureStdout(t, func() error {
		return PrintList([]map[string]any{{"name": "users"}}, nil)
	})
	if !strings.Contains(got, `"name":"users"`) {
		t.Errorf("ordinary record did not print:\n%s", got)
	}
}

// Truncation is a normalization too — an index spec must not acquire an
// ellipsis or a {field}Length companion.
func TestPrintListVerbatimDoesNotTruncate(t *testing.T) {
	ConfigureFormat("jsonl")
	t.Cleanup(func() { ConfigureFormat("") })

	long := strings.Repeat("x", 500)
	got := captureStdout(t, func() error {
		return PrintListVerbatim([]serialize.Ordered{{{Key: "name", Value: long}}}, nil)
	})
	if !strings.Contains(got, long) {
		t.Errorf("value was truncated:\n%s", got)
	}
	if strings.Contains(got, "nameLength") {
		t.Errorf("truncation companion key leaked into a verbatim record:\n%s", got)
	}
}

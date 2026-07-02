package output

import (
	"reflect"
	"testing"

	out "github.com/shhac/lib-agent-output"
)

// TestPruneEmptyRemovesEmptyValues ports test/compact-json.test.ts, which
// pinned the behavior of the TS pruneEmpty helper. That helper has been
// replaced by lib-agent-output's out.PruneEmpty (see internal/output/output.go
// pruneTruncate). Most expectations carry over unchanged.
//
// DIVERGENCE — slice element pruning. TS pruneEmpty recursively pruned array
// elements and dropped empties (null, "", {}), so
//
//	[null, "", 2, {z:""}, {z:"a"}]  ->  [2, {z:"a"}]
//
// out.PruneEmpty instead only drops nil elements from a slice; empty strings
// and (recursively-emptied) maps are retained:
//
//	[null, "", 2, {z:""}, {z:"a"}]  ->  ["", 2, {}, {z:"a"}]
//
// Per the migration brief we do not modify the library; this test asserts the
// library's actual (retained-empties) behavior for the `k` slice and documents
// the difference here.
func TestPruneEmptyRemovesEmptyValues(t *testing.T) {
	input := map[string]any{
		"a": 1,
		"b": nil,
		// "c": undefined has no Go analogue; omitted.
		"d": "",
		"e": "  ",
		"f": "hello",
		"g": []any{},
		"h": map[string]any{},
		"i": map[string]any{"x": 1},
		"j": map[string]any{"nested": nil, "keep": "ok"},
		"k": []any{nil, "", 2, map[string]any{"z": ""}, map[string]any{"z": "a"}},
		"l": 0,
		"m": false,
		"n": true,
	}

	want := map[string]any{
		"a": 1,
		"f": "hello",
		"i": map[string]any{"x": 1},
		"j": map[string]any{"keep": "ok"},
		// Divergence: empty string and empty map retained inside the slice.
		"k": []any{"", 2, map[string]any{}, map[string]any{"z": "a"}},
		"l": 0,
		"m": false,
		"n": true,
	}

	got := out.PruneEmpty(input)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PruneEmpty() mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestPruneEmptyReturnsEmptyObjectForFullyEmptyInput(t *testing.T) {
	input := map[string]any{"a": nil, "b": "", "c": "  "}
	got := out.PruneEmpty(input)
	want := map[string]any{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PruneEmpty() = %#v, want empty map", got)
	}
}

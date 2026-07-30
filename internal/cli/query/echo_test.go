package query

import (
	"encoding/json"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// enableEcho turns the flag on for one test. The flag is package state (cobra
// binds it once on the group), so it must be restored.
func enableEcho(t *testing.T) {
	t.Helper()
	prev := echoEnabled
	echoEnabled = true
	t.Cleanup(func() { echoEnabled = prev })
}

func encodeMeta(t *testing.T, meta map[string]any) string {
	t.Helper()
	b, err := json.Marshal(meta[MetaKeyQuery])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// The echo exists so a caller can verify what actually ran. That is only worth
// anything if the echo is faithful: field order and null clauses both change
// what a filter means.
func TestEchoIsFaithfulToTheQuery(t *testing.T) {
	enableEcho(t)

	tests := []struct {
		name  string
		build func(e *echo)
		want  string
	}{
		{
			name: "filter key order is not alphabetized",
			build: func(e *echo) {
				e.doc("filter", bson.D{{Key: "status", Value: "pending"}, {Key: "amount", Value: 1}})
			},
			want: `{"filter":{"status":"pending","amount":1}}`,
		},
		{
			name: "a null clause survives",
			build: func(e *echo) {
				e.doc("filter", bson.D{{Key: "deletedAt", Value: nil}})
			},
			want: `{"filter":{"deletedAt":null}}`,
		},
		{
			name: "$in element order is preserved",
			build: func(e *echo) {
				e.doc("filter", bson.D{{Key: "status", Value: bson.D{
					{Key: "$in", Value: bson.A{"pending", "confirmed"}},
				}}})
			},
			want: `{"filter":{"status":{"$in":["pending","confirmed"]}}}`,
		},
		{
			name: "parts appear in the order the command recorded them",
			build: func(e *echo) {
				e.doc("filter", bson.D{{Key: "a", Value: 1}})
				e.doc("sort", bson.D{{Key: "_id", Value: -1}})
				e.num("limit", 20)
				e.num("skip", 5)
			},
			want: `{"filter":{"a":1},"sort":{"_id":-1},"limit":20,"skip":5}`,
		},
		{
			name: "unset options are omitted, not echoed as zero",
			build: func(e *echo) {
				e.doc("filter", bson.D{{Key: "a", Value: 1}})
				e.doc("sort", nil)
				e.num("skip", 0)
			},
			want: `{"filter":{"a":1}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var e echo
			tc.build(&e)
			if got := encodeMeta(t, e.meta()); got != tc.want {
				t.Errorf("\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// Opt-in means opt-in: without the flag there is no @query entry at all, so
// existing output is byte-identical.
func TestEchoIsSilentUnlessRequested(t *testing.T) {
	var e echo
	e.doc("filter", bson.D{{Key: "a", Value: 1}})
	if meta := e.meta(); meta != nil {
		t.Errorf("echo emitted %v with the flag off", meta)
	}
}

// A command that recorded nothing should not emit an empty @query line.
func TestEchoOmitsAnEmptyRecord(t *testing.T) {
	enableEcho(t)
	var e echo
	if meta := e.meta(); meta != nil {
		t.Errorf("expected no entry for an empty echo, got %v", meta)
	}
}

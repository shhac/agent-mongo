package query

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
	"github.com/shhac/agent-mongo/internal/testutil"
)

// These cover the echo wiring for every query command without a live session —
// previously only three of six were exercised at all, and only by the
// docker-gated integration suite.

func capture(t *testing.T, fn func() error) string {
	t.Helper()
	output.ConfigureFormat("jsonl")
	t.Cleanup(func() { output.ConfigureFormat("") })

	buf, restore := testutil.CaptureStdout(t)
	err := fn()
	restore()
	if err != nil {
		t.Fatalf("print: %v", err)
	}
	return buf.String()
}

func ref() mongo.Ref { return mongo.Ref{DB: "myapp", Collection: "orders"} }

// A filter whose key order is the reverse of alphabetical and whose null clause
// would vanish under pruning — the two failure modes the echo exists to avoid.
func trickyFilter() bson.D {
	return bson.D{{Key: "status", Value: "pending"}, {Key: "deletedAt", Value: nil}}
}

func TestEveryCommandEchoesFaithfully(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "find",
			run: func() error {
				return printFind(mongo.FindResult{}, ref(), findEcho{
					filter: trickyFilter(),
					sort:   bson.D{{Key: "_id", Value: -1}},
					limit:  20,
				})
			},
			want: `{"@query":{"filter":{"status":"pending","deletedAt":null},"sort":{"_id":-1},"limit":20}}`,
		},
		{
			name: "count",
			run:  func() error { return printCount(7, ref(), trickyFilter()) },
			want: `{"@query":{"filter":{"status":"pending","deletedAt":null}}}`,
		},
		{
			name: "sample",
			run:  func() error { return printSample(nil, ref(), trickyFilter(), 5) },
			want: `{"@query":{"filter":{"status":"pending","deletedAt":null},"size":5}}`,
		},
		{
			name: "distinct",
			run:  func() error { return printDistinct(nil, ref(), "status", trickyFilter()) },
			want: `{"@query":{"field":"status","filter":{"status":"pending","deletedAt":null}}}`,
		},
		{
			name: "get",
			run: func() error {
				return printGet(map[string]any{"_id": "x"}, ref(), "abc", "string",
					bson.D{{Key: "name", Value: 1}, {Key: "age", Value: 1}})
			},
			want: `{"@query":{"id":"abc","idType":"string","projection":{"name":1,"age":1}}}`,
		},
		{
			name: "aggregate",
			run: func() error {
				return printAggregate(nil, ref(), bson.A{
					bson.D{{Key: "$match", Value: bson.D{{Key: "status", Value: "pending"}, {Key: "deletedAt", Value: nil}}}},
				}, 20)
			},
			want: `{"@query":{"pipeline":[{"$match":{"status":"pending","deletedAt":null}}],"limit":20}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enableEcho(t)
			got := capture(t, tc.run)
			if !strings.Contains(got, tc.want) {
				t.Errorf("echo missing or altered\n want: %s\n got:\n%s", tc.want, got)
			}
		})
	}
}

// Without the flag, every command's output must be byte-identical to before the
// feature existed.
func TestNoCommandEchoesWhenTheFlagIsOff(t *testing.T) {
	runs := map[string]func() error{
		"find":      func() error { return printFind(mongo.FindResult{}, ref(), findEcho{filter: trickyFilter()}) },
		"count":     func() error { return printCount(7, ref(), trickyFilter()) },
		"sample":    func() error { return printSample(nil, ref(), trickyFilter(), 5) },
		"distinct":  func() error { return printDistinct(nil, ref(), "status", trickyFilter()) },
		"get":       func() error { return printGet(map[string]any{"_id": "x"}, ref(), "abc", "", nil) },
		"aggregate": func() error { return printAggregate(nil, ref(), bson.A{}, 20) },
	}

	for name, run := range runs {
		t.Run(name, func(t *testing.T) {
			if got := capture(t, run); strings.Contains(got, "@query") {
				t.Errorf("%s echoed without the flag:\n%s", name, got)
			}
		})
	}
}

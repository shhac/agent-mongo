package query

import (
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/testutil"
)

func TestResolvePipelinePrecedence(t *testing.T) {
	piped := strings.NewReader(`[{"$count":"from-stdin"}]`)
	cases := []struct {
		positional, flag string
		want             string
	}{
		{`[{"$count":"positional"}]`, `[{"$count":"flag"}]`, "positional"},
		{"", `[{"$count":"flag"}]`, "flag"},
		{"", "", "from-stdin"},
	}
	for _, tc := range cases {
		pipeline, err := resolvePipeline(tc.positional, tc.flag, piped)
		if err != nil {
			t.Fatal(err)
		}
		if got := pipeline[0].(bson.D)[0].Value; got != tc.want {
			t.Errorf("took %v, want %s", got, tc.want)
		}
	}

	if _, err := resolvePipeline("", "", strings.NewReader("  \n")); err == nil ||
		!strings.Contains(err.Error(), "Empty stdin") {
		t.Errorf("empty stdin: %v", err)
	}
}

func TestAggregateLimit(t *testing.T) {
	testutil.IsolateConfig(t)
	own := bson.A{bson.D{{Key: "$limit", Value: 500}}}
	if got := aggregateLimit(own, 5); got != 100 {
		t.Errorf("own $limit: capped at %d, want query.maxDocuments (100); --limit does not apply", got)
	}
	if got := aggregateLimit(bson.A{}, 5); got != 5 {
		t.Errorf("--limit: %d, want 5", got)
	}
	if got := aggregateLimit(bson.A{}, 0); got != 20 {
		t.Errorf("default: %d, want 20", got)
	}
}

package collection

import (
	"strings"
	"testing"

	"github.com/shhac/agent-mongo/internal/mongo"
	"github.com/shhac/agent-mongo/internal/output"
	"github.com/shhac/agent-mongo/internal/serialize"
	"github.com/shhac/agent-mongo/internal/testutil"
)

// Pins the command's print policy under `make test`. The regression this
// catches is a one-line swap to output.PrintList, which would alphabetize the
// compound key — making it disagree with the index name — and drop the null
// clause that makes a partial index partial.
func TestPrintIndexesEmitsSpecsVerbatim(t *testing.T) {
	output.ConfigureFormat("jsonl")
	t.Cleanup(func() { output.ConfigureFormat("") })

	indexes := []serialize.Ordered{{
		{Key: "name", Value: "status_1_expiryDate_1"},
		{Key: "key", Value: serialize.Ordered{
			{Key: "status", Value: 1},
			{Key: "expiryDate", Value: 1},
		}},
		{Key: "partialFilterExpression", Value: serialize.Ordered{
			{Key: "deletedAt", Value: nil},
		}},
	}}

	buf, restore := testutil.CaptureStdout(t)
	err := printIndexes(indexes, mongo.Ref{DB: "myapp", Collection: "reservations"})
	restore()
	if err != nil {
		t.Fatalf("printIndexes: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	want := `{"name":"status_1_expiryDate_1","key":{"status":1,"expiryDate":1},` +
		`"partialFilterExpression":{"deletedAt":null}}`
	if lines[0] != want {
		t.Errorf("spec was normalized\n got: %s\nwant: %s", lines[0], want)
	}
	if len(lines) != 2 || !strings.Contains(lines[1], `"@meta"`) {
		t.Errorf("expected a trailing @meta line, got: %v", lines)
	}
}

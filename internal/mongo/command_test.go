package mongo

import (
	"slices"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func keys(doc bson.D) []string {
	out := make([]string, len(doc))
	for i, e := range doc {
		out[i] = e.Key
	}
	return out
}

func lookup(t *testing.T, doc bson.D, key string) any {
	t.Helper()
	for _, e := range doc {
		if e.Key == key {
			return e.Value
		}
	}
	t.Fatalf("%q missing from %v", key, doc)
	return nil
}

// A command sent through RunCommand gets the driver's maxTimeMS; one carrying
// its own is undefined behaviour under a client timeout.
func TestCommandsLeaveMaxTimeToTheDriver(t *testing.T) {
	commands := map[string]bson.D{
		"find":      findCommand(FindOpts{Ref: Ref{Collection: "c"}, Limit: 5}),
		"aggregate": aggregateCommand("c", bson.A{}, 5),
	}
	for name, cmd := range commands {
		for _, key := range keys(cmd) {
			if key == "maxTimeMS" {
				t.Errorf("%s carries its own maxTimeMS: %v", name, cmd)
			}
		}
	}
}

func TestFindCommandFetchesOnePastTheLimitInOneBatch(t *testing.T) {
	cmd := findCommand(FindOpts{
		Ref:        Ref{DB: "db", Collection: "users"},
		Filter:     bson.D{{Key: "age", Value: 30}},
		Sort:       bson.D{{Key: "_id", Value: -1}},
		Projection: bson.D{{Key: "name", Value: 1}},
		Limit:      20,
		Skip:       40,
	})

	if cmd[0].Key != "find" || cmd[0].Value != "users" {
		t.Fatalf("the command name must come first: %v", cmd)
	}
	if got := lookup(t, cmd, "limit"); got != int64(21) {
		t.Errorf("limit = %v, want 21", got)
	}
	if got := lookup(t, cmd, "batchSize"); got != int64(21) {
		t.Errorf("batchSize = %v, want 21", got)
	}
	if got := lookup(t, cmd, "skip"); got != int64(40) {
		t.Errorf("skip = %v, want 40", got)
	}
}

func TestFindCommandOmitsUnsetOptions(t *testing.T) {
	cmd := findCommand(FindOpts{Ref: Ref{Collection: "users"}, Limit: 1})
	want := []string{"find", "filter", "limit", "batchSize"}
	if got := keys(cmd); !slices.Equal(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
	if filter, ok := lookup(t, cmd, "filter").(bson.D); !ok || filter == nil {
		t.Errorf("an absent filter must be sent as {}, got %#v", lookup(t, cmd, "filter"))
	}
}

func TestAggregateCommandBatchOutgrowsTheResult(t *testing.T) {
	cursor := lookup(t, aggregateCommand("c", bson.A{}, 20), "cursor").(bson.D)
	if got := lookup(t, cursor, "batchSize"); got != int64(21) {
		t.Errorf("batchSize = %v, want 21 so the first batch closes the cursor", got)
	}

	cursor = lookup(t, aggregateCommand("c", bson.A{}, 0), "cursor").(bson.D)
	if len(cursor) != 0 {
		t.Errorf("an unknown size must leave the server's batch size: %v", cursor)
	}
}

func TestTagAddsTheCommentOnlyWhenSet(t *testing.T) {
	cmd := bson.D{{Key: "dbStats", Value: 1}}
	if got := (&Session{}).tag(cmd); len(got) != 1 {
		t.Errorf("untagged session added fields: %v", got)
	}
	got := (&Session{comment: "agent-mongo database stats"}).tag(cmd)
	if lookup(t, got, "comment") != "agent-mongo database stats" {
		t.Errorf("comment missing: %v", got)
	}
}

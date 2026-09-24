package mongo

import (
	"slices"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/shhac/agent-mongo/internal/config"
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

func TestKeepLimit(t *testing.T) {
	docs := []bson.D{{}, {}, {}}
	if kept, more := keepLimit(docs, 2); len(kept) != 2 || !more {
		t.Errorf("one past the limit: %d kept, more=%v", len(kept), more)
	}
	if kept, more := keepLimit(docs, 3); len(kept) != 3 || more {
		t.Errorf("exactly the limit: %d kept, more=%v", len(kept), more)
	}
}

// $sample draws from what reaches it, so the filter has to come first.
func TestSamplePipelineMatchesBeforeSampling(t *testing.T) {
	filter := bson.D{{Key: "status", Value: "active"}}
	got := samplePipeline(filter, 5)
	if len(got) != 2 || got[0].(bson.D)[0].Key != "$match" || got[1].(bson.D)[0].Key != "$sample" {
		t.Errorf("pipeline = %v", got)
	}
	if got := samplePipeline(nil, 5); len(got) != 1 || got[0].(bson.D)[0].Key != "$sample" {
		t.Errorf("unfiltered pipeline = %v", got)
	}
}

func TestHasLimitStageLooksAtTheTopLevelOnly(t *testing.T) {
	limit := bson.D{{Key: "$limit", Value: 5}}
	if !HasLimitStage(bson.A{bson.D{{Key: "$match", Value: bson.D{}}}, limit}) {
		t.Error("a top-level $limit was missed")
	}
	nested := bson.A{bson.D{{Key: "$facet", Value: bson.D{{Key: "a", Value: bson.A{limit}}}}}}
	if HasLimitStage(nested) {
		t.Error("a $limit inside $facet bounds one facet, not the result")
	}
}

// RunCommand applies neither, so what the connection string asked for has to
// ride on the session.
func TestNewSessionKeepsReadPreferenceAndConcern(t *testing.T) {
	conn := config.Connection{
		ConnectionString: "mongodb://localhost/app?readPreference=secondary&readConcernLevel=majority",
	}
	opts, err := clientOptions(conn, ConnectOpts{})
	if err != nil {
		t.Fatal(err)
	}
	session := newSession(nil, "prod", conn, opts, "agent-mongo query find")
	if session.readPref == nil || session.readPref.Mode().String() != "secondary" {
		t.Errorf("readPref = %v, want secondary", session.readPref)
	}
	if session.readConcern != "majority" {
		t.Errorf("readConcern = %q, want majority", session.readConcern)
	}
	if session.DBName != "app" {
		t.Errorf("DBName = %q", session.DBName)
	}
}

func TestStatsRecords(t *testing.T) {
	coll := collectionStatsRecord(Ref{DB: "app", Collection: "users"}, bson.M{"count": 3, "nindexes": 2})
	if coll["documentCount"] != 3 || coll["indexes"] != 2 || coll["capped"] != false {
		t.Errorf("collection stats = %v", coll)
	}
	db := databaseStatsRecord("app", bson.M{"objects": 7, "collections": 2})
	if db["documents"] != 7 || db["collections"] != 2 || db["database"] != "app" {
		t.Errorf("database stats = %v", db)
	}
}

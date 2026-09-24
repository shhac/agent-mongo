//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// slowFilter makes every scanned order cost 100ms of server time, so a scan of
// the 150 seeded orders runs for ~15s — far past the 1s timeout the tests use.
// $function rather than $where, which aggregate's $match (and so count and
// sample) refuses.
const slowFilter = `{"$expr":{"$function":{"body":"function() { sleep(100); return true }","args":[],"lang":"js"}}}`

func adminClient(t *testing.T) *driver.Client {
	t.Helper()
	client, err := driver.Connect(options.Client().ApplyURI(testURI).
		SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return client
}

// ordersOps lists in-progress operations against the seeded orders collection.
func ordersOps(t *testing.T, client *driver.Client) []bson.M {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cursor, err := client.Database("admin").Aggregate(ctx, bson.A{
		bson.D{{Key: "$currentOp", Value: bson.D{{Key: "allUsers", Value: true}}}},
		bson.D{{Key: "$match", Value: bson.D{{Key: "ns", Value: testDB + ".orders"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var ops []bson.M
	if err := cursor.All(ctx, &ops); err != nil {
		t.Fatal(err)
	}
	return ops
}

func killOrdersOps(t *testing.T, client *driver.Client) {
	for _, op := range ordersOps(t, client) {
		_ = client.Database("admin").RunCommand(context.Background(),
			bson.D{{Key: "killOp", Value: 1}, {Key: "op", Value: op["opid"]}}).Err()
	}
}

// A query the CLI gives up on must not keep running on the server. The client
// abandoning its socket does not stop mongod, so the only thing that can is the
// maxTimeMS the command carried.
func TestTimedOutQueryStopsOnTheServer(t *testing.T) {
	h := home(t)
	client := adminClient(t)

	cases := map[string][]string{
		"find":      {"query", "find", testDB, "orders", "--limit", "100", "--filter", slowFilter},
		"aggregate": {"query", "aggregate", testDB, "orders", "--limit", "100", `[{"$match":` + slowFilter + `}]`},
		"sample":    {"query", "sample", testDB, "orders", "--filter", slowFilter},
		"count":     {"query", "count", testDB, "orders", "--filter", slowFilter},
		"distinct":  {"query", "distinct", testDB, "orders", "status", "--filter", slowFilter},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { killOrdersOps(t, client) })

			started := time.Now()
			r := runIn(t, h, append([]string{"-t", "1000"}, args...)...)
			if r.exitCode != 1 || !strings.Contains(r.stderr, "timed out") {
				t.Fatalf("want a timeout error: exit=%d stderr=%s", r.exitCode, r.stderr)
			}
			if elapsed := time.Since(started); elapsed > 5*time.Second {
				t.Errorf("CLI took %v to give up on a 1s timeout", elapsed)
			}

			deadline := time.Now().Add(3 * time.Second)
			for {
				ops := ordersOps(t, client)
				if len(ops) == 0 {
					return
				}
				if time.Now().After(deadline) {
					t.Fatalf("query still running on the server after the CLI timed out: %v", ops[0]["command"])
				}
				time.Sleep(100 * time.Millisecond)
			}
		})
	}
}

// Every data command is tagged so a DBA reading the server log or currentOp can
// tell where it came from, and bounded so the server stops it on its own.
func TestCommandsCarryMaxTimeAndComment(t *testing.T) {
	h := home(t)
	client := adminClient(t)
	db := client.Database(testDB)
	ctx := context.Background()

	if err := db.RunCommand(ctx, bson.D{{Key: "profile", Value: 2}}).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.RunCommand(ctx, bson.D{{Key: "profile", Value: 0}}).Err()
		_ = db.Collection("system.profile").Drop(ctx)
	})

	commands := map[string][]string{
		"find":      {"query", "find", testDB, "orders", "--limit", "3"},
		"aggregate": {"query", "aggregate", testDB, "orders", `[{"$match":{"status":"done"}}]`},
		"sample":    {"query", "sample", testDB, "orders"},
		"count":     {"query", "count", testDB, "orders", "--filter", `{"status":"done"}`},
		"distinct":  {"query", "distinct", testDB, "orders", "status"},
	}
	for name, args := range commands {
		if r := runIn(t, h, args...); r.exitCode != 0 {
			t.Fatalf("%s: exit=%d stderr=%s", name, r.exitCode, r.stderr)
		}
	}

	cursor, err := db.Collection("system.profile").Find(ctx, bson.D{{Key: "ns", Value: testDB + ".orders"}})
	if err != nil {
		t.Fatal(err)
	}
	var entries []bson.Raw
	if err := cursor.All(ctx, &entries); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		command, ok := entry.Lookup("command").DocumentOK()
		if !ok {
			continue
		}
		first, err := command.IndexErr(0)
		if err != nil {
			continue
		}
		name := first.Key()
		if name == "getMore" {
			t.Errorf("a result needed a getMore; it should fit the first batch: %v", command)
			continue
		}
		seen[name] = true
		if _, err := command.LookupErr("maxTimeMS"); err != nil {
			t.Errorf("%s sent without maxTimeMS: %v", name, command)
		}
		if comment, _ := command.Lookup("comment").StringValueOK(); !strings.HasPrefix(comment, "agent-mongo query ") {
			t.Errorf("%s comment = %v, want the command path", name, command.Lookup("comment"))
		}
		if appName, _ := entry.Lookup("appName").StringValueOK(); !strings.HasPrefix(appName, "agent-mongo/") {
			t.Errorf("%s appName = %q", name, appName)
		}
	}
	for _, name := range []string{"find", "aggregate", "distinct"} {
		if !seen[name] {
			t.Errorf("no %s reached the profiler: %v", name, entries)
		}
	}
}

// find and aggregate go through RunCommand, which ignores the connection
// string's read concern unless it is forwarded.
func TestConnectionReadConcernIsForwarded(t *testing.T) {
	h := t.TempDir()
	if r := runIn(t, h, "connection", "add", "rc", testURI+"/"+testDB+"?readConcernLevel=majority", "--default"); r.exitCode != 0 {
		t.Fatalf("connection add: %s", r.stderr)
	}
	client := adminClient(t)
	db := client.Database(testDB)
	ctx := context.Background()
	if err := db.RunCommand(ctx, bson.D{{Key: "profile", Value: 2}}).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.RunCommand(ctx, bson.D{{Key: "profile", Value: 0}}).Err()
		_ = db.Collection("system.profile").Drop(ctx)
	})

	if r := runIn(t, h, "query", "find", testDB, "users", "--limit", "1"); r.exitCode != 0 {
		t.Fatalf("find: %s", r.stderr)
	}

	var entry bson.Raw
	err := db.Collection("system.profile").FindOne(ctx, bson.D{{Key: "command.find", Value: "users"}}).Decode(&entry)
	if err != nil {
		t.Fatal(err)
	}
	level, _ := entry.Lookup("command", "readConcern", "level").StringValueOK()
	if level != "majority" {
		t.Errorf("readConcern level = %q, want majority: %v", level, entry.Lookup("command"))
	}
}

// A pipeline's own $limit does not lift the cap every query is held to.
func TestAggregateIsCappedAtMaxDocuments(t *testing.T) {
	h := home(t)

	r := runIn(t, h, "query", "aggregate", testDB, "orders", `[{"$limit":500}]`)
	items, meta := r.records(t)
	if len(items) != 100 || meta["@pagination"]["has_more"] != true {
		t.Errorf("own $limit: %d items, pagination %v", len(items), meta["@pagination"])
	}

	r = runIn(t, h, "query", "aggregate", testDB, "orders", "--limit", "5", `[{"$match":{}}]`)
	items, meta = r.records(t)
	if len(items) != 5 || meta["@pagination"]["has_more"] != true {
		t.Errorf("--limit: %d items, pagination %v", len(items), meta["@pagination"])
	}

	r = runIn(t, h, "query", "aggregate", testDB, "orders", `[{"$count":"n"}]`)
	items, meta = r.records(t)
	if len(items) != 1 || meta["@pagination"] != nil {
		t.Errorf("uncapped result: %d items, pagination %v", len(items), meta["@pagination"])
	}
}

// A server that cannot be reached is a connection problem, reported as one
// within the timeout — not a slow query sent off to check its indexes.
func TestUnreachableServerIsNotAQueryTimeout(t *testing.T) {
	h := t.TempDir()
	if r := runIn(t, h, "connection", "add", "gone", "mongodb://127.0.0.1:1/"+testDB, "--default"); r.exitCode != 0 {
		t.Fatalf("connection add: %s", r.stderr)
	}

	started := time.Now()
	r := runIn(t, h, "-t", "1000", "query", "find", testDB, "orders")
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("took %v to give up on a 1s timeout", elapsed)
	}
	payload := r.stderrJSON(t)
	hint, _ := payload["hint"].(string)
	if r.exitCode != 1 || payload["fixable_by"] != "retry" || !strings.Contains(hint, "connection test") {
		t.Errorf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}
	if strings.Contains(hint, "indexes") {
		t.Errorf("an unreachable server was diagnosed as a slow query: %s", hint)
	}
}

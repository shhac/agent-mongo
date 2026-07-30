//go:build integration

// Package integration exercises the compiled binary against a real mongod.
// Run via `make test-integration` (starts a throwaway docker mongo:8) or set
// AGENT_MONGO_TEST_URI to a disposable server. Never point this at real data:
// the test drops and reseeds its database.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const testDB = "agent_mongo_it"

var (
	binaryPath string
	testURI    string
)

func TestMain(m *testing.M) {
	testURI = os.Getenv("AGENT_MONGO_TEST_URI")
	if testURI == "" {
		testURI = "mongodb://localhost:27099"
	}

	if err := seed(); err != nil {
		os.Stderr.WriteString("integration: cannot reach mongod at " + testURI + ": " + err.Error() + "\n")
		os.Exit(1)
	}

	dir, err := os.MkdirTemp("", "agent-mongo-it")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	binaryPath = filepath.Join(dir, "agent-mongo")
	build := exec.Command("go", "build", "-o", binaryPath, "../../cmd/agent-mongo")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func seed() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := driver.Connect(options.Client().ApplyURI(testURI).
		SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background())
	if err := client.Ping(ctx, nil); err != nil {
		return err
	}

	db := client.Database(testDB)
	if err := db.Drop(ctx); err != nil {
		return err
	}

	oid, _ := bson.ObjectIDFromHex("665a1b2c3d4e5f6a7b8c9d0e")
	dec, _ := bson.ParseDecimal128("1234.56")
	users := db.Collection("users")
	_, err = users.InsertMany(ctx, []any{
		bson.D{
			{Key: "_id", Value: oid},
			{Key: "name", Value: "alice"},
			{Key: "age", Value: int32(30)},
			{Key: "tags", Value: bson.A{"admin", "dev"}},
			{Key: "joined", Value: bson.NewDateTimeFromTime(time.Date(2024, 1, 15, 10, 30, 0, 123e6, time.UTC))},
			{Key: "balance", Value: dec},
			{Key: "bigNum", Value: int64(9007199254740993)},
			{Key: "blob", Value: bson.Binary{Subtype: 0, Data: []byte("hello world")}},
			{Key: "pattern", Value: bson.Regex{Pattern: "^test", Options: "i"}},
			{Key: "profile", Value: bson.D{{Key: "bio", Value: "hello"}}},
			{Key: "score", Value: 4.5},
			{Key: "active", Value: true},
			{Key: "nothing", Value: nil},
		},
		bson.D{
			{Key: "name", Value: "bob"},
			{Key: "age", Value: int32(25)},
			{Key: "longBio", Value: strings.Repeat("x", 500)},
		},
	})
	if err != nil {
		return err
	}

	// Indexes whose specs are only correct if reported verbatim: a compound key
	// whose real order is the reverse of alphabetical, and a partial filter
	// carrying a null clause and a multi-value $in.
	reservations := db.Collection("reservations")
	if _, err := reservations.Indexes().CreateMany(ctx, []driver.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "expiryDate", Value: 1}}},
		{
			Keys: bson.D{{Key: "participantIds", Value: 1}},
			Options: options.Index().SetPartialFilterExpression(bson.D{
				{Key: "participantIds", Value: bson.D{{Key: "$type", Value: "string"}}},
				{Key: "deletedAt", Value: nil},
				{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"pending", "confirmed"}}}},
			}),
		},
	}); err != nil {
		return err
	}

	orders := make([]any, 150)
	for i := range orders {
		status := "done"
		if i%3 == 0 {
			status = "pending"
		}
		orders[i] = bson.D{
			{Key: "orderNo", Value: int32(i)},
			{Key: "status", Value: status},
			{Key: "amount", Value: int32(i * 10)},
		}
	}
	_, err = db.Collection("orders").InsertMany(ctx, orders)
	return err
}

type result struct {
	stdout   string
	stderr   string
	exitCode int
}

// records parses stdout NDJSON into bare records and @-metadata lines.
func (r result) records(t *testing.T) (items []map[string]any, meta map[string]map[string]any) {
	t.Helper()
	meta = map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(r.stdout), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("invalid NDJSON line %q: %v", line, err)
		}
		isMeta := false
		for key, value := range record {
			if strings.HasPrefix(key, "@") {
				if inner, ok := value.(map[string]any); ok {
					meta[key] = inner
				}
				isMeta = true
			}
		}
		if !isMeta {
			items = append(items, record)
		}
	}
	return items, meta
}

func (r result) stderrJSON(t *testing.T) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(r.stderr)), &payload); err != nil {
		t.Fatalf("invalid stderr JSON %q: %v", r.stderr, err)
	}
	return payload
}

func runIn(t *testing.T, home string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+home,
		"AGENT_MONGO_NO_KEYCHAIN=1",
		"AGENT_MONGO_CONNECTION=",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode}
}

// home creates an isolated config dir with the test connection registered.
func home(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	r := runIn(t, dir, "connection", "add", "it", testURI+"/"+testDB, "--default")
	if r.exitCode != 0 {
		t.Fatalf("connection add failed: %s", r.stderr)
	}
	return dir
}

func TestConnectionTest(t *testing.T) {
	r := runIn(t, home(t), "connection", "test")
	if r.exitCode != 0 {
		t.Fatalf("exit %d: %s", r.exitCode, r.stderr)
	}
	items, _ := r.records(t)
	if len(items) != 1 || items[0]["ok"] != true || items[0]["alias"] != "it" {
		t.Fatalf("unexpected: %v", items)
	}
}

func TestDatabaseListAndStats(t *testing.T) {
	h := home(t)

	r := runIn(t, h, "database", "list")
	items, meta := r.records(t)
	found := false
	for _, db := range items {
		if db["name"] == testDB {
			found = true
		}
	}
	if !found {
		t.Fatalf("database %s not listed: %v", testDB, items)
	}
	if meta["@meta"]["totalSize"] == nil {
		t.Fatal("missing totalSize meta")
	}

	r = runIn(t, h, "database", "stats", testDB)
	items, _ = r.records(t)
	if len(items) != 1 || items[0]["documents"].(float64) != 152 {
		t.Fatalf("unexpected stats: %v", items)
	}
}

func TestCollectionCommands(t *testing.T) {
	h := home(t)

	r := runIn(t, h, "collection", "list", testDB)
	items, _ := r.records(t)
	names := map[string]bool{}
	for _, coll := range items {
		names[coll["name"].(string)] = true
	}
	if !names["users"] || !names["orders"] {
		t.Fatalf("missing collections: %v", items)
	}

	r = runIn(t, h, "collection", "schema", testDB, "users")
	items, meta := r.records(t)
	fieldTypes := map[string]string{}
	for _, field := range items {
		types := field["types"].([]any)
		fieldTypes[field["path"].(string)] = types[0].(string)
	}
	expect := map[string]string{
		"_id": "ObjectId", "age": "int", "balance": "decimal", "bigNum": "long",
		"blob": "binary", "joined": "date", "pattern": "regex", "score": "double",
		"tags": "array", "tags.$": "string", "profile.bio": "string", "nothing": "null",
	}
	for path, want := range expect {
		if fieldTypes[path] != want {
			t.Errorf("schema %s: got %q want %q", path, fieldTypes[path], want)
		}
	}
	if meta["@meta"]["sampleSize"].(float64) != 2 {
		t.Errorf("sampleSize: %v", meta["@meta"])
	}

	r = runIn(t, h, "collection", "schema", testDB, "missing")
	if r.exitCode != 1 || !strings.Contains(r.stderrJSON(t)["error"].(string), `Collection "missing" not found`) {
		t.Errorf("missing collection: exit=%d stderr=%s", r.exitCode, r.stderr)
	}

	r = runIn(t, h, "collection", "indexes", testDB, "users")
	items, _ = r.records(t)
	if len(items) != 1 || items[0]["name"] != "_id_" {
		t.Errorf("indexes: %v", items)
	}

	r = runIn(t, h, "collection", "stats", testDB, "orders")
	items, _ = r.records(t)
	if items[0]["documentCount"].(float64) != 150 {
		t.Errorf("stats: %v", items)
	}
}

// Index specs must survive as bytes: sorting a compound key makes the output
// disagree with the index name, and pruning a null clause out of a partial
// filter loses the only thing that makes the index partial.
func TestCollectionIndexesAreVerbatim(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(
		runIn(t, home(t), "collection", "indexes", testDB, "reservations").stdout), "\n")

	want := []string{
		`{"name":"status_1_expiryDate_1","key":{"status":1,"expiryDate":1}}`,
		`{"name":"participantIds_1","key":{"participantIds":1},"partialFilterExpression":` +
			`{"participantIds":{"$type":"string"},"deletedAt":null,"status":{"$in":["pending","confirmed"]}}}`,
	}
	for _, line := range want {
		if !slices.Contains(lines, line) {
			t.Errorf("missing index line\n want: %s\n got:  %s", line, strings.Join(lines, "\n       "))
		}
	}

	// The json/yaml envelope is a separate branch in lib-agent-output, so it
	// needs its own check that the compound key was not reordered.
	for _, format := range []string{"json", "yaml"} {
		out := runIn(t, home(t), "collection", "indexes", testDB, "reservations", "-f", format).stdout
		if strings.Index(out, "status") > strings.Index(out, "expiryDate") {
			t.Errorf("-f %s reordered the compound key:\n%s", format, out)
		}
		if !strings.Contains(out, "deletedAt") {
			t.Errorf("-f %s dropped the null clause:\n%s", format, out)
		}
	}
}

// The echo must be faithful end-to-end: a caller uses it to confirm what ran,
// so an alphabetized filter or a swallowed null clause would answer with a
// query they did not send.
func TestQueryEcho(t *testing.T) {
	h := home(t)

	t.Run("off by default", func(t *testing.T) {
		r := runIn(t, h, "query", "count", testDB, "orders", "--filter", `{"status":"pending"}`)
		if strings.Contains(r.stdout, "@query") {
			t.Errorf("echo appeared without the flag:\n%s", r.stdout)
		}
	})

	t.Run("filter order and nulls survive", func(t *testing.T) {
		r := runIn(t, h, "query", "count", testDB, "orders",
			"--filter", `{"status":"pending","deletedAt":null}`, "--echo-query")
		want := `{"@query":{"filter":{"status":"pending","deletedAt":null}}}`
		if !slices.Contains(strings.Split(strings.TrimSpace(r.stdout), "\n"), want) {
			t.Errorf("want %s\ngot:\n%s", want, r.stdout)
		}
	})

	t.Run("find echoes effective sort and limit", func(t *testing.T) {
		r := runIn(t, h, "query", "find", testDB, "orders",
			"--filter", `{"status":"pending"}`, "--limit", "1", "--echo-query")
		want := `{"@query":{"filter":{"status":"pending"},"sort":{"_id":-1},"limit":1}}`
		if !slices.Contains(strings.Split(strings.TrimSpace(r.stdout), "\n"), want) {
			t.Errorf("want %s\ngot:\n%s", want, r.stdout)
		}
	})

	t.Run("aggregate echoes stage and field order", func(t *testing.T) {
		r := runIn(t, h, "query", "aggregate", testDB, "orders",
			`[{"$match":{"status":"pending","amount":{"$gte":0}}},{"$count":"n"}]`, "--echo-query")
		if !strings.Contains(r.stdout,
			`"pipeline":[{"$match":{"status":"pending","amount":{"$gte":0}}},{"$count":"n"}]`) {
			t.Errorf("pipeline order lost:\n%s", r.stdout)
		}
	})

	t.Run("single-result echo stays valid in json", func(t *testing.T) {
		r := runIn(t, h, "query", "count", testDB, "orders",
			"--filter", `{"b":1,"a":2}`, "--echo-query", "-f", "json")
		var doc map[string]any
		if err := json.Unmarshal([]byte(r.stdout), &doc); err != nil {
			t.Fatalf("not a single valid JSON document: %v\n%s", err, r.stdout)
		}
		if _, ok := doc["@query"]; !ok {
			t.Errorf("@query missing from the json envelope: %s", r.stdout)
		}
	})
}

func TestQueryCommands(t *testing.T) {
	h := home(t)

	t.Run("get serializes BSON types", func(t *testing.T) {
		r := runIn(t, h, "query", "get", testDB, "users", "665a1b2c3d4e5f6a7b8c9d0e")
		items, _ := r.records(t)
		doc := items[0]["document"].(map[string]any)
		checks := map[string]any{
			"_id": "665a1b2c3d4e5f6a7b8c9d0e", "balance": "1234.56",
			"bigNum": "9007199254740993", "blob": "aGVsbG8gd29ybGQ=",
			"joined": "2024-01-15T10:30:00.123Z", "pattern": "/^test/i", "score": 4.5,
		}
		for key, want := range checks {
			if doc[key] != want {
				t.Errorf("%s: got %v want %v", key, doc[key], want)
			}
		}
		if items[0]["fieldCount"].(float64) != 13 {
			t.Errorf("fieldCount: %v", items[0]["fieldCount"])
		}
		if _, present := doc["nothing"]; present {
			t.Error("null field should be pruned")
		}
	})

	t.Run("get not found", func(t *testing.T) {
		r := runIn(t, h, "query", "get", testDB, "users", "000000000000000000000000")
		if r.exitCode != 1 || !strings.Contains(r.stderrJSON(t)["error"].(string), "Document not found") {
			t.Errorf("exit=%d stderr=%s", r.exitCode, r.stderr)
		}
	})

	t.Run("find with filter sort truncation pagination", func(t *testing.T) {
		r := runIn(t, h, "query", "find", testDB, "users", "--filter", `{"name":"bob"}`)
		items, meta := r.records(t)
		if len(items) != 1 {
			t.Fatalf("items: %v", items)
		}
		bio := items[0]["longBio"].(string)
		if len([]rune(bio)) != 201 || !strings.HasSuffix(bio, "…") {
			t.Errorf("longBio not truncated: %d runes", len([]rune(bio)))
		}
		if items[0]["longBioLength"].(float64) != 500 {
			t.Errorf("longBioLength: %v", items[0]["longBioLength"])
		}
		if meta["@pagination"]["total_items"].(float64) != 1 {
			t.Errorf("pagination: %v", meta["@pagination"])
		}

		r = runIn(t, h, "query", "find", testDB, "users", "--filter", `{"name":"bob"}`, "--full")
		items, _ = r.records(t)
		if len(items[0]["longBio"].(string)) != 500 {
			t.Error("--full should expand truncated fields")
		}
	})

	t.Run("find respects limit and maxDocuments", func(t *testing.T) {
		r := runIn(t, h, "query", "find", testDB, "orders", "--limit", "5")
		items, meta := r.records(t)
		if len(items) != 5 {
			t.Fatalf("limit: got %d items", len(items))
		}
		if meta["@pagination"]["has_more"] != true ||
			meta["@pagination"]["total_items"].(float64) != 150 {
			t.Errorf("pagination: %v", meta["@pagination"])
		}
	})

	t.Run("count distinct sample", func(t *testing.T) {
		r := runIn(t, h, "query", "count", testDB, "orders", "--filter", `{"status":"pending"}`)
		items, _ := r.records(t)
		if items[0]["count"].(float64) != 50 {
			t.Errorf("count: %v", items[0])
		}

		r = runIn(t, h, "query", "distinct", testDB, "orders", "status")
		items, _ = r.records(t)
		values := items[0]["values"].([]any)
		if len(values) != 2 {
			t.Errorf("distinct: %v", values)
		}

		r = runIn(t, h, "query", "sample", testDB, "orders", "--size", "3")
		items, meta := r.records(t)
		if len(items) != 3 || meta["@meta"]["sampleSize"].(float64) != 3 {
			t.Errorf("sample: %d items, meta %v", len(items), meta["@meta"])
		}
	})

	t.Run("aggregate", func(t *testing.T) {
		pipeline := `[{"$group":{"_id":"$status","n":{"$sum":1}}},{"$sort":{"_id":1}}]`
		r := runIn(t, h, "query", "aggregate", testDB, "orders", pipeline)
		items, _ := r.records(t)
		if len(items) != 2 || items[0]["_id"] != "done" || items[0]["n"].(float64) != 100 {
			t.Errorf("aggregate: %v", items)
		}

		r = runIn(t, h, "query", "aggregate", testDB, "orders", `[{"$out":"evil"}]`)
		if r.exitCode != 1 || !strings.Contains(r.stderrJSON(t)["error"].(string), `Write stage "$out" is not allowed`) {
			t.Errorf("$out: exit=%d stderr=%s", r.exitCode, r.stderr)
		}

		r = runIn(t, h, "query", "aggregate", testDB, "orders", "--pipeline", `[{"$count":"total"}]`)
		items, _ = r.records(t)
		if items[0]["total"].(float64) != 150 {
			t.Errorf("--pipeline flag: %v", items)
		}
	})

	t.Run("ejson filter", func(t *testing.T) {
		r := runIn(t, h, "query", "count", testDB, "users",
			"--filter", `{"_id":{"$oid":"665a1b2c3d4e5f6a7b8c9d0e"}}`)
		items, _ := r.records(t)
		if items[0]["count"].(float64) != 1 {
			t.Errorf("$oid filter: %v", items[0])
		}
	})
}

func TestFormatJSONEnvelope(t *testing.T) {
	r := runIn(t, home(t), "query", "find", testDB, "orders", "--limit", "2", "-f", "json")
	var envelope map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &envelope); err != nil {
		t.Fatalf("not a JSON envelope: %v", err)
	}
	if data, ok := envelope["data"].([]any); !ok || len(data) != 2 {
		t.Fatalf("envelope: %v", envelope)
	}
}

func TestUnknownConnection(t *testing.T) {
	r := runIn(t, home(t), "-c", "ghost", "database", "list")
	payload := r.stderrJSON(t)
	if r.exitCode != 1 ||
		!strings.Contains(payload["error"].(string), `Connection "ghost" not found`) ||
		payload["fixable_by"] != "agent" {
		t.Errorf("exit=%d stderr=%s", r.exitCode, r.stderr)
	}
}

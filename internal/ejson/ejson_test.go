package ejson

import (
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// get returns the value for key in an order-preserving document.
func get(t *testing.T, d bson.D, key string) any {
	t.Helper()
	for _, e := range d {
		if e.Key == key {
			return e.Value
		}
	}
	t.Fatalf("key %q not found in %v", key, d)
	return nil
}

func TestParsePlainObject(t *testing.T) {
	d, err := Parse(`{"name": "test", "value": 42}`, "filter")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if got := get(t, d, "name"); got != "test" {
		t.Errorf("name = %v, want test", got)
	}
	// Relaxed ext-JSON decodes integers to int32.
	if got := get(t, d, "value"); got != int32(42) {
		t.Errorf("value = %v (%T), want int32 42", got, got)
	}
}

func TestParseConvertsDate(t *testing.T) {
	d, err := Parse(`{"createdAt": {"$gte": {"$date": "2026-01-01T00:00:00Z"}}}`, "filter")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	createdAt := get(t, d, "createdAt").(bson.D)
	gte, ok := get(t, createdAt, "$gte").(bson.DateTime)
	if !ok {
		t.Fatalf("$gte = %T, want bson.DateTime", get(t, createdAt, "$gte"))
	}
	want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !gte.Time().UTC().Equal(want) {
		t.Errorf("$gte = %v, want %v", gte.Time().UTC(), want)
	}
}

func TestParseConvertsOID(t *testing.T) {
	d, err := Parse(`{"_id": {"$oid": "507f1f77bcf86cd799439011"}}`, "filter")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	oid, ok := get(t, d, "_id").(bson.ObjectID)
	if !ok {
		t.Fatalf("_id = %T, want bson.ObjectID", get(t, d, "_id"))
	}
	if oid.Hex() != "507f1f77bcf86cd799439011" {
		t.Errorf("_id = %s, want 507f1f77bcf86cd799439011", oid.Hex())
	}
}

func TestParseNestedDateOperators(t *testing.T) {
	input := `{"updatedAt": {"$gte": {"$date": "2026-04-01T14:41:00Z"}, "$lte": {"$date": "2026-04-02T15:57:00Z"}}}`
	d, err := Parse(input, "filter")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	updatedAt := get(t, d, "updatedAt").(bson.D)
	gte, ok := get(t, updatedAt, "$gte").(bson.DateTime)
	if !ok {
		t.Fatalf("$gte = %T, want bson.DateTime", get(t, updatedAt, "$gte"))
	}
	lte, ok := get(t, updatedAt, "$lte").(bson.DateTime)
	if !ok {
		t.Fatalf("$lte = %T, want bson.DateTime", get(t, updatedAt, "$lte"))
	}
	if got := gte.Time().UTC().Format(time.RFC3339); got != "2026-04-01T14:41:00Z" {
		t.Errorf("$gte = %s, want 2026-04-01T14:41:00Z", got)
	}
	if got := lte.Time().UTC().Format(time.RFC3339); got != "2026-04-02T15:57:00Z" {
		t.Errorf("$lte = %s, want 2026-04-02T15:57:00Z", got)
	}
}

func TestParsePassesThroughStringsAndNumbers(t *testing.T) {
	d, err := Parse(`{"status": "active", "count": 5}`, "filter")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if got := get(t, d, "status"); got != "active" {
		t.Errorf("status = %v, want active", got)
	}
	if got := get(t, d, "count"); got != int32(5) {
		t.Errorf("count = %v, want int32 5", got)
	}
}

func TestParseThrowsInvalidJSONWithFlagName(t *testing.T) {
	_, err := Parse("not-json", "filter")
	if err == nil || !strings.Contains(err.Error(), "Invalid JSON for --filter") {
		t.Errorf("error = %v, want 'Invalid JSON for --filter'", err)
	}
}

func TestParseThrowsInvalidJSONWithCustomFlagName(t *testing.T) {
	_, err := Parse("{bad", "sort")
	if err == nil || !strings.Contains(err.Error(), "Invalid JSON for --sort") {
		t.Errorf("error = %v, want 'Invalid JSON for --sort'", err)
	}
}

func TestParseArrayParsesArray(t *testing.T) {
	arr, err := ParseArray(`[{"$match": {"status": "active"}}]`, "pipeline")
	if err != nil {
		t.Fatalf("ParseArray() error: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("len = %d, want 1", len(arr))
	}
	stage := arr[0].(bson.D)
	match := get(t, stage, "$match").(bson.D)
	if got := get(t, match, "status"); got != "active" {
		t.Errorf("status = %v, want active", got)
	}
}

func TestParseArrayConvertsEJSONInElements(t *testing.T) {
	input := `[{"$match": {"createdAt": {"$gte": {"$date": "2026-01-01T00:00:00Z"}}}}]`
	arr, err := ParseArray(input, "pipeline")
	if err != nil {
		t.Fatalf("ParseArray() error: %v", err)
	}
	match := get(t, arr[0].(bson.D), "$match").(bson.D)
	createdAt := get(t, match, "createdAt").(bson.D)
	if _, ok := get(t, createdAt, "$gte").(bson.DateTime); !ok {
		t.Errorf("$gte = %T, want bson.DateTime", get(t, createdAt, "$gte"))
	}
}

func TestParseArrayThrowsWhenNotArray(t *testing.T) {
	_, err := ParseArray(`{"not": "array"}`, "pipeline")
	if err == nil || !strings.Contains(err.Error(), "--pipeline must be a JSON array") {
		t.Errorf("error = %v, want '--pipeline must be a JSON array'", err)
	}
}

func TestParseArrayThrowsInvalidJSON(t *testing.T) {
	_, err := ParseArray("not-json", "pipeline")
	if err == nil || !strings.Contains(err.Error(), "Invalid JSON for --pipeline") {
		t.Errorf("error = %v, want 'Invalid JSON for --pipeline'", err)
	}
}

package mongo

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func mustOID(t *testing.T, hexStr string) bson.ObjectID {
	t.Helper()
	oid, err := bson.ObjectIDFromHex(hexStr)
	if err != nil {
		t.Fatalf("ObjectIDFromHex(%q): %v", hexStr, err)
	}
	return oid
}

// fieldMap indexes inferred fields by path for assertion.
func fieldMap(fields []FieldInfo) map[string]FieldInfo {
	m := make(map[string]FieldInfo, len(fields))
	for _, f := range fields {
		m[f.Path] = f
	}
	return m
}

func assertTypes(t *testing.T, m map[string]FieldInfo, path string, want []string) {
	t.Helper()
	f, ok := m[path]
	if !ok {
		t.Fatalf("field %q not found", path)
	}
	if !reflect.DeepEqual(f.Types, want) {
		t.Errorf("%s.types = %v, want %v", path, f.Types, want)
	}
}

func TestInferFieldsDetectsBasicTypes(t *testing.T) {
	docs := []bson.D{
		{{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")}, {Key: "name", Value: "Alice"}, {Key: "age", Value: int32(30)}, {Key: "active", Value: true}},
		{{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439012")}, {Key: "name", Value: "Bob"}, {Key: "age", Value: int32(25)}, {Key: "active", Value: false}},
	}
	m := fieldMap(InferFields(docs, 0))
	assertTypes(t, m, "_id", []string{"ObjectId"})
	assertTypes(t, m, "name", []string{"string"})
	assertTypes(t, m, "age", []string{"int"})
	assertTypes(t, m, "active", []string{"boolean"})
}

// TestInferFieldsDetectsBSONTypes covers the BSON type names. NOTE on the
// int64→type-name mapping (a deliberate divergence from the TS test): the TS
// driver keeps a Long wrapper, so schema.ts reports any Long as "long"
// regardless of magnitude. The Go driver decodes BSON longs to a bare int64
// with no wrapper, so mongo.typeName reports safe-magnitude int64 as "int" and
// only unsafe magnitudes (|v| > 2^53-1) as "long" — matching serialize's
// promoteLongs behavior. We assert the Go behavior for both magnitudes.
func TestInferFieldsDetectsBSONTypes(t *testing.T) {
	dec, err := bson.ParseDecimal128("1.23")
	if err != nil {
		t.Fatalf("ParseDecimal128: %v", err)
	}
	docs := []bson.D{{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "created", Value: bson.NewDateTimeFromTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))},
		{Key: "data", Value: bson.Binary{Subtype: 0x00, Data: []byte("hello")}},
		{Key: "bigNumSafe", Value: int64(999)},
		{Key: "bigNumUnsafe", Value: int64(9007199254740993)},
		{Key: "decimal", Value: dec},
	}}
	m := fieldMap(InferFields(docs, 0))
	assertTypes(t, m, "created", []string{"date"})
	assertTypes(t, m, "data", []string{"binary"})
	assertTypes(t, m, "bigNumSafe", []string{"int"}) // TS reported "long"; see note above
	assertTypes(t, m, "bigNumUnsafe", []string{"long"})
	assertTypes(t, m, "decimal", []string{"decimal"})
}

func TestInferFieldsWalksNestedObjects(t *testing.T) {
	docs := []bson.D{{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "address", Value: bson.D{{Key: "city", Value: "NYC"}, {Key: "zip", Value: int32(10001)}}},
	}}
	m := fieldMap(InferFields(docs, 0))
	assertTypes(t, m, "address", []string{"object"})
	assertTypes(t, m, "address.city", []string{"string"})
	assertTypes(t, m, "address.zip", []string{"int"})
}

func TestInferFieldsWalksArrayElements(t *testing.T) {
	docs := []bson.D{{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "tags", Value: bson.A{"a", "b"}},
		{Key: "items", Value: bson.A{bson.D{{Key: "name", Value: "widget"}, {Key: "qty", Value: int32(5)}}}},
	}}
	m := fieldMap(InferFields(docs, 0))
	assertTypes(t, m, "tags", []string{"array"})
	assertTypes(t, m, "tags.$", []string{"string"})
	assertTypes(t, m, "items", []string{"array"})
	assertTypes(t, m, "items.$", []string{"object"})
	assertTypes(t, m, "items.$.name", []string{"string"})
	assertTypes(t, m, "items.$.qty", []string{"int"})
}

func TestInferFieldsDetectsMixedTypes(t *testing.T) {
	docs := []bson.D{
		{{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")}, {Key: "value", Value: "text"}},
		{{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439012")}, {Key: "value", Value: int32(42)}},
		{{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439013")}, {Key: "value", Value: nil}},
	}
	m := fieldMap(InferFields(docs, 0))
	// Types are sorted; with string/int/null present order is int,null,string.
	assertTypes(t, m, "value", []string{"int", "null", "string"})
}

func TestInferFieldsCalculatesPresenceRatio(t *testing.T) {
	docs := []bson.D{
		{{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")}, {Key: "name", Value: "A"}, {Key: "optional", Value: "yes"}},
		{{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439012")}, {Key: "name", Value: "B"}},
	}
	m := fieldMap(InferFields(docs, 0))
	if m["_id"].Presence != 1 {
		t.Errorf("_id presence = %v, want 1", m["_id"].Presence)
	}
	if m["name"].Presence != 1 {
		t.Errorf("name presence = %v, want 1", m["name"].Presence)
	}
	if m["optional"].Presence != 0.5 {
		t.Errorf("optional presence = %v, want 0.5", m["optional"].Presence)
	}
}

func TestInferFieldsReturnsSortedByPath(t *testing.T) {
	docs := []bson.D{{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "zebra", Value: int32(1)},
		{Key: "alpha", Value: int32(2)},
		{Key: "middle", Value: bson.D{{Key: "nested", Value: "x"}}},
	}}
	fields := InferFields(docs, 0)
	for i := 1; i < len(fields); i++ {
		if fields[i-1].Path > fields[i].Path {
			t.Errorf("fields not sorted: %q before %q", fields[i-1].Path, fields[i].Path)
		}
	}
}

func TestInferFieldsHandlesEmptyDocs(t *testing.T) {
	if got := InferFields([]bson.D{}, 0); len(got) != 0 {
		t.Errorf("InferFields([]) = %v, want empty", got)
	}
}

func TestInferFieldsDetectsDoubleVsInt(t *testing.T) {
	docs := []bson.D{{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "integer", Value: int32(42)},
		{Key: "decimal", Value: 3.14},
	}}
	m := fieldMap(InferFields(docs, 0))
	assertTypes(t, m, "integer", []string{"int"})
	assertTypes(t, m, "decimal", []string{"double"})
}

func TestInferFieldsDepth1LimitsToTopLevel(t *testing.T) {
	docs := []bson.D{{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "name", Value: "Alice"},
		{Key: "address", Value: bson.D{
			{Key: "city", Value: "NYC"}, {Key: "zip", Value: int32(10001)},
			{Key: "geo", Value: bson.D{{Key: "lat", Value: 40.7}, {Key: "lng", Value: -74.0}}},
		}},
	}}
	m := fieldMap(InferFields(docs, 1))
	for _, p := range []string{"_id", "name", "address"} {
		if _, ok := m[p]; !ok {
			t.Errorf("expected path %q present at depth 1", p)
		}
	}
	for _, p := range []string{"address.city", "address.zip", "address.geo"} {
		if _, ok := m[p]; ok {
			t.Errorf("path %q present, want excluded at depth 1", p)
		}
	}
}

func TestInferFieldsDepth2IncludesOneLevelOfNesting(t *testing.T) {
	docs := []bson.D{{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "address", Value: bson.D{
			{Key: "city", Value: "NYC"},
			{Key: "geo", Value: bson.D{{Key: "lat", Value: 40.7}, {Key: "lng", Value: -74.0}}},
		}},
	}}
	m := fieldMap(InferFields(docs, 2))
	for _, p := range []string{"address", "address.city", "address.geo"} {
		if _, ok := m[p]; !ok {
			t.Errorf("expected path %q present at depth 2", p)
		}
	}
	for _, p := range []string{"address.geo.lat", "address.geo.lng"} {
		if _, ok := m[p]; ok {
			t.Errorf("path %q present, want excluded at depth 2", p)
		}
	}
}

func TestInferFieldsDepthLimitsArrayElementNesting(t *testing.T) {
	docs := []bson.D{{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "items", Value: bson.A{bson.D{
			{Key: "name", Value: "widget"},
			{Key: "meta", Value: bson.D{{Key: "color", Value: "red"}}},
		}}},
	}}
	m := fieldMap(InferFields(docs, 2))
	for _, p := range []string{"items", "items.$", "items.$.name", "items.$.meta"} {
		if _, ok := m[p]; !ok {
			t.Errorf("expected path %q present at depth 2", p)
		}
	}
	if _, ok := m["items.$.meta.color"]; ok {
		t.Error("path items.$.meta.color present, want excluded at depth 2")
	}
}

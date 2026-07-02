package serialize

import (
	"encoding/base64"
	"encoding/hex"
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

func TestValueObjectIDToHex(t *testing.T) {
	if got := Value(mustOID(t, "507f1f77bcf86cd799439011")); got != "507f1f77bcf86cd799439011" {
		t.Errorf("Value(ObjectID) = %v, want hex string", got)
	}
}

func TestValueDateToISO(t *testing.T) {
	tm := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	want := "2024-01-15T10:30:00.000Z"
	if got := Value(bson.NewDateTimeFromTime(tm)); got != want {
		t.Errorf("Value(DateTime) = %v, want %q", got, want)
	}
	if got := Value(tm); got != want {
		t.Errorf("Value(time.Time) = %v, want %q", got, want)
	}
}

func TestValueBinaryToBase64(t *testing.T) {
	bin := bson.Binary{Subtype: 0x00, Data: []byte("hello world")}
	want := base64.StdEncoding.EncodeToString([]byte("hello world"))
	if got := Value(bin); got != want {
		t.Errorf("Value(Binary) = %v, want %q", got, want)
	}
}

func TestValueSafeInt64ToNumber(t *testing.T) {
	if got := Value(int64(42)); got != int64(42) {
		t.Errorf("Value(int64 42) = %v (%T), want int64 42", got, got)
	}
}

func TestValueUnsafeInt64ToString(t *testing.T) {
	// 9007199254740993 > Number.MAX_SAFE_INTEGER (2^53-1), so it can't round-trip
	// through a JS double — serialize emits it as a string.
	if got := Value(int64(9007199254740993)); got != "9007199254740993" {
		t.Errorf("Value(unsafe int64) = %v, want string", got)
	}
}

func TestValueDecimal128ToString(t *testing.T) {
	dec, err := bson.ParseDecimal128("3.14159")
	if err != nil {
		t.Fatalf("ParseDecimal128: %v", err)
	}
	if got := Value(dec); got != "3.14159" {
		t.Errorf("Value(Decimal128) = %v, want '3.14159'", got)
	}
}

// TestValueUUIDBinaryToUUIDString documents a DELIBERATE divergence from the TS
// implementation. In the TS mongodb driver, UUID extends Binary, so the Binary
// branch matches first and a UUID serializes to base64. The Go driver has no
// UUID wrapper type — a UUID is a Binary with subtype 0x04 — and serialize.Value
// intentionally renders that subtype as the canonical hyphenated UUID string
// (see CLAUDE.md: "UUID → string"). We assert the Go behavior.
func TestValueUUIDBinaryToUUIDString(t *testing.T) {
	raw, err := hex.DecodeString("550e8400e29b41d4a716446655440000")
	if err != nil {
		t.Fatalf("hex decode: %v", err)
	}
	bin := bson.Binary{Subtype: bson.TypeBinaryUUID, Data: raw}
	if got := Value(bin); got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("Value(UUID Binary) = %v, want hyphenated uuid string", got)
	}
}

func TestValueRegexToString(t *testing.T) {
	if got := Value(bson.Regex{Pattern: "test.*pattern", Options: "i"}); got != "/test.*pattern/i" {
		t.Errorf("Value(Regex) = %v, want '/test.*pattern/i'", got)
	}
}

func TestValueNilPassesThrough(t *testing.T) {
	if got := Value(nil); got != nil {
		t.Errorf("Value(nil) = %v, want nil", got)
	}
	if got := Value(bson.Null{}); got != nil {
		t.Errorf("Value(bson.Null) = %v, want nil", got)
	}
}

func TestValuePrimitivesPassThrough(t *testing.T) {
	cases := []struct {
		in   any
		want any
	}{
		{"hello", "hello"},
		{42, 42},
		{true, true},
		{false, false},
		{0, 0},
	}
	for _, tc := range cases {
		if got := Value(tc.in); got != tc.want {
			t.Errorf("Value(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestValueNestedDocumentsRecursive(t *testing.T) {
	doc := bson.D{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "name", Value: "Test"},
		{Key: "metadata", Value: bson.D{
			{Key: "createdAt", Value: bson.NewDateTimeFromTime(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC))},
			{Key: "tags", Value: bson.A{"a", "b"}},
		}},
	}
	want := map[string]any{
		"_id":  "507f1f77bcf86cd799439011",
		"name": "Test",
		"metadata": map[string]any{
			"createdAt": "2024-01-15T10:30:00.000Z",
			"tags":      []any{"a", "b"},
		},
	}
	if got := Value(doc); !reflect.DeepEqual(got, want) {
		t.Errorf("Value(nested doc) mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestValueArraysWithBSONTypes(t *testing.T) {
	arr := bson.A{
		mustOID(t, "507f1f77bcf86cd799439011"),
		bson.NewDateTimeFromTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		"plain",
		42,
	}
	want := []any{"507f1f77bcf86cd799439011", "2024-01-01T00:00:00.000Z", "plain", 42}
	if got := Value(arr); !reflect.DeepEqual(got, want) {
		t.Errorf("Value(array) mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestDocumentSerializesSingleDocument(t *testing.T) {
	doc := bson.D{
		{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")},
		{Key: "name", Value: "Test"},
	}
	want := map[string]any{"_id": "507f1f77bcf86cd799439011", "name": "Test"}
	if got := Document(doc); !reflect.DeepEqual(got, want) {
		t.Errorf("Document() mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestDocumentsSerializesArrayOfDocuments(t *testing.T) {
	docs := []bson.D{
		{{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439011")}, {Key: "name", Value: "A"}},
		{{Key: "_id", Value: mustOID(t, "507f1f77bcf86cd799439012")}, {Key: "name", Value: "B"}},
	}
	want := []map[string]any{
		{"_id": "507f1f77bcf86cd799439011", "name": "A"},
		{"_id": "507f1f77bcf86cd799439012", "name": "B"},
	}
	if got := Documents(docs); !reflect.DeepEqual(got, want) {
		t.Errorf("Documents() mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestDocumentsHandlesEmptyArray(t *testing.T) {
	if got := Documents([]bson.D{}); len(got) != 0 {
		t.Errorf("Documents([]) = %v, want empty", got)
	}
}

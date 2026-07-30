package serialize

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	yaml "gopkg.in/yaml.v3"
)

func TestOrderedDocumentPreservesOrderAndNulls(t *testing.T) {
	doc := bson.D{
		{Key: "zebra", Value: int32(1)},
		{Key: "apple", Value: nil},
		{Key: "nested", Value: bson.D{{Key: "b", Value: int32(-1)}, {Key: "a", Value: int32(1)}}},
	}

	got, err := json.Marshal(OrderedDocument(doc))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"zebra":1,"apple":null,"nested":{"b":-1,"a":1}}`
	if string(got) != want {
		t.Errorf("JSON\n got: %s\nwant: %s", got, want)
	}
}

func TestOrderedMarshalsYAMLInOrder(t *testing.T) {
	got, err := yaml.Marshal(Ordered{
		{Key: "name", Value: "status_1_expiryDate_1"},
		{Key: "key", Value: Ordered{{Key: "status", Value: 1}, {Key: "expiryDate", Value: 1}}},
		{Key: "deletedAt", Value: nil},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := "name: status_1_expiryDate_1\nkey:\n    status: 1\n    expiryDate: 1\ndeletedAt: null\n"
	if string(got) != want {
		t.Errorf("YAML\n got: %q\nwant: %q", got, want)
	}
}

// MarshalJSON must not pre-escape, so the caller's escaping policy is the one
// that applies — the output contract disables HTML escaping so `<`, `>` and `&`
// survive in values. Encoded the way lib-agent-output encodes.
func TestOrderedLeavesHTMLEscapingToTheEncoder(t *testing.T) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(Ordered{{Key: "expr", Value: "a<b && c>d"}}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	want := `{"expr":"a<b && c>d"}`
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

// An index document missing both name and key yields an empty spec; it must
// degrade to an empty object, not null.
func TestOrderedMarshalsEmptyDocument(t *testing.T) {
	got, err := json.Marshal(Ordered{})
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	if string(got) != "{}" {
		t.Errorf("JSON: got %s want {}", got)
	}

	got, err = yaml.Marshal(Ordered{})
	if err != nil {
		t.Fatalf("marshal YAML: %v", err)
	}
	if string(got) != "{}\n" {
		t.Errorf("YAML: got %q want %q", got, "{}\n")
	}
}

// Whole-valued floats render as integers, matching the family YAML encoder,
// which cannot reach inside an Ordered to normalize them itself.
func TestOrderedYAMLRendersWholeFloatsAsIntegers(t *testing.T) {
	got, err := yaml.Marshal(Ordered{
		{Key: "max", Value: float64(1500000)},
		{Key: "ratio", Value: 1.5},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := "max: 1500000\nratio: 1.5\n"
	if string(got) != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestOrderedLookup(t *testing.T) {
	o := Ordered{{Key: "a", Value: 1}, {Key: "b", Value: nil}}
	if v, ok := o.Lookup("a"); !ok || v != 1 {
		t.Errorf(`Lookup("a") = %v, %v`, v, ok)
	}
	if v, ok := o.Lookup("b"); !ok || v != nil {
		t.Errorf(`Lookup("b") = %v, %v — a null field is present`, v, ok)
	}
	if _, ok := o.Lookup("missing"); ok {
		t.Error(`Lookup("missing") reported present`)
	}
}

// Document output keeps its sorted-map behavior: OrderedDocument is opt-in.
func TestValueStillReturnsMaps(t *testing.T) {
	if _, ok := Value(bson.D{{Key: "a", Value: 1}}).(map[string]any); !ok {
		t.Error("Value() should still produce map[string]any")
	}
}

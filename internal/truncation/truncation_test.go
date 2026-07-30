package truncation

import (
	"slices"
	"strings"
	"testing"

	out "github.com/shhac/lib-agent-output"
)

const ell = "…"

func TestApplyTruncatesOverDefaultAndAddsCompanionLength(t *testing.T) {
	Configure(Options{})
	long := strings.Repeat("a", 300)
	result := Apply(map[string]any{"description": long}).(map[string]any)
	if got := result["description"]; got != strings.Repeat("a", 200)+ell {
		t.Errorf("description = %q, want truncated", got)
	}
	if got := result["descriptionLength"]; got != 300 {
		t.Errorf("descriptionLength = %v, want 300", got)
	}
}

func TestApplyPreservesShortStrings(t *testing.T) {
	Configure(Options{})
	result := Apply(map[string]any{"title": "short"}).(map[string]any)
	if got := result["title"]; got != "short" {
		t.Errorf("title = %q, want short", got)
	}
	if _, ok := result["titleLength"]; ok {
		t.Error("titleLength present, want absent")
	}
}

func TestApplyPreservesStringAtExactlyMaxLength(t *testing.T) {
	Configure(Options{})
	exact := strings.Repeat("x", 200)
	result := Apply(map[string]any{"body": exact}).(map[string]any)
	if got := result["body"]; got != exact {
		t.Errorf("body truncated, want preserved")
	}
	if _, ok := result["bodyLength"]; ok {
		t.Error("bodyLength present, want absent")
	}
}

func TestApplyTruncatesAt201Chars(t *testing.T) {
	Configure(Options{})
	str := strings.Repeat("y", 201)
	result := Apply(map[string]any{"data": str}).(map[string]any)
	if got := result["data"]; got != strings.Repeat("y", 200)+ell {
		t.Errorf("data = %q, want truncated", got)
	}
	if got := result["dataLength"]; got != 201 {
		t.Errorf("dataLength = %v, want 201", got)
	}
}

func TestApplyHandlesNestedObjects(t *testing.T) {
	Configure(Options{})
	data := map[string]any{"outer": map[string]any{"inner": strings.Repeat("z", 300), "keep": "ok"}}
	result := Apply(data).(map[string]any)
	outer := result["outer"].(map[string]any)
	if got := outer["inner"]; got != strings.Repeat("z", 200)+ell {
		t.Errorf("inner = %q, want truncated", got)
	}
	if got := outer["innerLength"]; got != 300 {
		t.Errorf("innerLength = %v, want 300", got)
	}
	if got := outer["keep"]; got != "ok" {
		t.Errorf("keep = %q, want ok", got)
	}
}

func TestApplyHandlesArraysOfObjects(t *testing.T) {
	Configure(Options{})
	data := []any{
		map[string]any{"id": "1", "text": strings.Repeat("a", 300)},
		map[string]any{"id": "2", "text": "short"},
	}
	result := Apply(data).([]any)
	first := result[0].(map[string]any)
	if got := first["text"]; got != strings.Repeat("a", 200)+ell {
		t.Errorf("result[0].text = %q, want truncated", got)
	}
	if got := first["textLength"]; got != 300 {
		t.Errorf("result[0].textLength = %v, want 300", got)
	}
	second := result[1].(map[string]any)
	if got := second["text"]; got != "short" {
		t.Errorf("result[1].text = %q, want short", got)
	}
	if _, ok := second["textLength"]; ok {
		t.Error("result[1].textLength present, want absent")
	}
}

func TestApplyHandlesNilValue(t *testing.T) {
	Configure(Options{})
	if got := Apply(nil); got != nil {
		t.Errorf("Apply(nil) = %v, want nil", got)
	}
}

func TestApplyHandlesNilFieldsInObjects(t *testing.T) {
	Configure(Options{})
	result := Apply(map[string]any{"a": nil, "b": "ok"}).(map[string]any)
	if result["a"] != nil {
		t.Errorf("a = %v, want nil", result["a"])
	}
	if result["b"] != "ok" {
		t.Errorf("b = %v, want ok", result["b"])
	}
}

func TestApplyPassesThroughNonObjectPrimitives(t *testing.T) {
	Configure(Options{})
	if got := Apply("hello"); got != "hello" {
		t.Errorf("Apply(hello) = %v, want hello", got)
	}
	if got := Apply(42); got != 42 {
		t.Errorf("Apply(42) = %v, want 42", got)
	}
	if got := Apply(true); got != true {
		t.Errorf("Apply(true) = %v, want true", got)
	}
}

func TestApplyPreservesNonStringFieldValues(t *testing.T) {
	Configure(Options{})
	result := Apply(map[string]any{"count": 42, "active": true}).(map[string]any)
	if result["count"] != 42 {
		t.Errorf("count = %v, want 42", result["count"])
	}
	if result["active"] != true {
		t.Errorf("active = %v, want true", result["active"])
	}
	if _, ok := result["countLength"]; ok {
		t.Error("countLength present, want absent")
	}
}

func TestApplyTruncatesAnyStringFieldName(t *testing.T) {
	Configure(Options{})
	long := strings.Repeat("q", 250)
	result := Apply(map[string]any{"customField": long}).(map[string]any)
	if got := result["customField"]; got != strings.Repeat("q", 200)+ell {
		t.Errorf("customField = %q, want truncated", got)
	}
	if got := result["customFieldLength"]; got != 250 {
		t.Errorf("customFieldLength = %v, want 250", got)
	}
}

func TestConfigureFullExpandsAllFields(t *testing.T) {
	Configure(Options{Full: true})
	long := strings.Repeat("a", 300)
	result := Apply(map[string]any{"description": long, "body": long}).(map[string]any)
	// --full leaves the value intact but still records the companion length.
	if result["description"] != long {
		t.Error("description truncated under --full, want intact")
	}
	if result["descriptionLength"] != 300 {
		t.Errorf("descriptionLength = %v, want 300", result["descriptionLength"])
	}
	if result["body"] != long {
		t.Error("body truncated under --full, want intact")
	}
	if result["bodyLength"] != 300 {
		t.Errorf("bodyLength = %v, want 300", result["bodyLength"])
	}
}

func TestConfigureExpandOnlySpecifiedFields(t *testing.T) {
	Configure(Options{Expand: "description"})
	long := strings.Repeat("a", 300)
	result := Apply(map[string]any{"description": long, "body": long}).(map[string]any)
	if result["description"] != long {
		t.Error("description truncated, want expanded")
	}
	if result["body"] != strings.Repeat("a", 200)+ell {
		t.Error("body not truncated, want truncated")
	}
}

func TestConfigureExpandMultipleCommaSeparated(t *testing.T) {
	Configure(Options{Expand: "description,body"})
	long := strings.Repeat("a", 300)
	result := Apply(map[string]any{"description": long, "body": long, "content": long}).(map[string]any)
	if result["description"] != long {
		t.Error("description truncated, want expanded")
	}
	if result["body"] != long {
		t.Error("body truncated, want expanded")
	}
	if result["content"] != strings.Repeat("a", 200)+ell {
		t.Error("content not truncated, want truncated")
	}
}

func TestConfigureExpandHandlesWhitespace(t *testing.T) {
	Configure(Options{Expand: " description , body "})
	long := strings.Repeat("a", 300)
	result := Apply(map[string]any{"description": long, "body": long}).(map[string]any)
	if result["description"] != long || result["body"] != long {
		t.Error("whitespace in expand list not trimmed, want both expanded")
	}
}

// TestConfigureExpandIsCaseInsensitive pins the TS quirk: the expand list is
// lowercased, and field names are matched against the lowercased entries. So
// expand "Description" matches a field literally named "description".
func TestConfigureExpandIsCaseInsensitive(t *testing.T) {
	Configure(Options{Expand: "Description"})
	long := strings.Repeat("a", 300)
	result := Apply(map[string]any{"description": long}).(map[string]any)
	if result["description"] != long {
		t.Error("description truncated, want expanded via case-insensitive match")
	}
}

func TestConfigureFullTakesPrecedenceOverExpand(t *testing.T) {
	Configure(Options{Full: true, Expand: "description"})
	long := strings.Repeat("a", 300)
	result := Apply(map[string]any{"body": long}).(map[string]any)
	if result["body"] != long {
		t.Error("body truncated, want expanded (--full overrides --expand)")
	}
}

func TestConfigureRespectsCustomMaxLength(t *testing.T) {
	Configure(Options{MaxLength: 50})
	long := strings.Repeat("m", 100)
	result := Apply(map[string]any{"field": long}).(map[string]any)
	if got := result["field"]; got != strings.Repeat("m", 50)+ell {
		t.Errorf("field = %q, want truncated at 50", got)
	}
	if got := result["fieldLength"]; got != 100 {
		t.Errorf("fieldLength = %v, want 100", got)
	}
}

func TestConfigureResetsToDefault(t *testing.T) {
	Configure(Options{Full: true})
	Configure(Options{})
	long := strings.Repeat("a", 300)
	result := Apply(map[string]any{"description": long}).(map[string]any)
	if result["description"] != strings.Repeat("a", 200)+ell {
		t.Error("description not truncated after reset, want default truncation")
	}
}

// Apply runs chained after out.PruneEmpty over the same tree, so the two must
// agree on what a document is. Before Ordered was added here, an ordered
// document hit the default branch and was returned unshaped — silently, which
// is the same fail-open shape the library's own walkers warn about.
func TestApplyWalksOrderedDocuments(t *testing.T) {
	Configure(Options{MaxLength: 10})
	t.Cleanup(func() { Configure(Options{}) })

	long := strings.Repeat("x", 25)
	got, ok := Apply(out.Ordered{
		{Key: "zebra", Value: long},
		{Key: "apple", Value: "short"},
	}).(out.Ordered)
	if !ok {
		t.Fatalf("Apply returned %T, want out.Ordered", Apply(out.Ordered{}))
	}

	// Field order survives, and the companion key follows the field it describes.
	var keys []string
	for _, field := range got {
		keys = append(keys, field.Key)
	}
	want := []string{"zebra", "zebraLength", "apple"}
	if !slices.Equal(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}
	if value, _ := got.Lookup("zebra"); value != strings.Repeat("x", 10)+"…" {
		t.Errorf("value not truncated: %v", value)
	}
	if length, _ := got.Lookup("zebraLength"); length != 25 {
		t.Errorf("length companion = %v, want 25", length)
	}
}

func TestApplyWalksOrderedNestedInAMap(t *testing.T) {
	Configure(Options{MaxLength: 10})
	t.Cleanup(func() { Configure(Options{}) })

	got, _ := Apply(map[string]any{
		"spec": out.Ordered{{Key: "note", Value: strings.Repeat("y", 25)}},
	}).(map[string]any)

	nested, ok := got["spec"].(out.Ordered)
	if !ok {
		t.Fatalf("nested Ordered became %T", got["spec"])
	}
	if _, present := nested.Lookup("noteLength"); !present {
		t.Errorf("nested ordered document escaped truncation: %v", nested)
	}
}

package serialize

import (
	"bytes"
	"encoding/json"
	"math"

	yaml "gopkg.in/yaml.v3"
)

// Field is one key/value pair of an Ordered document.
type Field struct {
	Key   string
	Value any
}

// Ordered is a document that keeps its server-reported field order through JSON
// and YAML encoding, where a Go map would not (encoding/json sorts map keys
// alphabetically). Use it wherever field order is part of the meaning rather
// than presentation — an index key spec above all, since {a:1,b:1} and
// {b:1,a:1} are different indexes serving different queries.
type Ordered []Field

// VerbatimRecord marks Ordered as an output.Verbatim record, so the normalizing
// printers refuse it instead of re-sorting the field order it exists to keep.
// Declared structurally: serialize takes no dependency on output.
func (Ordered) VerbatimRecord() {}

// Lookup returns the value stored under key and whether the field is present.
func (o Ordered) Lookup(key string) (any, bool) {
	for _, field := range o {
		if field.Key == key {
			return field.Value, true
		}
	}
	return nil, false
}

func (o Ordered) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, field := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := encodeJSON(field.Key)
		if err != nil {
			return nil, err
		}
		value, err := encodeJSON(field.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// encodeJSON matches the family wire contract's HTML escaping (off), which
// json.Marshal would otherwise apply to `<`, `>` and `&` inside values.
func encodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// MarshalYAML emits an explicit mapping node so `-f yaml` preserves the same
// order as the JSON formats; yaml.v3 has no ordered map type to return instead.
func (o Ordered) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, field := range o {
		key := &yaml.Node{}
		if err := key.Encode(field.Key); err != nil {
			return nil, err
		}
		value := &yaml.Node{}
		if err := value.Encode(wholeFloatToInt(field.Value)); err != nil {
			return nil, err
		}
		node.Content = append(node.Content, key, value)
	}
	return node, nil
}

// wholeFloatToInt keeps `-f yaml` consistent with every other command: the
// family's YAML encoder normalizes whole-valued floats so a large number
// renders as 1500000 rather than yaml.v3's default 1.5e+06, but its tree-walker
// only descends maps and slices, so it cannot see inside an Ordered.
func wholeFloatToInt(v any) any {
	f, ok := v.(float64)
	if !ok || math.IsInf(f, 0) || math.IsNaN(f) || math.Trunc(f) != f {
		return v
	}
	return int64(f)
}

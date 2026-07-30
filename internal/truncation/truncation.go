// Package truncation implements generic string-length truncation: any string
// field exceeding the configured max length is cut with an ellipsis and gains
// a companion "{field}Length" key holding the full length. Unlike lin (which
// truncates preset field names), this applies to every string field.
package truncation

import (
	"strings"

	out "github.com/shhac/lib-agent-output"
)

const (
	defaultMaxLength = 200
	ellipsis         = "…"
	lengthSuffix     = "Length"
)

type Options struct {
	Expand    string // comma-separated field names exempt from truncation
	Full      bool   // exempt every field
	MaxLength int    // 0 = default (200)
}

var (
	expandAll    bool
	expandFields = map[string]bool{}
	maxLength    = defaultMaxLength
)

// Configure sets process-wide truncation behavior; called once from the root
// command's PersistentPreRun before any output happens.
func Configure(opts Options) {
	expandAll = opts.Full
	expandFields = map[string]bool{}
	if !opts.Full && opts.Expand != "" {
		for _, f := range strings.Split(opts.Expand, ",") {
			expandFields[strings.ToLower(strings.TrimSpace(f))] = true
		}
	}
	maxLength = defaultMaxLength
	if opts.MaxLength > 0 {
		maxLength = opts.MaxLength
	}
}

func shouldExpand(field string) bool { return expandAll || expandFields[field] }

// Apply walks JSON-decoded data (documents, slices, scalars), truncating
// oversized string fields. Only object properties are truncated — a bare string
// inside an array has no field name to hang a {field}Length companion on.
//
// It walks out.Ordered as well as map[string]any because it runs chained after
// out.PruneEmpty over the same tree (see output.pruneTruncate), and the two
// must agree on what counts as a document. A walker that skips a container type
// does not fail loudly; it silently returns that subtree unshaped. Records that
// must not be truncated at all are kept away from this path entirely — the
// output layer rejects them rather than relying on a gap here.
func Apply(data any) any {
	switch v := data.(type) {
	case out.Ordered:
		fields := make(out.Ordered, 0, len(v))
		for _, field := range v {
			if truncated, length, ok := truncateString(field.Key, field.Value); ok {
				fields = append(fields,
					out.Field{Key: field.Key, Value: truncated},
					out.Field{Key: field.Key + lengthSuffix, Value: length})
				continue
			}
			fields = append(fields, out.Field{Key: field.Key, Value: Apply(field.Value)})
		}
		return fields
	case []any:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = Apply(item)
		}
		return items
	case map[string]any:
		fields := make(map[string]any, len(v))
		for key, value := range v {
			if truncated, length, ok := truncateString(key, value); ok {
				fields[key] = truncated
				fields[key+lengthSuffix] = length
				continue
			}
			fields[key] = Apply(value)
		}
		return fields
	}
	return data
}

// truncateString reports whether value is an oversized string and, if so,
// returns what to emit plus the original rune count for the companion key. An
// expanded field keeps its full value and still reports its length, so a caller
// can always tell how long the original was.
func truncateString(key string, value any) (any, int, bool) {
	s, ok := value.(string)
	if !ok {
		return nil, 0, false
	}
	runes := []rune(s)
	if len(runes) <= maxLength {
		return nil, 0, false
	}
	if shouldExpand(key) {
		return s, len(runes), true
	}
	return string(runes[:maxLength]) + ellipsis, len(runes), true
}

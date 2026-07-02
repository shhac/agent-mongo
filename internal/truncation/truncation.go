// Package truncation implements generic string-length truncation: any string
// field exceeding the configured max length is cut with an ellipsis and gains
// a companion "{field}Length" key holding the full length. Unlike lin (which
// truncates preset field names), this applies to every string field.
package truncation

import "strings"

const (
	defaultMaxLength = 200
	ellipsis         = "…"
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

// Apply walks JSON-decoded data (maps, slices, scalars), truncating oversized
// string fields. Only object properties are truncated — a bare string inside
// an array has no field name to hang a {field}Length companion on.
func Apply(data any) any {
	switch v := data.(type) {
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = Apply(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, value := range v {
			if s, ok := value.(string); ok {
				runes := []rune(s)
				if len(runes) > maxLength {
					out[key+"Length"] = len(runes)
					if shouldExpand(key) {
						out[key] = s
					} else {
						out[key] = string(runes[:maxLength]) + ellipsis
					}
					continue
				}
			}
			out[key] = Apply(value)
		}
		return out
	}
	return data
}

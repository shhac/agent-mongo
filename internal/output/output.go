// Package output adapts lib-agent-output's wire contract for agent-mongo:
// NDJSON records on stdout (default), pretty JSON/YAML under --format, and
// pruning + truncation applied to data-bearing results.
package output

import (
	"os"

	out "github.com/shhac/lib-agent-output"

	"github.com/shhac/agent-mongo/internal/truncation"
)

var configuredFormat string

// ConfigureFormat records the root --format flag value; called from the root
// command's PersistentPreRun.
func ConfigureFormat(format string) { configuredFormat = format }

// ResolveFormat returns the effective output format (default NDJSON). The flag
// value is validated by lib-agent-cli's root before commands run.
func ResolveFormat() out.Format {
	format, err := out.ResolveFormat(configuredFormat, out.FormatNDJSON)
	if err != nil {
		return out.FormatNDJSON
	}
	return format
}

// pruneTruncate is the pruner for data-bearing output: strip empty fields
// first, then truncate oversized strings — truncating first could leave a
// companion {field}Length key whose field was later pruned.
func pruneTruncate(v any) any { return truncation.Apply(out.PruneEmpty(v)) }

// PrintResult emits a single data-bearing record (pruned + truncated).
func PrintResult(item any) error {
	return out.Print(os.Stdout, item, ResolveFormat(), pruneTruncate)
}

// PrintRaw emits a single admin/receipt record (pruned, never truncated).
func PrintRaw(item any) error {
	return out.Print(os.Stdout, item, ResolveFormat(), out.PruneEmpty)
}

// PrintList streams records with optional @-metadata (NDJSON) or a {"data":
// [...]} envelope (json/yaml). Items are pruned + truncated.
func PrintList[T any](items []T, meta map[string]any) error {
	widened := make([]any, len(items))
	for i, item := range items {
		widened[i] = item
	}
	return out.WriteList(os.Stdout, ResolveFormat(), widened, meta, pruneTruncate)
}

// MetaKeyMeta is the @-line key carrying per-command context (database,
// collection, sample sizes, …).
const MetaKeyMeta = "@meta"

// Meta wraps command-context fields in the standard @meta metadata entry.
// Compose with PaginationMeta via maps.Copy.
func Meta(fields map[string]any) map[string]any {
	return map[string]any{MetaKeyMeta: fields}
}

// PaginationMeta builds the standard @pagination metadata entry, or nil when
// there is nothing to report.
func PaginationMeta(hasMore bool, nextCursor string, totalItems int) map[string]any {
	if !hasMore && totalItems == 0 {
		return nil
	}
	return map[string]any{
		out.MetaKeyPagination: out.Pagination{
			HasMore:    hasMore,
			NextCursor: nextCursor,
			TotalItems: totalItems,
		},
	}
}

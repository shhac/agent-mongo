// Package output adapts lib-agent-output's wire contract for agent-mongo:
// NDJSON records on stdout (default), pretty JSON/YAML under --format, and
// pruning + truncation applied to data-bearing results.
package output

import (
	"maps"
	"os"
	"slices"

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

// A record built as an out.Ordered is one whose encoded shape is itself the
// data — in this CLI, an index key spec or an echoed query filter. Field order
// survives the normalizing path since lib-agent-output v0.11.0, but the rest of
// that path does not leave such a record intact: PruneEmpty drops a `field:
// null` clause, which is the whole difference between a partial index that
// excludes soft-deleted documents and one that does not, and truncation would
// silently rewrite a long value.
//
// So the normalizing printers reject these records rather than half-mangling
// them. Print them with PrintListVerbatim.
var errVerbatim = out.New(
	"internal: an ordered record was printed through a normalizing path, which would "+
		"drop its nulls and truncate its values — use output.PrintListVerbatim",
	out.FixableByHuman)

// isVerbatim recognizes exactly out.Ordered. It cannot be an opt-in marker
// interface: serialize.Ordered is a type *alias* for the library's type (it has
// to be, or the library's own walkers would not recognize it), and an alias
// cannot carry methods. So a future verbatim-shaped record that is not an
// out.Ordered must be added here explicitly — it has no way to declare itself.
func isVerbatim(v any) bool { _, ok := v.(out.Ordered); return ok }

// rejectVerbatim checks T's zero value as well as the records themselves, so a
// call passing a concrete verbatim slice type is caught even when it is empty.
func rejectVerbatim[T any](items []T) error {
	var zero T
	if isVerbatim(zero) {
		return errVerbatim
	}
	for _, item := range items {
		if isVerbatim(item) {
			return errVerbatim
		}
	}
	return nil
}

// PrintResult emits a single data-bearing record (pruned + truncated).
func PrintResult(item any) error { return printOne(item, pruneTruncate, nil) }

// PrintRaw emits a single admin/receipt record (pruned, never truncated).
func PrintRaw(item any) error { return printOne(item, out.PruneEmpty, nil) }

// PrintResultWithMeta emits a single data-bearing record plus @-prefixed
// metadata. The record is pruned and truncated as usual; the metadata is not —
// it rides through verbatim, matching how WriteList treats a list's metadata,
// which is what lets an echoed query keep its field order and null clauses.
func PrintResultWithMeta(item map[string]any, meta map[string]any) error {
	return printOne(item, pruneTruncate, meta)
}

// printOne is the single-record counterpart to printList: one guard, one prune
// policy, one place that decides where metadata goes.
//
// NDJSON gives metadata its own trailing lines, as it does for lists. The
// single-document formats have nowhere else to put it, so the keys merge into
// the record; being @-prefixed they stay distinguishable from data. Merging
// happens after pruning the record and never touches the metadata itself.
func printOne(item any, prune out.Pruner, meta map[string]any) error {
	if isVerbatim(item) {
		return errVerbatim
	}
	format := ResolveFormat()
	if len(meta) == 0 {
		return out.Print(os.Stdout, item, format, prune)
	}
	if format == out.FormatNDJSON {
		if err := out.Print(os.Stdout, item, format, prune); err != nil {
			return err
		}
		writer := out.NewNDJSONWriter(os.Stdout)
		for _, key := range slices.Sorted(maps.Keys(meta)) {
			if err := writer.WriteMetaLine(key, meta[key]); err != nil {
				return err
			}
		}
		return nil
	}

	merged := map[string]any{}
	if pruned, ok := prune(item).(map[string]any); ok {
		maps.Copy(merged, pruned)
	}
	maps.Copy(merged, meta)
	return out.Print(os.Stdout, merged, format, nil)
}

// PrintList streams records with optional @-metadata (NDJSON) or a {"data":
// [...]} envelope (json/yaml). Items are pruned + truncated.
func PrintList[T any](items []T, meta map[string]any) error {
	if err := rejectVerbatim(items); err != nil {
		return err
	}
	return printList(items, meta, pruneTruncate)
}

// PrintListVerbatim streams records exactly as given: no pruning, no
// truncation, and no reordering of order-preserving types like
// serialize.Ordered. Use it where the shape of a record is itself the answer
// (index specs) — the pruning path normalizes through a JSON round-trip, which
// re-sorts object keys and drops null-valued fields.
func PrintListVerbatim[T any](items []T, meta map[string]any) error {
	return printList(items, meta, nil)
}

func printList[T any](items []T, meta map[string]any, prune out.Pruner) error {
	widened := make([]any, len(items))
	for i, item := range items {
		widened[i] = item
	}
	return out.WriteList(os.Stdout, ResolveFormat(), widened, meta, prune)
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

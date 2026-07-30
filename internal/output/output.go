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

// Verbatim marks a record whose encoded shape is itself the data: field order,
// nulls and exact values all carry meaning. serialize.Ordered is one — an index
// key spec describes a different index once its fields are re-sorted.
//
// The normalizing printers below reject these records rather than quietly
// mangling them. They have to reject rather than adapt: pruning runs on the far
// side of a JSON round-trip that re-sorts object keys and drops nulls, so by
// the time a Pruner sees the record the order is already gone. Print verbatim
// records with PrintListVerbatim.
type Verbatim interface{ VerbatimRecord() }

var errVerbatim = out.New(
	"internal: a verbatim record was printed through a normalizing path, which would "+
		"re-sort its keys and drop its nulls — use output.PrintListVerbatim",
	out.FixableByHuman)

func isVerbatim(v any) bool { _, ok := v.(Verbatim); return ok }

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
func PrintResult(item any) error {
	if isVerbatim(item) {
		return errVerbatim
	}
	return out.Print(os.Stdout, item, ResolveFormat(), pruneTruncate)
}

// PrintRaw emits a single admin/receipt record (pruned, never truncated).
func PrintRaw(item any) error {
	if isVerbatim(item) {
		return errVerbatim
	}
	return out.Print(os.Stdout, item, ResolveFormat(), out.PruneEmpty)
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

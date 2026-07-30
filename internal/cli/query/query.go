// Package query implements `agent-mongo query` — document retrieval.
package query

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-mongo/internal/cli/shared"
	"github.com/shhac/agent-mongo/internal/ejson"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func Register(root *cobra.Command, globals func() *shared.GlobalFlags) {
	cmd := &cobra.Command{Use: "query", Short: "Document retrieval (read-only)"}
	registerEchoFlag(cmd)

	registerFind(cmd, globals)
	registerGet(cmd, globals)
	registerCount(cmd, globals)
	registerSample(cmd, globals)
	registerDistinct(cmd, globals)
	registerAggregate(cmd, globals)
	shared.RegisterUsage(cmd, "query", usageText)

	root.AddCommand(cmd)
}

// parseOptionalDoc parses an EJSON flag value, or returns nil when empty.
func parseOptionalDoc(value, name string) (bson.D, error) {
	if value == "" {
		return nil, nil
	}
	return ejson.Parse(value, name)
}

const usageText = `query — Document retrieval (read-only)

COMMANDS:
  query find <database> <collection> [--filter <json>] [--sort <json>] [--projection <json>] [--limit <n>] [--skip <n>] [-c <alias>]
    Find documents matching a filter. Default sort: { _id: -1 }. Default limit: 20.
    One record per document; a trailing {"@pagination": ...} line carries
    has_more and total_items (total matching count).

  query get <database> <collection> <id> [--type objectid|string|number] [--projection <json>] [-c <alias>]
    Get a single document by _id. Auto-detects ObjectId (24-char hex) vs string.
    Use --type to force id interpretation. Use --projection to select specific fields.
    Returns { database, collection, fieldCount, document }. Use fieldCount to decide if --projection is needed.

  query count <database> <collection> [--filter <json>] [-c <alias>]
    Count documents matching a filter. Omit --filter for total count.

  query sample <database> <collection> [--size <n>] [--filter <json>] [-c <alias>]
    Get random documents. Default size: 5 (configurable via defaults.sampleSize).
    Use --filter to sample from a subset of documents.

  query distinct <database> <collection> <field> [--filter <json>] [-c <alias>]
    Get distinct values for a field. Supports dot notation (e.g. address.city).

  query aggregate <database> <collection> [pipeline] [--pipeline <json>] [--limit <n>] [-c <alias>]
    Run aggregation pipeline. Write stages ($out, $merge) are rejected.
    Pipeline can be passed as positional arg, via --pipeline flag, or piped via stdin.

ECHO: --echo-query adds an {"@query": ...} line reporting what actually ran —
  filter, sort, projection, limit, skip, pipeline, as sent to the server,
  including defaults the CLI applied (e.g. sort {_id:-1}, the resolved limit).
  Emitted verbatim: field order is the real order and null clauses are kept, so
  it can be compared against the query you meant to send. Off by default.
  For single-result commands (count, distinct, get) the @query keys merge into
  the record under -f json/yaml, where there is no separate metadata line.

JSON ARGS: All --filter, --sort, --projection, --pipeline values accept MongoDB Extended JSON (EJSON).
  Use {"$date":"2026-01-01T00:00:00Z"} for dates, {"$oid":"..."} for ObjectIds, etc.

LIMITS: Results capped at query.maxDocuments (default 100). Timeout: query.timeout (default 30s).
  Override per-command with -t/--timeout <ms>. On timeout, hints suggest increasing timeout or checking indexes.

OUTPUT: NDJSON — one JSON record per line; metadata on @-prefixed lines.
  Use -f json for a pretty {"data": [...]} envelope.

EXAMPLES:
  agent-mongo query find myapp users --filter '{"age":{"$gte":21}}' --limit 10
  agent-mongo query get myapp users 665a1b2c3d4e5f6a7b8c9d0e --projection '{"name":1,"email":1}'
  agent-mongo query count myapp orders --filter '{"status":"pending"}'
  agent-mongo query sample myapp users --size 10 --filter '{"status":"active"}'
  agent-mongo query distinct myapp orders status
  agent-mongo query aggregate myapp orders '[{"$group":{"_id":"$status","count":{"$sum":1}}}]'
  agent-mongo query aggregate myapp orders --pipeline '[{"$group":{"_id":"$status","count":{"$sum":1}}}]'
  agent-mongo query count myapp orders --filter '{"status":"pending","deletedAt":null}' --echo-query`

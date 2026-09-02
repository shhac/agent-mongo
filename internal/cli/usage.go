package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const usageText = `agent-mongo — MongoDB CLI for AI agents (NDJSON output, read-only)

COMMANDS:
  connection add|remove|update|list|test|set-default   Manage MongoDB connections
  credential add|remove|list|login|logout              Manage stored credentials
                                                       (--oidc for identity-provider auth)
  config get|set|reset|list-keys                       Persistent settings

  database list                                         List all databases
  database stats <database>                             Database statistics

  collection list <database>                      List collections
  collection schema <database> <collection>       Infer schema from samples
  collection indexes <database> <collection>      List indexes (verbatim specs)
  collection stats <database> <collection>        Collection statistics

  query find <database> <collection> [--filter] [--sort]              Find documents
  query get <database> <collection> <id> [--projection]        Get document by _id
  query count <database> <collection> [--filter]              Count documents
  query sample <database> <collection> [--size] [--filter]    Random documents
  query distinct <database> <collection> <field>              Distinct field values
  query aggregate <database> <collection> [pipeline] [--pipeline <json>]   Aggregation pipeline

GLOBAL FLAGS: -c <alias> (connection), --expand <fields>, --full,
  -t/--timeout <ms>, -f/--format <jsonl|json|yaml>
QUERY FLAG: --echo-query echoes the executed query on an {"@query": ...} line
  (verbatim: real field order, nulls kept). Off by default.

CONNECTION: -c flag > AGENT_MONGO_CONNECTION env > config default.
  Connections can reference stored credentials via --credential for shared auth.
  Credentials are scram (username/password) or oidc (identity provider, no
  stored secret; requires TLS and an allowed host).
  A user:pass embedded in an added URI is auto-extracted into a credential
  named after the connection alias; displayed connection strings redact passwords.

SAFETY: Read-only. No write operations. Aggregation rejects $out/$merge.
  Results capped at query.maxDocuments (default 100). Timeout: query.timeout (default 30s).

OUTPUT: NDJSON to stdout — one JSON record per line; metadata rides on
  @-prefixed lines (e.g. {"@pagination": ...}). Use -f json for a pretty
  {"data": [...]} envelope. Errors: {"error", "fixable_by", "hint"} to stderr,
  exit code 1. Records are normalized — object keys sorted, empty/null fields
  pruned, long strings truncated. "collection indexes" is exempt: index specs
  come out verbatim (real key order, nulls kept) so they can be compared
  byte-for-byte against a declared index.

DETAIL: Run "<command> usage" for per-command docs.`

func registerUsage(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "usage",
		Short: "Print concise documentation (LLM-optimized)",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(strings.TrimSpace(usageText))
		},
	})
}

import type { Command } from "commander";

const USAGE_TEXT = `agent-mongo — MongoDB CLI for AI agents (JSON output, read-only)

COMMANDS:
  connection add|remove|update|list|test|set-default   Manage MongoDB connections
  credential add|remove|list                           Manage stored credentials
  config get|set|reset|list-keys                       Persistent settings

  db list                                         List all databases
  db stats <database>                             Database statistics

  collection list <database>                      List collections
  collection schema <database> <collection>       Infer schema from samples
  collection indexes <database> <collection>      List indexes
  collection stats <database> <collection>        Collection statistics

  query find <db> <coll> [--filter] [--sort]      Find documents
  query get <db> <coll> <id>                      Get document by _id
  query count <db> <coll> [--filter]              Count documents
  query sample <db> <coll> [--size]               Random documents
  query distinct <db> <coll> <field>              Distinct field values
  query aggregate <db> <coll> --pipeline <json>   Aggregation pipeline

GLOBAL FLAGS: -c <alias> (connection), --expand <fields>, --full

CONNECTION: -c flag > AGENT_MONGO_CONNECTION env > config default.
  Connections can reference stored credentials via --credential for shared auth.

SAFETY: Read-only. No write operations. Aggregation rejects $out/$merge.
  Results capped at query.maxDocuments (default 100). Timeout: query.timeout (default 30s).

OUTPUT: JSON to stdout. Errors: { "error": "..." } to stderr, exit code 1.

DETAIL: Run "<command> usage" for per-command docs.
`;

export function registerUsageCommand({ program }: { program: Command }): void {
  program
    .command("usage")
    .description("Print concise documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}

import type { Command } from "commander";

const USAGE_TEXT = `agent-mongo — MongoDB CLI for AI agents (JSON output, read-only)

COMMANDS:
  connection add|remove|update|list|test|set-default   Manage MongoDB connections
  credential add|remove|list                           Manage stored credentials
  config get|set|reset|list-keys                       Persistent settings

  database list                                         List all databases
  database stats <database>                             Database statistics

  collection list <database>                      List collections
  collection schema <database> <collection>       Infer schema from samples
  collection indexes <database> <collection>      List indexes
  collection stats <database> <collection>        Collection statistics

  query find <database> <collection> [--filter] [--sort]      Find documents
  query get <database> <collection> <id> [--projection]        Get document by _id
  query count <database> <collection> [--filter]              Count documents
  query sample <database> <collection> [--size] [--filter]    Random documents
  query distinct <database> <collection> <field>              Distinct field values
  query aggregate <database> <collection> [pipeline] [--pipeline <json>]   Aggregation pipeline

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

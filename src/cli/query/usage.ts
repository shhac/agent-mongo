import type { Command } from "commander";

const USAGE_TEXT = `query — Document retrieval (read-only)

COMMANDS:
  query find <database> <collection> [--filter <json>] [--sort <json>] [--projection <json>] [--limit <n>] [--skip <n>] [-c <alias>]
    Find documents matching a filter. Default sort: { _id: -1 }. Default limit: 20.
    Returns documents, count, hasMore flag, and totalMatching count.

  query get <database> <collection> <id> [--type objectid|string|number] [--projection <json>] [-c <alias>]
    Get a single document by _id. Auto-detects ObjectId (24-char hex) vs string.
    Use --type to force id interpretation. Use --projection to select specific fields.

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

JSON ARGS: All --filter, --sort, --projection, --pipeline values must be valid JSON.

LIMITS: Results capped at query.maxDocuments (default 100). Timeout: query.timeout (default 30s).
  On timeout, hints suggest increasing timeout or checking indexes.

EXAMPLES:
  agent-mongo query find myapp users --filter '{"age":{"$gte":21}}' --limit 10
  agent-mongo query get myapp users 665a1b2c3d4e5f6a7b8c9d0e --projection '{"name":1,"email":1}'
  agent-mongo query count myapp orders --filter '{"status":"pending"}'
  agent-mongo query sample myapp users --size 10 --filter '{"status":"active"}'
  agent-mongo query distinct myapp orders status
  agent-mongo query aggregate myapp orders '[{"$group":{"_id":"$status","count":{"$sum":1}}}]'
  agent-mongo query aggregate myapp orders --pipeline '[{"$group":{"_id":"$status","count":{"$sum":1}}}]'
`;

export function registerUsage(parent: Command): void {
  parent
    .command("usage")
    .description("Print query command documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}

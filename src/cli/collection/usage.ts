import type { Command } from "commander";

const USAGE_TEXT = `collection — Collection discovery

COMMANDS:
  collection list <database> [-c <alias>]
    List all collections in a database. Returns name and type (collection or view).

  collection schema <database> <collection> [--sample-size <n>] [--depth <n>] [--limit <n>] [--skip <n>] [-c <alias>]
    Infer collection schema by sampling documents. Default sample: 100 (configurable via defaults.schemaSampleSize).
    Returns field paths, types, and presence rates (0.0-1.0).
    Array element types shown as "path.$" entries.
    Errors if collection does not exist.
    --depth <n>    Limit nesting depth (1 = top-level only, 2 = one level of nesting, etc.)
    --limit <n>    Max fields to return (paginate large schemas)
    --skip <n>     Number of fields to skip (use with --limit for pagination)
    When paginated, includes totalFields count and pagination.nextSkip for the next page.

  collection indexes <database> <collection> [-c <alias>]
    List all indexes with key patterns, uniqueness, and other properties.

  collection stats <database> <collection> [-c <alias>]
    Get collection statistics: document count, data/storage/index sizes, capped flag.

EXAMPLES:
  agent-mongo collection list myapp
  agent-mongo collection schema myapp users
  agent-mongo collection schema myapp users --sample-size 500
  agent-mongo collection schema myapp events --depth 2
  agent-mongo collection schema myapp events --limit 50
  agent-mongo collection schema myapp events --limit 50 --skip 50
  agent-mongo collection indexes myapp users
  agent-mongo collection stats myapp orders
`;

export function registerUsage(parent: Command): void {
  parent
    .command("usage")
    .description("Print collection command documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}

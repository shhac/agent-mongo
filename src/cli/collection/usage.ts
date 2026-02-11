import type { Command } from "commander";

const USAGE_TEXT = `collection — Collection discovery

COMMANDS:
  collection list <database> [-c <alias>]
    List all collections in a database. Returns name and type (collection or view).

  collection schema <database> <collection> [--sample-size <n>] [-c <alias>]
    Infer collection schema by sampling documents. Default: 100 (configurable via defaults.schemaSampleSize).
    Returns field paths, types, and presence rates (0.0-1.0).
    Array element types shown as "path.$" entries.

  collection indexes <database> <collection> [-c <alias>]
    List all indexes with key patterns, uniqueness, and other properties.

  collection stats <database> <collection> [-c <alias>]
    Get collection statistics: document count, data/storage/index sizes, capped flag.

EXAMPLES:
  agent-mongo collection list myapp
  agent-mongo collection schema myapp users
  agent-mongo collection schema myapp users --sample-size 500
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

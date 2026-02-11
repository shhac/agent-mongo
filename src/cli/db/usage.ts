import type { Command } from "commander";

const USAGE_TEXT = `db — Database discovery

COMMANDS:
  db list [-c <alias>]
    List all databases with sizes. Returns name, sizeOnDisk, empty flag, and totalSize.

  db stats <database> [-c <alias>]
    Get database statistics: collection count, document count, data/storage/index sizes.

EXAMPLES:
  agent-mongo db list
  agent-mongo db list -c production
  agent-mongo db stats myapp
`;

export function registerUsage(db: Command): void {
  db.command("usage")
    .description("Print db command documentation (LLM-optimized)")
    .action(() => {
      console.log(USAGE_TEXT.trim());
    });
}

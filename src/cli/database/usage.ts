import { Command, NoArgs } from "vipvot";

const USAGE_TEXT = `database — Database discovery

COMMANDS:
  database list [-c <alias>]
    List all databases with sizes. Returns name, sizeOnDisk, empty flag, and totalSize.

  database stats <database> [-c <alias>]
    Get database statistics: collection count, document count, data/storage/index sizes.

EXAMPLES:
  agent-mongo database list
  agent-mongo database list -c production
  agent-mongo database stats myapp
`;

export function buildUsageCommand(): Command {
  return Command({
    use: "usage",
    short: "Print database command documentation (LLM-optimized)",
    args: NoArgs,
    run: () => {
      console.log(USAGE_TEXT.trim());
    },
  });
}

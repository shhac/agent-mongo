import { Command } from "vipvot";
import { buildListCommand } from "./list.ts";
import { buildSchemaCommand } from "./schema.ts";
import { buildIndexesCommand } from "./indexes.ts";
import { buildStatsCommand } from "./stats.ts";
import { buildUsageCommand } from "./usage.ts";

export function buildCollectionCommand(): Command {
  const cmd = Command({
    use: "collection",
    short: "Collection discovery",
  });
  cmd.addCommand(buildListCommand());
  cmd.addCommand(buildSchemaCommand());
  cmd.addCommand(buildIndexesCommand());
  cmd.addCommand(buildStatsCommand());
  cmd.addCommand(buildUsageCommand());
  return cmd;
}

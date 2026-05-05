import { Command } from "vipvot";
import { buildListCommand } from "./list.ts";
import { buildStatsCommand } from "./stats.ts";
import { buildUsageCommand } from "./usage.ts";

export function buildDatabaseCommand(): Command {
  const cmd = Command({
    use: "database",
    short: "Database discovery",
  });
  cmd.addCommand(buildListCommand());
  cmd.addCommand(buildStatsCommand());
  cmd.addCommand(buildUsageCommand());
  return cmd;
}

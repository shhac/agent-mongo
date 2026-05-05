import { Command } from "vipvot";
import { buildFindCommand } from "./find.ts";
import { buildGetCommand } from "./get.ts";
import { buildCountCommand } from "./count.ts";
import { buildSampleCommand } from "./sample.ts";
import { buildDistinctCommand } from "./distinct.ts";
import { buildAggregateCommand } from "./aggregate.ts";
import { buildUsageCommand } from "./usage.ts";

export function buildQueryCommand(): Command {
  const cmd = Command({
    use: "query",
    short: "Query documents (read-only)",
  });
  cmd.addCommand(buildFindCommand());
  cmd.addCommand(buildGetCommand());
  cmd.addCommand(buildCountCommand());
  cmd.addCommand(buildSampleCommand());
  cmd.addCommand(buildDistinctCommand());
  cmd.addCommand(buildAggregateCommand());
  cmd.addCommand(buildUsageCommand());
  return cmd;
}

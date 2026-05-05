import { Command } from "vipvot";
import { buildGetCommand } from "./get.ts";
import { buildSetCommand } from "./set.ts";
import { buildResetCommand } from "./reset.ts";
import { buildListKeysCommand } from "./list-keys.ts";
import { buildUsageCommand } from "./usage.ts";

export function buildConfigCommand(): Command {
  const cmd = Command({
    use: "config",
    short: "Manage CLI settings",
  });
  cmd.addCommand(buildGetCommand());
  cmd.addCommand(buildSetCommand());
  cmd.addCommand(buildResetCommand());
  cmd.addCommand(buildListKeysCommand());
  cmd.addCommand(buildUsageCommand());
  return cmd;
}

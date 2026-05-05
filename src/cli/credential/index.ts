import { Command } from "vipvot";
import { buildAddCommand } from "./add.ts";
import { buildRemoveCommand } from "./remove.ts";
import { buildListCommand } from "./list.ts";
import { buildUsageCommand } from "./usage.ts";

export function buildCredentialCommand(): Command {
  const cmd = Command({
    use: "credential",
    short: "Manage stored credentials",
  });
  cmd.addCommand(buildAddCommand());
  cmd.addCommand(buildRemoveCommand());
  cmd.addCommand(buildListCommand());
  cmd.addCommand(buildUsageCommand());
  return cmd;
}

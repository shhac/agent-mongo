import { Command } from "vipvot";
import { buildAddCommand } from "./add.ts";
import { buildRemoveCommand } from "./remove.ts";
import { buildUpdateCommand } from "./update.ts";
import { buildListCommand } from "./list.ts";
import { buildTestCommand } from "./test.ts";
import { buildSetDefaultCommand } from "./set-default.ts";
import { buildUsageCommand } from "./usage.ts";

export function buildConnectionCommand(): Command {
  const cmd = Command({
    use: "connection",
    short: "Manage MongoDB connections",
  });
  cmd.addCommand(buildAddCommand());
  cmd.addCommand(buildRemoveCommand());
  cmd.addCommand(buildUpdateCommand());
  cmd.addCommand(buildListCommand());
  cmd.addCommand(buildTestCommand());
  cmd.addCommand(buildSetDefaultCommand());
  cmd.addCommand(buildUsageCommand());
  return cmd;
}

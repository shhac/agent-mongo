import type { Command } from "commander";
import { registerAdd } from "./add.ts";
import { registerRemove } from "./remove.ts";
import { registerList } from "./list.ts";
import { registerTest } from "./test.ts";
import { registerSetDefault } from "./set-default.ts";
import { registerUsage } from "./usage.ts";

export function registerConnectionCommand({ program }: { program: Command }): void {
  const connection = program.command("connection").description("Manage MongoDB connections");
  registerAdd(connection);
  registerRemove(connection);
  registerList(connection);
  registerTest(connection);
  registerSetDefault(connection);
  registerUsage(connection);
}

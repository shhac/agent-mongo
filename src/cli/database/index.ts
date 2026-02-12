import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerStats } from "./stats.ts";
import { registerUsage } from "./usage.ts";

export function registerDatabaseCommand({ program }: { program: Command }): void {
  const database = program.command("database").description("Database discovery");
  registerList(database);
  registerStats(database);
  registerUsage(database);
}

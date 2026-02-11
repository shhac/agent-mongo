import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerStats } from "./stats.ts";
import { registerUsage } from "./usage.ts";

export function registerDbCommand({ program }: { program: Command }): void {
  const db = program.command("db").description("Database discovery");
  registerList(db);
  registerStats(db);
  registerUsage(db);
}

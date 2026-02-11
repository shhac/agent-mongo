import type { Command } from "commander";
import { registerList } from "./list.ts";
import { registerSchema } from "./schema.ts";
import { registerIndexes } from "./indexes.ts";
import { registerStats } from "./stats.ts";
import { registerUsage } from "./usage.ts";

export function registerCollectionCommand({ program }: { program: Command }): void {
  const collection = program.command("collection").description("Collection discovery");
  registerList(collection);
  registerSchema(collection);
  registerIndexes(collection);
  registerStats(collection);
  registerUsage(collection);
}

import type { Command } from "commander";
import { registerFind } from "./find.ts";
import { registerGet } from "./get.ts";
import { registerCount } from "./count.ts";
import { registerSample } from "./sample.ts";
import { registerDistinct } from "./distinct.ts";
import { registerAggregate } from "./aggregate.ts";
import { registerUsage } from "./usage.ts";

export function registerQueryCommand({ program }: { program: Command }): void {
  const query = program.command("query").description("Query documents (read-only)");
  registerFind(query);
  registerGet(query);
  registerCount(query);
  registerSample(query);
  registerDistinct(query);
  registerAggregate(query);
  registerUsage(query);
}

import type { Command } from "commander";
import { getSetting } from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";
import { VALID_KEYS } from "./valid-keys.ts";

export function registerGet(config: Command): void {
  config
    .command("get")
    .description("Get a config value")
    .argument("<key>", "Config key (e.g. defaults.limit)")
    .action((key: string) => {
      try {
        if (!VALID_KEYS.has(key)) {
          throw new Error(`Unknown key: "${key}". Valid keys: ${[...VALID_KEYS].join(", ")}`);
        }
        const value = getSetting(key);
        printJson({ key, value: value ?? null });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Get failed");
      }
    });
}

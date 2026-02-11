import type { Command } from "commander";
import { updateSetting } from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";
import { VALID_KEYS, parseConfigValue } from "./valid-keys.ts";

export function registerSet(config: Command): void {
  config
    .command("set")
    .description("Set a config value")
    .argument("<key>", "Config key (e.g. defaults.limit)")
    .argument("<value>", "Value to set")
    .action((key: string, rawValue: string) => {
      try {
        if (!VALID_KEYS.has(key)) {
          throw new Error(`Unknown key: "${key}". Valid keys: ${[...VALID_KEYS].join(", ")}`);
        }
        const value = parseConfigValue(key, rawValue);
        updateSetting(key, value);
        printJson({ ok: true, key, value });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Set failed");
      }
    });
}

import type { Command } from "commander";
import { printJson } from "../../lib/output.ts";
import { KEY_DEFINITIONS } from "./valid-keys.ts";

export function registerListKeys(config: Command): void {
  config
    .command("list-keys")
    .description("List all valid config keys with defaults")
    .action(() => {
      printJson({
        keys: KEY_DEFINITIONS.map((k) => ({
          key: k.key,
          type: k.type,
          default: k.defaultValue,
          description: k.description,
        })),
      });
    });
}

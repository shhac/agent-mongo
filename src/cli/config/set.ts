import { Command, ExactArgs } from "vipvot";
import { updateSetting } from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";
import { VALID_KEYS, parseConfigValue } from "./valid-keys.ts";

export function buildSetCommand(): Command {
  return Command({
    use: "set <key> <value>",
    short: "Set a config value",
    args: ExactArgs(2),
    run: (_cmd, args) => {
      const [key, rawValue] = args as [string, string];
      try {
        if (!VALID_KEYS.has(key)) {
          throw new Error(`Unknown key: "${key}". Valid keys: ${[...VALID_KEYS].join(", ")}`);
        }
        const value = parseConfigValue(key, rawValue);
        updateSetting(key, value);
        printJsonRaw({ ok: true, key, value });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Set failed");
      }
    },
  });
}

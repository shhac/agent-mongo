import { Command, ExactArgs } from "vipvot";
import { getSetting } from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";
import { VALID_KEYS } from "./valid-keys.ts";

export function buildGetCommand(): Command {
  return Command({
    use: "get <key>",
    short: "Get a config value",
    args: ExactArgs(1),
    run: (_cmd, args) => {
      const [key] = args as [string];
      try {
        if (!VALID_KEYS.has(key)) {
          throw new Error(`Unknown key: "${key}". Valid keys: ${[...VALID_KEYS].join(", ")}`);
        }
        const value = getSetting(key);
        printJsonRaw({ key, value: value ?? null });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Get failed");
      }
    },
  });
}

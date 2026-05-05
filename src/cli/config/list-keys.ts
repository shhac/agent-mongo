import { Command, NoArgs } from "vipvot";
import { printJsonRaw } from "../../lib/output.ts";
import { KEY_DEFINITIONS } from "./valid-keys.ts";

export function buildListKeysCommand(): Command {
  return Command({
    use: "list-keys",
    short: "List all valid config keys with defaults",
    args: NoArgs,
    run: () => {
      printJsonRaw({
        keys: KEY_DEFINITIONS.map((k) => ({
          key: k.key,
          type: k.type,
          default: k.defaultValue,
          description: k.description,
        })),
      });
    },
  });
}

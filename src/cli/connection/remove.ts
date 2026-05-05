import { Command, ExactArgs } from "vipvot";
import { removeConnection } from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function buildRemoveCommand(): Command {
  return Command({
    use: "remove <alias>",
    short: "Remove a saved connection",
    args: ExactArgs(1),
    run: (_cmd, args) => {
      const [alias] = args as [string];
      try {
        removeConnection(alias);
        printJsonRaw({ ok: true, removed: alias });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to remove connection");
      }
    },
  });
}

import { Command, ExactArgs } from "vipvot";
import { setDefaultConnection } from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function buildSetDefaultCommand(): Command {
  return Command({
    use: "set-default <alias>",
    short: "Set the default connection",
    args: ExactArgs(1),
    run: (_cmd, args) => {
      const [alias] = args as [string];
      try {
        setDefaultConnection(alias);
        printJsonRaw({ ok: true, default: alias });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to set default");
      }
    },
  });
}

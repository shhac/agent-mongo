import { Command, NoArgs } from "vipvot";
import { resetSettings } from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function buildResetCommand(): Command {
  return Command({
    use: "reset",
    short: "Reset all settings to defaults",
    args: NoArgs,
    run: () => {
      try {
        resetSettings();
        printJsonRaw({ ok: true, message: "Settings reset to defaults" });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Reset failed");
      }
    },
  });
}

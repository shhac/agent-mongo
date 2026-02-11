import type { Command } from "commander";
import { setDefaultConnection } from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function registerSetDefault(connection: Command): void {
  connection
    .command("set-default")
    .description("Set the default connection")
    .argument("<alias>", "Connection alias to set as default")
    .action((alias: string) => {
      try {
        setDefaultConnection(alias);
        printJsonRaw({ ok: true, default: alias });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to set default");
      }
    });
}

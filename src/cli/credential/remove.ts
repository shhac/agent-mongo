import type { Command } from "commander";
import {
  removeCredential,
  getConnectionsUsingCredential,
  updateConnection,
} from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerRemove(credential: Command): void {
  credential
    .command("remove")
    .description("Remove a stored credential")
    .argument("<name>", "Credential name to remove")
    .option("--force", "Remove even if referenced by connections (clears their credential refs)")
    .action((name: string, opts: { force?: boolean }) => {
      try {
        const usedBy = getConnectionsUsingCredential(name);

        if (usedBy.length > 0 && opts.force) {
          for (const connAlias of usedBy) {
            updateConnection(connAlias, { credential: undefined });
          }
        }

        removeCredential(name);
        printJson({
          ok: true,
          removed: name,
          clearedFrom: usedBy.length > 0 && opts.force ? usedBy : undefined,
        });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to remove credential");
      }
    });
}

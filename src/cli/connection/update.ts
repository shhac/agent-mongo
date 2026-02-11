import type { Command } from "commander";
import { updateConnection, getCredential, getCredentials } from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerUpdate(connection: Command): void {
  connection
    .command("update")
    .description("Update a saved connection")
    .argument("<alias>", "Connection alias to update")
    .option("--credential <name>", "Credential alias for authentication")
    .option("--no-credential", "Remove credential from connection")
    .option("--database <db>", "Override database name")
    .action((alias: string, opts: { credential?: string | false; database?: string }) => {
      try {
        if (typeof opts.credential === "string") {
          const cred = getCredential(opts.credential);
          if (!cred) {
            const available = Object.keys(getCredentials());
            throw new Error(
              `Credential "${opts.credential}" not found. Available: ${available.join(", ") || "(none)"}. Run: agent-mongo credential add <alias> --username <user> --password <pass>`,
            );
          }
        }

        const updates: Record<string, string | undefined> = {};
        if (opts.database !== undefined) {
          updates.database = opts.database;
        }
        if (opts.credential === false) {
          updates.credential = undefined;
        } else if (typeof opts.credential === "string") {
          updates.credential = opts.credential;
        }

        updateConnection(alias, updates);
        printJson({ ok: true, alias, updated: Object.keys(updates) });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to update connection");
      }
    });
}

import type { Command } from "commander";
import {
  storeConnection,
  setDefaultConnection,
  getCredential,
  getCredentials,
} from "../../lib/config.ts";
import { parseDbFromUri } from "../../mongo/client.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerAdd(connection: Command): void {
  connection
    .command("add")
    .description("Add a MongoDB connection")
    .argument("<alias>", "Short name for this connection (e.g. local, staging, prod)")
    .argument("<connection-string>", "MongoDB connection URI (mongodb:// or mongodb+srv://)")
    .option("--database <db>", "Override database name from URI")
    .option("--credential <name>", "Credential alias for authentication")
    .option("--default", "Set as default connection")
    .action((...args: unknown[]) => {
      const [alias, connectionString, opts] = args as [
        string,
        string,
        { database?: string; credential?: string; default?: boolean },
      ];
      try {
        if (opts.credential) {
          const cred = getCredential(opts.credential);
          if (!cred) {
            const available = Object.keys(getCredentials());
            throw new Error(
              `Credential "${opts.credential}" not found. Available: ${available.join(", ") || "(none)"}. Run: agent-mongo credential add <alias> --username <user> --password <pass>`,
            );
          }
        }

        storeConnection(alias, {
          connection_string: connectionString,
          name: alias,
          database: opts.database,
          credential: opts.credential,
        });

        if (opts.default) {
          setDefaultConnection(alias);
        }

        printJson({
          ok: true,
          alias,
          database: opts.database ?? parseDbFromUri(connectionString),
          credential: opts.credential,
          isDefault: opts.default ?? false,
          hint: "Test with: agent-mongo connection test",
        });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to add connection");
      }
    });
}

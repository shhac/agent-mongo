import type { Command } from "commander";
import { storeConnection } from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerAdd(connection: Command): void {
  connection
    .command("add")
    .description("Add a MongoDB connection")
    .argument("<alias>", "Short name for this connection (e.g. local, staging, prod)")
    .argument("<connection-string>", "MongoDB connection URI (mongodb:// or mongodb+srv://)")
    .option("--database <db>", "Override database name from URI")
    .option("--default", "Set as default connection")
    .action((...args: unknown[]) => {
      const [alias, connectionString, opts] = args as [
        string,
        string,
        { database?: string; default?: boolean },
      ];
      try {
        storeConnection(alias, {
          connection_string: connectionString,
          name: alias,
          database: opts.database,
        });

        printJson({
          ok: true,
          alias,
          database: opts.database ?? parseDbFromUri(connectionString),
          hint: "Test with: agent-mongo connection test",
        });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to add connection");
      }
    });
}

function parseDbFromUri(uri: string): string | undefined {
  try {
    const url = new URL(uri);
    const path = url.pathname.replace(/^\//, "");
    return path || undefined;
  } catch {
    return undefined;
  }
}

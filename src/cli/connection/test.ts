import type { Command } from "commander";
import { MongoClient } from "mongodb";
import { getConnection, getDefaultConnectionAlias, getConnections } from "../../lib/config.ts";
import { printError, printJson } from "../../lib/output.ts";

export function registerTest(connection: Command): void {
  connection
    .command("test")
    .description("Test a MongoDB connection (ping)")
    .action(async (_opts: unknown, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection ?? getDefaultConnectionAlias();
        if (!alias) {
          const available = Object.keys(getConnections());
          throw new Error(
            `No connection specified. Available: ${available.join(", ") || "(none)"}. Run: agent-mongo connection add <alias> <connection-string>`,
          );
        }

        const conn = getConnection(alias);
        if (!conn) {
          const available = Object.keys(getConnections());
          throw new Error(
            `Connection "${alias}" not found. Available: ${available.join(", ") || "(none)"}`,
          );
        }

        const client = new MongoClient(conn.connection_string);
        try {
          await client.connect();
          const result = await client.db("admin").command({ ping: 1 });
          printJson({
            ok: true,
            alias,
            ping: result,
          });
        } finally {
          await client.close();
        }
      } catch (err) {
        printError(err instanceof Error ? err.message : "Connection test failed");
      }
    });
}

import type { Command } from "commander";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function registerTest(connection: Command): void {
  connection
    .command("test")
    .description("Test a MongoDB connection (ping)")
    .action(async (_opts: unknown, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection;
        const { client, alias: resolved } = await getMongoClient(alias);
        try {
          const result = await client.db("admin").command({ ping: 1 });
          printJsonRaw({
            ok: true,
            alias: resolved,
            ping: result,
          });
        } finally {
          await closeAllClients();
        }
      } catch (err) {
        printError(err instanceof Error ? err.message : "Connection test failed");
      }
    });
}

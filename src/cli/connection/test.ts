import type { Command } from "commander";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function registerTest(connection: Command): void {
  connection
    .command("test")
    .description("Test a MongoDB connection (ping)")
    .argument("[alias]", "Connection alias to test (overrides -c flag)")
    // oxlint-disable-next-line max-params -- commander dictates this signature
    .action(async (aliasArg: string | undefined, _opts: unknown, command: Command) => {
      try {
        const alias = aliasArg ?? command.optsWithGlobals().connection;
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

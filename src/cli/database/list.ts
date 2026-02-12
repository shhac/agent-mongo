import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { listDatabases } from "../../mongo/databases.ts";

export function registerList(database: Command): void {
  database.command("list")
    .description("List all databases")
    .action(async (_opts: unknown, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection;
        const { client } = await getMongoClient(alias);
        const result = await listDatabases(client);
        printJson(result);
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to list databases");
      } finally {
        await closeAllClients();
      }
    });
}

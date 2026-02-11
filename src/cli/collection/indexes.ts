import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { listIndexes } from "../../mongo/indexes.ts";

export function registerIndexes(parent: Command): void {
  parent
    .command("indexes")
    .description("List indexes on a collection")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .action(async (database: string, collection: string, _opts: unknown, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection;
        const { client } = await getMongoClient(alias);
        const indexes = await listIndexes(client, database, collection);
        printJson({ database, collection, indexes });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to list indexes");
      } finally {
        await closeAllClients();
      }
    });
}

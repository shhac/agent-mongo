import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { getCollectionStats } from "../../mongo/collections.ts";

export function registerStats(parent: Command): void {
  parent
    .command("stats")
    .description("Get collection statistics")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .action(async (database: string, collection: string, _opts: unknown, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection;
        const { client } = await getMongoClient(alias);
        const result = await getCollectionStats(client, database, collection);
        printJson(result);
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to get collection stats");
      } finally {
        await closeAllClients();
      }
    });
}

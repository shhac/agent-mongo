import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { listCollections } from "../../mongo/collections.ts";

export function registerList(parent: Command): void {
  parent
    .command("list")
    .description("List collections in a database")
    .argument("<database>", "Database name")
    // oxlint-disable-next-line max-params -- commander dictates this signature
    .action(async (database: string, _opts: unknown, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection;
        const { client } = await getMongoClient(alias);
        const collections = await listCollections(client, database);
        printJson({ database, collections });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to list collections");
      } finally {
        await closeAllClients();
      }
    });
}

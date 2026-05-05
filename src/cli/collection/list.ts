import { Command, ExactArgs } from "vipvot";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { listCollections } from "../../mongo/collections.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildListCommand(): Command {
  return Command({
    use: "list <database>",
    short: "List collections in a database",
    args: ExactArgs(1),
    run: async (_cmd, args) => {
      const [database] = args as [string];
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());
        const collections = await listCollections(client, database);
        printJson({ database, collections });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to list collections");
      } finally {
        await closeAllClients();
      }
    },
  });
}

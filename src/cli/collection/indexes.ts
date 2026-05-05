import { Command, ExactArgs } from "vipvot";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { listIndexes } from "../../mongo/indexes.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildIndexesCommand(): Command {
  return Command({
    use: "indexes <database> <collection>",
    short: "List indexes on a collection",
    args: ExactArgs(2),
    run: async (_cmd, args) => {
      const [database, collection] = args as [string, string];
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());
        const indexes = await listIndexes(client, { dbName: database, collName: collection });
        printJson({ database, collection, indexes });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to list indexes");
      } finally {
        await closeAllClients();
      }
    },
  });
}

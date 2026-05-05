import { Command, ExactArgs } from "vipvot";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { getCollectionStats } from "../../mongo/collections.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildStatsCommand(): Command {
  return Command({
    use: "stats <database> <collection>",
    short: "Get collection statistics",
    args: ExactArgs(2),
    run: async (_cmd, args) => {
      const [database, collection] = args as [string, string];
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());
        const result = await getCollectionStats(client, { dbName: database, collName: collection });
        printJson(result);
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to get collection stats");
      } finally {
        await closeAllClients();
      }
    },
  });
}

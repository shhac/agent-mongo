import { Command, ExactArgs } from "vipvot";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { getDatabaseStats } from "../../mongo/databases.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildStatsCommand(): Command {
  return Command({
    use: "stats <database>",
    short: "Get database statistics",
    args: ExactArgs(1),
    run: async (_cmd, args) => {
      const [database] = args as [string];
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());
        const result = await getDatabaseStats(client, database);
        printJson(result);
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to get database stats");
      } finally {
        await closeAllClients();
      }
    },
  });
}

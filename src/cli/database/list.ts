import { Command, NoArgs } from "vipvot";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { listDatabases } from "../../mongo/databases.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildListCommand(): Command {
  return Command({
    use: "list",
    short: "List all databases",
    args: NoArgs,
    run: async () => {
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());
        const result = await listDatabases(client);
        printJson(result);
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to list databases");
      } finally {
        await closeAllClients();
      }
    },
  });
}

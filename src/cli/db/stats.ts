import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { getDatabaseStats } from "../../mongo/databases.ts";

export function registerStats(db: Command): void {
  db.command("stats")
    .description("Get database statistics")
    .argument("<database>", "Database name")
    .action(async (database: string, _opts: unknown, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection;
        const { client } = await getMongoClient(alias);
        const result = await getDatabaseStats(client, database);
        printJson(result);
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to get database stats");
      } finally {
        await closeAllClients();
      }
    });
}

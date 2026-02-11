import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { countDocuments } from "../../mongo/query.ts";

export function registerCount(parent: Command): void {
  parent
    .command("count")
    .description("Count documents matching a filter")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .option("--filter <json>", "MongoDB query filter (JSON)")
    .action(
      async (database: string, collection: string, opts: { filter?: string }, command: Command) => {
        try {
          const alias = command.optsWithGlobals().connection;
          const { client } = await getMongoClient(alias);
          const filter = opts.filter ? parseJson(opts.filter) : undefined;
          const count = await countDocuments(client, database, collection, filter);
          printJson({ database, collection, filter: filter ?? {}, count });
        } catch (err) {
          printError(err instanceof Error ? err.message : "Failed to count documents");
        } finally {
          await closeAllClients();
        }
      },
    );
}

function parseJson(value: string): Record<string, unknown> {
  try {
    return JSON.parse(value) as Record<string, unknown>;
  } catch {
    throw new Error(`Invalid JSON for --filter: ${value}`);
  }
}

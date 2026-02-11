import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { getDistinctValues } from "../../mongo/query.ts";

export function registerDistinct(parent: Command): void {
  parent
    .command("distinct")
    .description("Get distinct values for a field")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .argument("<field>", "Field path (supports dot notation: address.city)")
    .option("--filter <json>", "MongoDB query filter (JSON)")
    .action(
      async (
        database: string,
        collection: string,
        field: string,
        opts: { filter?: string },
        command: Command,
      ) => {
        try {
          const alias = command.optsWithGlobals().connection;
          const { client } = await getMongoClient(alias);
          const filter = opts.filter ? parseJson(opts.filter) : undefined;
          const values = await getDistinctValues(client, database, collection, field, filter);
          printJson({ database, collection, field, values, count: values.length });
        } catch (err) {
          printError(err instanceof Error ? err.message : "Failed to get distinct values");
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

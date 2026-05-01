import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { getDistinctValues } from "../../mongo/query.ts";
import { enhanceErrorMessage } from "../../lib/errors.ts";
import { parseJson } from "../../lib/parse-json.ts";

export function registerDistinct(parent: Command): void {
  parent
    .command("distinct")
    .description("Get distinct values for a field")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .argument("<field>", "Field path (supports dot notation: address.city)")
    .option("--filter <json>", "MongoDB query filter (JSON)")
    .action(
      // oxlint-disable-next-line max-params -- commander dictates this signature
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
          const filter = opts.filter ? parseJson(opts.filter, "filter") : undefined;
          const values = await getDistinctValues(client, {
            dbName: database,
            collName: collection,
            field,
            filter,
          });
          printJson({ database, collection, field, values, count: values.length });
        } catch (err) {
          printError(
            err instanceof Error
              ? enhanceErrorMessage(err, { database, collection })
              : "Failed to get distinct values",
          );
        } finally {
          await closeAllClients();
        }
      },
    );
}

import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { findById } from "../../mongo/query.ts";
import { enhanceErrorMessage } from "../../lib/errors.ts";

export function registerGet(parent: Command): void {
  parent
    .command("get")
    .description("Get a single document by _id")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .argument("<id>", "Document _id value")
    .option("--type <type>", "Force ID type: objectid, string, number (auto-detected by default)")
    .option("--projection <json>", 'Field projection (e.g. {"name": 1, "email": 1})')
    .action(
      async (
        database: string,
        collection: string,
        id: string,
        opts: { type?: string; projection?: string },
        command: Command,
      ) => {
        try {
          if (opts.type && !["objectid", "string", "number"].includes(opts.type)) {
            throw new Error(`Invalid --type: "${opts.type}". Valid: objectid, string, number`);
          }

          const alias = command.optsWithGlobals().connection;
          const { client } = await getMongoClient(alias);
          const projection = opts.projection ? parseJson(opts.projection) : undefined;
          const doc = await findById(client, database, collection, id, opts.type, projection);

          if (!doc) {
            throw new Error(`Document not found: _id=${id} in ${database}.${collection}`);
          }

          printJson({
            database,
            collection,
            fieldCount: Object.keys(doc).length,
            document: doc,
          });
        } catch (err) {
          printError(
            err instanceof Error
              ? enhanceErrorMessage(err, { database, collection })
              : "Failed to get document",
          );
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
    throw new Error(`Invalid JSON for --projection: ${value}`);
  }
}

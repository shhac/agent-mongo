import type { Command } from "commander";
import type { Sort } from "mongodb";
import { printJson, printError, resolvePageSize } from "../../lib/output.ts";
import { getSettings } from "../../lib/config.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { findDocuments } from "../../mongo/query.ts";

type FindOpts = {
  filter?: string;
  sort?: string;
  projection?: string;
  limit?: string;
  skip?: string;
};

export function registerFind(parent: Command): void {
  parent
    .command("find")
    .description("Find documents matching a filter")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .option("--filter <json>", "MongoDB query filter (JSON)")
    .option("--sort <json>", 'Sort specification (e.g. {"createdAt": -1})')
    .option("--projection <json>", 'Field projection (e.g. {"name": 1, "email": 1})')
    .option("--limit <n>", "Max documents to return")
    .option("--skip <n>", "Number of documents to skip", "0")
    .action(async (database: string, collection: string, opts: FindOpts, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection;
        const { client } = await getMongoClient(alias);

        const maxDocs = getSettings().query?.maxDocuments ?? 100;
        const requestedLimit = resolvePageSize(opts);
        const limit = Math.min(requestedLimit, maxDocs);
        const skip = parseInt(opts.skip ?? "0", 10);

        const result = await findDocuments(client, database, collection, {
          filter: opts.filter ? parseJson(opts.filter, "filter") : undefined,
          sort: (opts.sort ? parseJson(opts.sort, "sort") : { _id: -1 }) as Sort,
          projection: opts.projection ? parseJson(opts.projection, "projection") : undefined,
          limit,
          skip,
        });

        printJson(result);
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to find documents");
      } finally {
        await closeAllClients();
      }
    });
}

function parseJson(value: string, name: string): Record<string, unknown> {
  try {
    return JSON.parse(value) as Record<string, unknown>;
  } catch {
    throw new Error(`Invalid JSON for --${name}: ${value}`);
  }
}

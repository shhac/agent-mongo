import type { Command } from "commander";
import type { Sort } from "mongodb";
import { printJson, printError, printNdjsonStream, resolvePageSize } from "../../lib/output.ts";
import { getSettings } from "../../lib/config.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { findDocuments, streamFind } from "../../mongo/query.ts";
import { enhanceErrorMessage } from "../../lib/errors.ts";
import { parseJson } from "../../lib/parse-json.ts";

type FindOpts = {
  filter?: string;
  sort?: string;
  projection?: string;
  limit?: string;
  skip?: string;
  stream?: boolean;
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
    .option("--stream", "Stream results as NDJSON (one JSON object per line, no limit)")
    // oxlint-disable-next-line max-params -- commander dictates this signature
    .action(async (database: string, collection: string, opts: FindOpts, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection;
        const { client } = await getMongoClient(alias);

        const filter = opts.filter ? parseJson(opts.filter, "filter") : undefined;
        const sort = (opts.sort ? parseJson(opts.sort, "sort") : { _id: -1 }) as Sort;
        const projection = opts.projection ? parseJson(opts.projection, "projection") : undefined;

        if (opts.stream) {
          const cursor = streamFind(client, {
            dbName: database,
            collName: collection,
            filter,
            sort,
            projection,
          });
          await printNdjsonStream(cursor);
        } else {
          const maxDocs = getSettings().query?.maxDocuments ?? 100;
          const requestedLimit = resolvePageSize(opts);
          const limit = Math.min(requestedLimit, maxDocs);
          const skip = parseInt(opts.skip ?? "0", 10);

          const result = await findDocuments(client, {
            dbName: database,
            collName: collection,
            filter,
            sort,
            projection,
            limit,
            skip,
          });

          printJson(result);
        }
      } catch (err) {
        printError(
          err instanceof Error
            ? enhanceErrorMessage(err, { database, collection })
            : "Failed to find documents",
        );
      } finally {
        await closeAllClients();
      }
    });
}

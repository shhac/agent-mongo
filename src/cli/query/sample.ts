import type { Command } from "commander";
import type { Document } from "mongodb";
import { printJson, printError } from "../../lib/output.ts";
import { getSettings } from "../../lib/config.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { serializeDocuments } from "../../mongo/serialize.ts";

export function registerSample(parent: Command): void {
  parent
    .command("sample")
    .description("Get random sample of documents")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .option("--size <n>", "Number of random documents")
    .action(
      async (database: string, collection: string, opts: { size?: string }, command: Command) => {
        try {
          const alias = command.optsWithGlobals().connection;
          const { client } = await getMongoClient(alias);

          const defaultSize = getSettings().defaults?.sampleSize ?? 5;
          const maxDocs = getSettings().query?.maxDocuments ?? 100;
          const requestedSize = opts.size ? parseInt(opts.size, 10) : defaultSize;
          if (!Number.isFinite(requestedSize) || requestedSize < 1) {
            throw new Error(`Invalid --size: "${opts.size}". Must be a positive integer.`);
          }
          const size = Math.min(requestedSize, maxDocs);

          const timeout = getSettings().query?.timeout ?? 30000;
          const coll = client.db(database).collection(collection);
          const docs = await coll
            .aggregate<Document>([{ $sample: { size } }], { maxTimeMS: timeout })
            .toArray();

          printJson({
            database,
            collection,
            sampleSize: docs.length,
            documents: serializeDocuments(docs),
          });
        } catch (err) {
          printError(err instanceof Error ? err.message : "Failed to sample documents");
        } finally {
          await closeAllClients();
        }
      },
    );
}

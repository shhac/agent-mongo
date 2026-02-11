import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { inferSchema } from "../../mongo/schema.ts";

const DEFAULT_SAMPLE_SIZE = 100;

export function registerSchema(parent: Command): void {
  parent
    .command("schema")
    .description("Infer collection schema by sampling documents")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .option("--sample-size <n>", "Number of documents to sample", String(DEFAULT_SAMPLE_SIZE))
    .action(
      async (
        database: string,
        collection: string,
        opts: { sampleSize: string },
        command: Command,
      ) => {
        try {
          const alias = command.optsWithGlobals().connection;
          const sampleSize = parseInt(opts.sampleSize, 10);
          if (!Number.isFinite(sampleSize) || sampleSize < 1) {
            throw new Error(
              `Invalid --sample-size: "${opts.sampleSize}". Must be a positive integer.`,
            );
          }
          const { client } = await getMongoClient(alias);
          const result = await inferSchema(client, database, collection, sampleSize);
          printJson(result);
        } catch (err) {
          printError(err instanceof Error ? err.message : "Failed to infer schema");
        } finally {
          await closeAllClients();
        }
      },
    );
}

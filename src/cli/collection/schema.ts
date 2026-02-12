import type { Command } from "commander";
import { printJson, printError } from "../../lib/output.ts";
import { getSettings } from "../../lib/config.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { inferSchema } from "../../mongo/schema.ts";
import type { SchemaResult } from "../../mongo/schema.ts";

type SchemaOpts = {
  sampleSize?: string;
  depth?: string;
  limit?: string;
  skip?: string;
};

export function registerSchema(parent: Command): void {
  parent
    .command("schema")
    .description("Infer collection schema by sampling documents")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .option("--sample-size <n>", "Number of documents to sample")
    .option("--depth <n>", "Max nesting depth for fields (1 = top-level only)")
    .option("--limit <n>", "Max fields to return (for pagination)")
    .option("--skip <n>", "Number of fields to skip (for pagination)")
    .action(
      async (
        database: string,
        collection: string,
        opts: SchemaOpts,
        command: Command,
      ) => {
        try {
          const alias = command.optsWithGlobals().connection;
          const defaultSize = getSettings().defaults?.schemaSampleSize ?? 100;
          const sampleSize = opts.sampleSize ? parseInt(opts.sampleSize, 10) : defaultSize;
          if (!Number.isFinite(sampleSize) || sampleSize < 1) {
            throw new Error(
              `Invalid --sample-size: "${opts.sampleSize}". Must be a positive integer.`,
            );
          }

          const maxDepth = parsePositiveInt(opts.depth, "--depth");
          const limit = parsePositiveInt(opts.limit, "--limit");
          const skip = opts.skip !== undefined ? parseNonNegativeInt(opts.skip, "--skip") : 0;

          const { client } = await getMongoClient(alias);
          const result = await inferSchema(client, database, collection, sampleSize, maxDepth);

          printSchemaResult(result, limit, skip);
        } catch (err) {
          printError(err instanceof Error ? err.message : "Failed to infer schema");
        } finally {
          await closeAllClients();
        }
      },
    );
}

function printSchemaResult(result: SchemaResult, limit: number | undefined, skip: number): void {
  const totalFields = result.fields.length;
  const slicedFields = limit !== undefined
    ? result.fields.slice(skip, skip + limit)
    : result.fields.slice(skip);
  const hasMore = skip + slicedFields.length < totalFields;

  const output: Record<string, unknown> = {
    database: result.database,
    collection: result.collection,
    sampleSize: result.sampleSize,
    totalDocuments: result.totalDocuments,
    totalFields,
    fields: slicedFields,
  };

  if (hasMore) {
    output.pagination = {
      hasMore: true,
      nextSkip: skip + slicedFields.length,
    };
  }

  printJson(output);
}

function parsePositiveInt(value: string | undefined, name: string): number | undefined {
  if (value === undefined) {
    return undefined;
  }
  const num = parseInt(value, 10);
  if (!Number.isFinite(num) || num < 1) {
    throw new Error(`Invalid ${name}: "${value}". Must be a positive integer.`);
  }
  return num;
}

function parseNonNegativeInt(value: string, name: string): number {
  const num = parseInt(value, 10);
  if (!Number.isFinite(num) || num < 0) {
    throw new Error(`Invalid ${name}: "${value}". Must be a non-negative integer.`);
  }
  return num;
}

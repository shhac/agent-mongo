import { Command, ExactArgs, ref } from "vipvot";
import { printJson, printError } from "../../lib/output.ts";
import { getSettings } from "../../lib/config.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { inferSchema } from "../../mongo/schema.ts";
import type { SchemaResult } from "../../mongo/schema.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildSchemaCommand(): Command {
  const sampleSize = ref<string>("");
  const depth = ref<string>("");
  const limit = ref<string>("");
  const skip = ref<string>("");

  const cmd = Command({
    use: "schema <database> <collection>",
    short: "Infer collection schema by sampling documents",
    args: ExactArgs(2),
    run: async (_cmd, args) => {
      const [database, collection] = args as [string, string];
      try {
        const defaultSize = getSettings().defaults?.schemaSampleSize ?? 100;
        const sample = sampleSize.value ? parseInt(sampleSize.value, 10) : defaultSize;
        if (!Number.isFinite(sample) || sample < 1) {
          throw new Error(
            `Invalid --sample-size: "${sampleSize.value}". Must be a positive integer.`,
          );
        }

        const maxDepth = parsePositiveInt(depth.value || undefined, "--depth");
        const limitVal = parsePositiveInt(limit.value || undefined, "--limit");
        const skipVal = skip.value ? parseNonNegativeInt(skip.value, "--skip") : 0;

        const { client } = await getMongoClient(resolveConnectionAlias());
        const result = await inferSchema(client, {
          dbName: database,
          collName: collection,
          sampleSize: sample,
          maxDepth,
        });

        printSchemaResult(result, { limit: limitVal, skip: skipVal });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to infer schema");
      } finally {
        await closeAllClients();
      }
    },
  });

  cmd.flags().stringVarP(sampleSize, "sample-size", "", "", "Number of documents to sample");
  cmd
    .flags()
    .stringVarP(depth, "depth", "", "", "Max nesting depth for fields (1 = top-level only)");
  cmd.flags().stringVarP(limit, "limit", "", "", "Max fields to return (for pagination)");
  cmd.flags().stringVarP(skip, "skip", "", "", "Number of fields to skip (for pagination)");

  return cmd;
}

function printSchemaResult(
  result: SchemaResult,
  { limit, skip }: { limit: number | undefined; skip: number },
): void {
  const totalFields = result.fields.length;
  const slicedFields =
    limit !== undefined ? result.fields.slice(skip, skip + limit) : result.fields.slice(skip);
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

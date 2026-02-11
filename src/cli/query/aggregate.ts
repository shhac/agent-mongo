import type { Command } from "commander";
import type { Document } from "mongodb";
import { printJson, printError, resolvePageSize } from "../../lib/output.ts";
import { getSettings } from "../../lib/config.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { runAggregate } from "../../mongo/aggregate.ts";

type AggregateOpts = {
  pipeline?: string;
  limit?: string;
};

export function registerAggregate(parent: Command): void {
  parent
    .command("aggregate")
    .description("Run a read-only aggregation pipeline")
    .argument("<database>", "Database name")
    .argument("<collection>", "Collection name")
    .option("--pipeline <json>", "Aggregation pipeline as JSON array (or pipe via stdin)")
    .option("--limit <n>", "Max results if pipeline has no $limit stage")
    .action(async (database: string, collection: string, opts: AggregateOpts, command: Command) => {
      try {
        const alias = command.optsWithGlobals().connection;
        const { client } = await getMongoClient(alias);

        const pipeline = await resolvePipeline(opts.pipeline);
        const maxDocs = getSettings().query?.maxDocuments ?? 100;
        const requestedLimit = resolvePageSize(opts);
        const limit = Math.min(requestedLimit, maxDocs);

        const result = await runAggregate(client, database, collection, pipeline, limit);
        printJson({ database, collection, ...result });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to run aggregation");
      } finally {
        await closeAllClients();
      }
    });
}

async function resolvePipeline(pipelineFlag?: string): Promise<Document[]> {
  let raw: string;

  if (pipelineFlag) {
    raw = pipelineFlag;
  } else if (!process.stdin.isTTY) {
    raw = await readStdin();
  } else {
    throw new Error("Provide --pipeline <json> or pipe a JSON array via stdin.");
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    throw new Error(`Invalid JSON pipeline: ${raw.slice(0, 100)}${raw.length > 100 ? "..." : ""}`);
  }

  if (!Array.isArray(parsed)) {
    throw new Error("Pipeline must be a JSON array of stage objects.");
  }

  return parsed as Document[];
}

async function readStdin(): Promise<string> {
  const chunks: string[] = [];
  process.stdin.setEncoding("utf8");
  for await (const chunk of process.stdin) {
    chunks.push(chunk as string);
  }
  const result = chunks.join("").trim();
  if (!result) {
    throw new Error("Empty stdin. Provide --pipeline <json> or pipe a JSON array via stdin.");
  }
  return result;
}

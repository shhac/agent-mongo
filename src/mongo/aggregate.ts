import type { MongoClient, Document } from "mongodb";
import { getTimeout } from "../lib/timeout.ts";
import { serializeDocuments } from "./serialize.ts";

const WRITE_STAGES = new Set(["$out", "$merge"]);

export function validatePipeline(pipeline: Document[]): void {
  for (const stage of pipeline) {
    if (typeof stage !== "object" || stage === null) {
      continue;
    }
    for (const key of Object.keys(stage)) {
      if (WRITE_STAGES.has(key)) {
        throw new Error(`Write stage "${key}" is not allowed. agent-mongo is read-only.`);
      }
    }
  }
}

type AggregateOpts = {
  dbName: string;
  collName: string;
  pipeline: Document[];
  limit: number;
};

export async function runAggregate(
  client: MongoClient,
  opts: AggregateOpts,
): Promise<{ documents: Record<string, unknown>[]; count: number }> {
  validatePipeline(opts.pipeline);

  const hasLimitStage = opts.pipeline.some((s) => "$limit" in s);
  const effectivePipeline = hasLimitStage ? opts.pipeline : [...opts.pipeline, { $limit: opts.limit }];

  const timeout = getTimeout();
  const collection = client.db(opts.dbName).collection(opts.collName);
  const docs = await collection.aggregate(effectivePipeline, { maxTimeMS: timeout }).toArray();

  return {
    documents: serializeDocuments(docs),
    count: docs.length,
  };
}

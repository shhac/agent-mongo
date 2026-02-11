import type { MongoClient, Document } from "mongodb";
import { getSettings } from "../lib/config.ts";
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

export async function runAggregate(
  client: MongoClient,
  dbName: string,
  collName: string,
  pipeline: Document[],
  limit: number,
): Promise<{ documents: Record<string, unknown>[]; count: number }> {
  validatePipeline(pipeline);

  const hasLimitStage = pipeline.some((s) => "$limit" in s);
  const effectivePipeline = hasLimitStage ? pipeline : [...pipeline, { $limit: limit }];

  const timeout = getSettings().query?.timeout ?? 30000;
  const collection = client.db(dbName).collection(collName);
  const docs = await collection.aggregate(effectivePipeline, { maxTimeMS: timeout }).toArray();

  return {
    documents: serializeDocuments(docs),
    count: docs.length,
  };
}

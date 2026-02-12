import type { MongoClient } from "mongodb";
import { getSettings } from "../lib/config.ts";

export async function validateCollectionExists(
  client: MongoClient,
  dbName: string,
  collName: string,
): Promise<void> {
  const db = client.db(dbName);
  const matches = await db.listCollections({ name: collName }).toArray();
  if (matches.length === 0) {
    throw new Error(
      `Collection "${collName}" not found in database "${dbName}". Use 'collection list ${dbName}' to see available collections.`,
    );
  }
}

export async function listCollections(
  client: MongoClient,
  dbName: string,
): Promise<{ name: string; type: string }[]> {
  const db = client.db(dbName);
  const collections = await db.listCollections({}, { nameOnly: false }).toArray();

  return collections.map((c) => ({
    name: c.name,
    type: c.type ?? "collection",
  }));
}

export async function getCollectionStats(
  client: MongoClient,
  dbName: string,
  collName: string,
): Promise<Record<string, unknown>> {
  const timeout = getSettings().query?.timeout ?? 30000;
  const db = client.db(dbName);
  const result = await db.command({ collStats: collName, maxTimeMS: timeout });

  return {
    database: dbName,
    collection: collName,
    documentCount: result.count,
    dataSize: result.size,
    avgDocumentSize: result.avgObjSize,
    storageSize: result.storageSize,
    indexes: result.nindexes,
    indexSize: result.totalIndexSize,
    capped: result.capped ?? false,
  };
}

import type { MongoClient } from "mongodb";
import { getTimeout } from "../lib/timeout.ts";

type CollectionRef = {
  dbName: string;
  collName: string;
};

export async function validateCollectionExists(
  client: MongoClient,
  { dbName, collName }: CollectionRef,
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
  { dbName, collName }: CollectionRef,
): Promise<Record<string, unknown>> {
  const timeout = getTimeout();
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

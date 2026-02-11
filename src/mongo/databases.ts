import type { MongoClient } from "mongodb";
import { getSettings } from "../lib/config.ts";

export async function listDatabases(client: MongoClient): Promise<{
  databases: { name: string; sizeOnDisk: number; empty: boolean }[];
  totalSize: number;
}> {
  const timeout = getSettings().query?.timeout ?? 30000;
  const result = await client.db("admin").admin().listDatabases({ maxTimeMS: timeout });

  const databases = result.databases.map((db) => ({
    name: db.name,
    sizeOnDisk: db.sizeOnDisk ?? 0,
    empty: db.empty ?? false,
  }));

  return { databases, totalSize: result.totalSize ?? 0 };
}

export async function getDatabaseStats(
  client: MongoClient,
  dbName: string,
): Promise<Record<string, unknown>> {
  const timeout = getSettings().query?.timeout ?? 30000;
  const result = await client.db(dbName).command({ dbStats: 1, maxTimeMS: timeout });

  return {
    database: dbName,
    collections: result.collections,
    documents: result.objects,
    dataSize: result.dataSize,
    storageSize: result.storageSize,
    indexes: result.indexes,
    indexSize: result.indexSize,
  };
}

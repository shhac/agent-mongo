import type { MongoClient } from "mongodb";

export async function listIndexes(
  client: MongoClient,
  dbName: string,
  collName: string,
): Promise<Record<string, unknown>[]> {
  const collection = client.db(dbName).collection(collName);
  const indexes = await collection.indexes();

  return indexes.map((idx) => {
    const info: Record<string, unknown> = {
      name: idx.name,
      key: idx.key,
    };
    if (idx.unique) {
      info.unique = true;
    }
    if (idx.sparse) {
      info.sparse = true;
    }
    if (idx.expireAfterSeconds !== undefined) {
      info.expireAfterSeconds = idx.expireAfterSeconds;
    }
    if (idx.partialFilterExpression) {
      info.partialFilterExpression = idx.partialFilterExpression;
    }
    return info;
  });
}

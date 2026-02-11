import { ObjectId } from "mongodb";
import type { MongoClient, Document, Filter, Sort } from "mongodb";
import { getSettings } from "../lib/config.ts";
import { serializeDocuments, serializeDocument } from "./serialize.ts";

type FindOptions = {
  filter?: Document;
  sort?: Sort;
  projection?: Document;
  limit: number;
  skip: number;
};

type FindResult = {
  database: string;
  collection: string;
  filter: Document;
  documents: Record<string, unknown>[];
  count: number;
  hasMore: boolean;
  totalMatching: number;
};

export async function findDocuments(
  client: MongoClient,
  dbName: string,
  collName: string,
  opts: FindOptions,
): Promise<FindResult> {
  const timeout = getSettings().query?.timeout ?? 30000;
  const collection = client.db(dbName).collection(collName);
  const filter = (opts.filter ?? {}) as Filter<Document>;

  const cursor = collection.find(filter, { maxTimeMS: timeout });
  if (opts.sort) {
    cursor.sort(opts.sort);
  }
  if (opts.projection) {
    cursor.project(opts.projection);
  }
  cursor.skip(opts.skip);
  cursor.limit(opts.limit + 1);

  const rawDocs = await cursor.toArray();
  const hasMore = rawDocs.length > opts.limit;
  const docs = hasMore ? rawDocs.slice(0, opts.limit) : rawDocs;

  const isEmptyFilter = Object.keys(filter).length === 0;
  const totalMatching = isEmptyFilter
    ? await collection.estimatedDocumentCount({ maxTimeMS: timeout })
    : await collection.countDocuments(filter, { maxTimeMS: timeout });

  return {
    database: dbName,
    collection: collName,
    filter,
    documents: serializeDocuments(docs),
    count: docs.length,
    hasMore,
    totalMatching,
  };
}

export async function findById(
  client: MongoClient,
  dbName: string,
  collName: string,
  rawId: string,
  idType?: string,
): Promise<Record<string, unknown> | null> {
  const timeout = getSettings().query?.timeout ?? 30000;
  const collection = client.db(dbName).collection(collName);
  const id = parseId(rawId, idType);
  const doc = await collection.findOne({ _id: id } as Filter<Document>, { maxTimeMS: timeout });
  return doc ? serializeDocument(doc) : null;
}

export async function countDocuments(
  client: MongoClient,
  dbName: string,
  collName: string,
  filter?: Document,
): Promise<number> {
  const timeout = getSettings().query?.timeout ?? 30000;
  const collection = client.db(dbName).collection(collName);
  const f = (filter ?? {}) as Filter<Document>;
  const isEmpty = Object.keys(f).length === 0;
  return isEmpty
    ? collection.estimatedDocumentCount({ maxTimeMS: timeout })
    : collection.countDocuments(f, { maxTimeMS: timeout });
}

export async function getDistinctValues(
  client: MongoClient,
  dbName: string,
  collName: string,
  field: string,
  filter?: Document,
): Promise<unknown[]> {
  const timeout = getSettings().query?.timeout ?? 30000;
  const collection = client.db(dbName).collection(collName);
  const f = (filter ?? {}) as Filter<Document>;
  const values = await collection.distinct(field, f, { maxTimeMS: timeout });
  return values.map((v) => {
    if (v instanceof ObjectId) {
      return v.toHexString();
    }
    if (v instanceof Date) {
      return v.toISOString();
    }
    return v;
  });
}

function parseId(raw: string, type?: string): unknown {
  if (type === "objectid" || (!type && /^[0-9a-fA-F]{24}$/.test(raw))) {
    return new ObjectId(raw);
  }
  if (type === "number") {
    const n = Number(raw);
    if (!Number.isFinite(n)) {
      throw new Error(`Invalid number ID: "${raw}"`);
    }
    return n;
  }
  return raw;
}

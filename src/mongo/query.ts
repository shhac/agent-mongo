import { ObjectId } from "mongodb";
import type { MongoClient, Document, Filter, Sort } from "mongodb";
import { getTimeout } from "../lib/timeout.ts";
import { serializeDocuments, serializeDocument } from "./serialize.ts";

type FindOpts = {
  dbName: string;
  collName: string;
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

export async function findDocuments(client: MongoClient, opts: FindOpts): Promise<FindResult> {
  const timeout = getTimeout();
  const collection = client.db(opts.dbName).collection(opts.collName);
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
    database: opts.dbName,
    collection: opts.collName,
    filter,
    documents: serializeDocuments(docs),
    count: docs.length,
    hasMore,
    totalMatching,
  };
}

type FindByIdOpts = {
  dbName: string;
  collName: string;
  rawId: string;
  idType?: string;
  projection?: Document;
};

export async function findById(
  client: MongoClient,
  opts: FindByIdOpts,
): Promise<Record<string, unknown> | null> {
  const timeout = getTimeout();
  const collection = client.db(opts.dbName).collection(opts.collName);
  const id = parseId(opts.rawId, opts.idType);
  const doc = await collection.findOne({ _id: id } as Filter<Document>, {
    maxTimeMS: timeout,
    projection: opts.projection,
  });
  return doc ? serializeDocument(doc) : null;
}

type CountOpts = {
  dbName: string;
  collName: string;
  filter?: Document;
};

export async function countDocuments(client: MongoClient, opts: CountOpts): Promise<number> {
  const timeout = getTimeout();
  const collection = client.db(opts.dbName).collection(opts.collName);
  const f = (opts.filter ?? {}) as Filter<Document>;
  const isEmpty = Object.keys(f).length === 0;
  return isEmpty
    ? collection.estimatedDocumentCount({ maxTimeMS: timeout })
    : collection.countDocuments(f, { maxTimeMS: timeout });
}

type DistinctOpts = {
  dbName: string;
  collName: string;
  field: string;
  filter?: Document;
};

export async function getDistinctValues(client: MongoClient, opts: DistinctOpts): Promise<unknown[]> {
  const timeout = getTimeout();
  const collection = client.db(opts.dbName).collection(opts.collName);
  const f = (opts.filter ?? {}) as Filter<Document>;
  const values = await collection.distinct(opts.field, f, { maxTimeMS: timeout });
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

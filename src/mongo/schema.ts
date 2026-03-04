import { ObjectId, Binary, Long, Decimal128, UUID } from "mongodb";
import type { MongoClient, Document } from "mongodb";
import { getTimeout } from "../lib/timeout.ts";
import { validateCollectionExists } from "./collections.ts";

type FieldInfo = {
  path: string;
  types: string[];
  presence: number;
};

export type SchemaResult = {
  database: string;
  collection: string;
  sampleSize: number;
  totalDocuments: number;
  fields: FieldInfo[];
};

type SchemaOpts = {
  dbName: string;
  collName: string;
  sampleSize: number;
  maxDepth?: number;
};

export async function inferSchema(client: MongoClient, opts: SchemaOpts): Promise<SchemaResult> {
  await validateCollectionExists(client, { dbName: opts.dbName, collName: opts.collName });

  const timeout = getTimeout();
  const collection = client.db(opts.dbName).collection(opts.collName);

  const totalDocuments = await collection.estimatedDocumentCount({ maxTimeMS: timeout });

  const effectiveSize = Math.min(opts.sampleSize, totalDocuments || opts.sampleSize);
  const docs = await collection
    .aggregate<Document>([{ $sample: { size: effectiveSize } }], { maxTimeMS: timeout })
    .toArray();

  const fieldMap = new Map<string, { types: Set<string>; count: number }>();

  for (const doc of docs) {
    const seen = new Set<string>();
    walkDocument({ doc, prefix: "", seen, fields: fieldMap, depth: 1, maxDepth: opts.maxDepth });
  }

  const fields: FieldInfo[] = Array.from(fieldMap.entries())
    .map(([path, info]) => ({
      path,
      types: Array.from(info.types).sort(),
      presence: docs.length > 0 ? Math.round((info.count / docs.length) * 100) / 100 : 0,
    }))
    .sort((a, b) => a.path.localeCompare(b.path));

  return {
    database: opts.dbName,
    collection: opts.collName,
    sampleSize: docs.length,
    totalDocuments,
    fields,
  };
}

type WalkOpts = {
  doc: Document;
  prefix: string;
  seen: Set<string>;
  fields: Map<string, { types: Set<string>; count: number }>;
  depth: number;
  maxDepth?: number;
};

function walkDocument({ doc, prefix, seen, fields, depth, maxDepth }: WalkOpts): void {
  for (const [key, value] of Object.entries(doc)) {
    const path = prefix ? `${prefix}.${key}` : key;
    recordField({ path, typeName: getTypeName(value), seen, fields });

    if (maxDepth !== undefined && depth >= maxDepth) {
      continue;
    }

    if (isPlainObject(value)) {
      walkDocument({ doc: value as Document, prefix: path, seen, fields, depth: depth + 1, maxDepth });
    } else if (Array.isArray(value)) {
      walkArrayElements({ arr: value, parentPath: path, seen, fields, depth, maxDepth });
    }
  }
}

type WalkArrayOpts = {
  arr: unknown[];
  parentPath: string;
  seen: Set<string>;
  fields: Map<string, { types: Set<string>; count: number }>;
  depth: number;
  maxDepth?: number;
};

function walkArrayElements({ arr, parentPath, seen, fields, depth, maxDepth }: WalkArrayOpts): void {
  const elemPath = `${parentPath}.$`;
  for (const elem of arr) {
    recordFieldType({ path: elemPath, typeName: getTypeName(elem), fields });

    if (maxDepth === undefined || depth < maxDepth) {
      if (isPlainObject(elem)) {
        walkDocument({
          doc: elem as Document,
          prefix: elemPath,
          seen,
          fields,
          depth: depth + 1,
          maxDepth,
        });
      }
    }
  }

  if (arr.length > 0 && !seen.has(elemPath)) {
    seen.add(elemPath);
    const field = fields.get(elemPath);
    if (field) {
      field.count++;
    }
  }
}

type RecordFieldOpts = {
  path: string;
  typeName: string;
  seen: Set<string>;
  fields: Map<string, { types: Set<string>; count: number }>;
};

function recordField({ path, typeName, seen, fields }: RecordFieldOpts): void {
  recordFieldType({ path, typeName, fields });
  if (!seen.has(path)) {
    seen.add(path);
    const field = fields.get(path);
    if (field) {
      field.count++;
    }
  }
}

type RecordFieldTypeOpts = {
  path: string;
  typeName: string;
  fields: Map<string, { types: Set<string>; count: number }>;
};

function recordFieldType({ path, typeName, fields }: RecordFieldTypeOpts): void {
  let field = fields.get(path);
  if (!field) {
    field = { types: new Set(), count: 0 };
    fields.set(path, field);
  }
  field.types.add(typeName);
}

function getTypeName(value: unknown): string {
  if (value === null) {
    return "null";
  }
  if (value === undefined) {
    return "null";
  }
  if (value instanceof ObjectId) {
    return "ObjectId";
  }
  if (value instanceof Date) {
    return "date";
  }
  if (value instanceof Binary) {
    return "binary";
  }
  if (value instanceof Long) {
    return "long";
  }
  if (value instanceof Decimal128) {
    return "decimal";
  }
  if (value instanceof UUID) {
    return "uuid";
  }
  if (value instanceof RegExp) {
    return "regex";
  }
  if (Array.isArray(value)) {
    return "array";
  }
  if (typeof value === "string") {
    return "string";
  }
  if (typeof value === "number") {
    return Number.isInteger(value) ? "int" : "double";
  }
  if (typeof value === "boolean") {
    return "boolean";
  }
  if (typeof value === "object") {
    return "object";
  }
  return typeof value;
}

function isPlainObject(value: unknown): boolean {
  if (value === null || value === undefined) {
    return false;
  }
  if (typeof value !== "object") {
    return false;
  }
  if (Array.isArray(value)) {
    return false;
  }
  if (value instanceof ObjectId) {
    return false;
  }
  if (value instanceof Date) {
    return false;
  }
  if (value instanceof Binary) {
    return false;
  }
  if (value instanceof Long) {
    return false;
  }
  if (value instanceof Decimal128) {
    return false;
  }
  if (value instanceof UUID) {
    return false;
  }
  if (value instanceof RegExp) {
    return false;
  }
  return true;
}

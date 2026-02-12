import { ObjectId, Binary, Long, Decimal128, UUID } from "mongodb";
import type { MongoClient, Document } from "mongodb";
import { getSettings } from "../lib/config.ts";
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

export async function inferSchema(
  client: MongoClient,
  dbName: string,
  collName: string,
  sampleSize: number,
  maxDepth?: number,
): Promise<SchemaResult> {
  await validateCollectionExists(client, dbName, collName);

  const timeout = getSettings().query?.timeout ?? 30000;
  const collection = client.db(dbName).collection(collName);

  const totalDocuments = await collection.estimatedDocumentCount({ maxTimeMS: timeout });

  const effectiveSize = Math.min(sampleSize, totalDocuments || sampleSize);
  const docs = await collection
    .aggregate<Document>([{ $sample: { size: effectiveSize } }], { maxTimeMS: timeout })
    .toArray();

  const fieldMap = new Map<string, { types: Set<string>; count: number }>();

  for (const doc of docs) {
    const seen = new Set<string>();
    walkDocument(doc, "", seen, fieldMap, 1, maxDepth);
  }

  const fields: FieldInfo[] = Array.from(fieldMap.entries())
    .map(([path, info]) => ({
      path,
      types: Array.from(info.types).sort(),
      presence: docs.length > 0 ? Math.round((info.count / docs.length) * 100) / 100 : 0,
    }))
    .sort((a, b) => a.path.localeCompare(b.path));

  return {
    database: dbName,
    collection: collName,
    sampleSize: docs.length,
    totalDocuments,
    fields,
  };
}

function walkDocument(
  doc: Document,
  prefix: string,
  seen: Set<string>,
  fields: Map<string, { types: Set<string>; count: number }>,
  depth: number,
  maxDepth?: number,
): void {
  for (const [key, value] of Object.entries(doc)) {
    const path = prefix ? `${prefix}.${key}` : key;
    recordField(path, getTypeName(value), seen, fields);

    if (maxDepth !== undefined && depth >= maxDepth) {
      continue;
    }

    if (isPlainObject(value)) {
      walkDocument(value as Document, path, seen, fields, depth + 1, maxDepth);
    } else if (Array.isArray(value)) {
      walkArrayElements(value, path, seen, fields, depth, maxDepth);
    }
  }
}

function walkArrayElements(
  arr: unknown[],
  parentPath: string,
  seen: Set<string>,
  fields: Map<string, { types: Set<string>; count: number }>,
  depth: number,
  maxDepth?: number,
): void {
  const elemPath = `${parentPath}.$`;
  for (const elem of arr) {
    recordFieldType(elemPath, getTypeName(elem), fields);

    if (maxDepth === undefined || depth < maxDepth) {
      if (isPlainObject(elem)) {
        walkDocument(elem as Document, elemPath, seen, fields, depth + 1, maxDepth);
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

function recordField(
  path: string,
  typeName: string,
  seen: Set<string>,
  fields: Map<string, { types: Set<string>; count: number }>,
): void {
  recordFieldType(path, typeName, fields);
  if (!seen.has(path)) {
    seen.add(path);
    const field = fields.get(path);
    if (field) {
      field.count++;
    }
  }
}

function recordFieldType(
  path: string,
  typeName: string,
  fields: Map<string, { types: Set<string>; count: number }>,
): void {
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

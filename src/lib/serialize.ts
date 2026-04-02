import { ObjectId, Binary, Long, Decimal128, UUID } from "mongodb";

export function serialize(value: unknown): unknown {
  if (value === null || value === undefined) {
    return value;
  }

  if (value instanceof ObjectId) {
    return value.toHexString();
  }
  if (value instanceof Date) {
    return value.toISOString();
  }
  if (value instanceof Binary) {
    return Buffer.from(value.buffer).toString("base64");
  }
  if (value instanceof Long) {
    const num = value.toNumber();
    return Number.isSafeInteger(num) ? num : value.toString();
  }
  if (value instanceof Decimal128) {
    return value.toString();
  }
  if (value instanceof UUID) {
    return value.toString();
  }
  if (value instanceof RegExp) {
    return value.toString();
  }

  if (Array.isArray(value)) {
    return value.map(serialize);
  }

  if (typeof value === "object") {
    if ("_bsontype" in value) {
      return String(value);
    }

    const result: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value)) {
      result[k] = serialize(v);
    }
    return result;
  }

  return value;
}

export function serializeDocument(doc: Record<string, unknown>): Record<string, unknown> {
  return serialize(doc) as Record<string, unknown>;
}

export function serializeDocuments(docs: Record<string, unknown>[]): Record<string, unknown>[] {
  return docs.map(serializeDocument);
}

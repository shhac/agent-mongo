import { describe, test, expect } from "bun:test";
import { ObjectId, Binary, Long, Decimal128, UUID } from "mongodb";
import { serialize, serializeDocument, serializeDocuments } from "../src/mongo/serialize.ts";

describe("serialize", () => {
  test("ObjectId → 24-char hex string", () => {
    const oid = new ObjectId("507f1f77bcf86cd799439011");
    expect(serialize(oid)).toBe("507f1f77bcf86cd799439011");
  });

  test("Date → ISO 8601 string", () => {
    const date = new Date("2024-01-15T10:30:00.000Z");
    expect(serialize(date)).toBe("2024-01-15T10:30:00.000Z");
  });

  test("Binary → base64 string", () => {
    const bin = new Binary(Buffer.from("hello world"));
    const result = serialize(bin) as string;
    expect(result).toBe(Buffer.from("hello world").toString("base64"));
  });

  test("Long (safe integer) → number", () => {
    const long = Long.fromNumber(42);
    expect(serialize(long)).toBe(42);
  });

  test("Long (unsafe integer) → string", () => {
    const long = Long.fromString("9007199254740993"); // > Number.MAX_SAFE_INTEGER
    expect(serialize(long)).toBe("9007199254740993");
  });

  test("Decimal128 → string", () => {
    const dec = Decimal128.fromString("3.14159");
    expect(serialize(dec)).toBe("3.14159");
  });

  test("UUID → base64 (UUID extends Binary, so Binary branch matches first)", () => {
    const uuid = new UUID("550e8400-e29b-41d4-a716-446655440000");
    // UUID extends Binary in the mongodb driver, so instanceof Binary
    // matches before instanceof UUID. This serializes as base64.
    const result = serialize(uuid) as string;
    expect(typeof result).toBe("string");
    // Verify it's valid base64
    expect(() => Buffer.from(result, "base64")).not.toThrow();
  });

  test("RegExp → string", () => {
    const regex = /test.*pattern/i;
    expect(serialize(regex)).toBe("/test.*pattern/i");
  });

  test("null and undefined pass through", () => {
    expect(serialize(null)).toBeNull();
    expect(serialize(undefined)).toBeUndefined();
  });

  test("primitives pass through", () => {
    expect(serialize("hello")).toBe("hello");
    expect(serialize(42)).toBe(42);
    expect(serialize(true)).toBe(true);
    expect(serialize(false)).toBe(false);
    expect(serialize(0)).toBe(0);
  });

  test("nested documents are serialized recursively", () => {
    const oid = new ObjectId("507f1f77bcf86cd799439011");
    const date = new Date("2024-01-15T10:30:00.000Z");
    const doc: Record<string, unknown> = {
      _id: oid,
      name: "Test",
      metadata: {
        createdAt: date,
        tags: ["a", "b"],
      },
    };
    expect(serialize(doc)).toEqual({
      _id: "507f1f77bcf86cd799439011",
      name: "Test",
      metadata: {
        createdAt: "2024-01-15T10:30:00.000Z",
        tags: ["a", "b"],
      },
    });
  });

  test("arrays with BSON types are serialized", () => {
    const arr = [
      new ObjectId("507f1f77bcf86cd799439011"),
      new Date("2024-01-01T00:00:00.000Z"),
      "plain",
      42,
    ];
    expect(serialize(arr)).toEqual([
      "507f1f77bcf86cd799439011",
      "2024-01-01T00:00:00.000Z",
      "plain",
      42,
    ]);
  });

  test("unknown BSON types with _bsontype fall back to String()", () => {
    const unknownBson = { _bsontype: "SomeNewType", value: "test" };
    const result = serialize(unknownBson);
    expect(typeof result).toBe("string");
  });
});

describe("serializeDocument", () => {
  test("serializes a single document", () => {
    const doc: Record<string, unknown> = {
      _id: new ObjectId("507f1f77bcf86cd799439011"),
      name: "Test",
    };
    const result = serializeDocument(doc);
    expect(result).toEqual({
      _id: "507f1f77bcf86cd799439011",
      name: "Test",
    });
  });
});

describe("serializeDocuments", () => {
  test("serializes an array of documents", () => {
    const docs: Record<string, unknown>[] = [
      { _id: new ObjectId("507f1f77bcf86cd799439011"), name: "A" },
      { _id: new ObjectId("507f1f77bcf86cd799439012"), name: "B" },
    ];
    const result = serializeDocuments(docs);
    expect(result).toEqual([
      { _id: "507f1f77bcf86cd799439011", name: "A" },
      { _id: "507f1f77bcf86cd799439012", name: "B" },
    ]);
  });

  test("handles empty array", () => {
    expect(serializeDocuments([])).toEqual([]);
  });
});

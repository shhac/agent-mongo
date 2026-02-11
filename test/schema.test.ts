import { describe, test, expect, mock, afterAll } from "bun:test";
import type { MongoClient } from "mongodb";
import { ObjectId, Binary, Long, Decimal128 } from "mongodb";
import { mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";

// Use a temp config directory so inferSchema can read settings without mocking
const SCHEMA_TEST_CONFIG_DIR = join(tmpdir(), `agent-mongo-schema-test-${Date.now()}`);
const configDir = join(SCHEMA_TEST_CONFIG_DIR, "agent-mongo");
mkdirSync(configDir, { recursive: true });
writeFileSync(
  join(configDir, "config.json"),
  JSON.stringify({ settings: { query: { timeout: 5000 } } }),
  "utf8",
);

// Must set before importing schema module
const origXdg = process.env.XDG_CONFIG_HOME;
process.env.XDG_CONFIG_HOME = SCHEMA_TEST_CONFIG_DIR;

const { inferSchema } = await import("../src/mongo/schema.ts");

afterAll(() => {
  rmSync(SCHEMA_TEST_CONFIG_DIR, { recursive: true, force: true });
  if (origXdg !== undefined) {
    process.env.XDG_CONFIG_HOME = origXdg;
  } else {
    delete process.env.XDG_CONFIG_HOME;
  }
});

function createMockClient(docs: Record<string, unknown>[]) {
  const toArray = mock(() => Promise.resolve(docs));
  const aggregate = mock(() => ({ toArray }));
  const estimatedDocumentCount = mock(() => Promise.resolve(docs.length));
  const collection = mock(() => ({ aggregate, estimatedDocumentCount }));
  const db = mock(() => ({ collection }));
  return { db } as unknown as MongoClient;
}

describe("inferSchema", () => {
  test("detects basic field types", async () => {
    const docs = [
      { _id: new ObjectId("507f1f77bcf86cd799439011"), name: "Alice", age: 30, active: true },
      { _id: new ObjectId("507f1f77bcf86cd799439012"), name: "Bob", age: 25, active: false },
    ];
    const client = createMockClient(docs);
    const result = await inferSchema(client, "testdb", "users", 100);

    expect(result.database).toBe("testdb");
    expect(result.collection).toBe("users");
    expect(result.sampleSize).toBe(2);
    expect(result.totalDocuments).toBe(2);

    const fieldMap = new Map(result.fields.map((f) => [f.path, f]));
    expect(fieldMap.get("_id")?.types).toEqual(["ObjectId"]);
    expect(fieldMap.get("name")?.types).toEqual(["string"]);
    expect(fieldMap.get("age")?.types).toEqual(["int"]);
    expect(fieldMap.get("active")?.types).toEqual(["boolean"]);
  });

  test("detects BSON types", async () => {
    const docs = [
      {
        _id: new ObjectId("507f1f77bcf86cd799439011"),
        created: new Date("2024-01-01"),
        data: new Binary(Buffer.from("hello")),
        bigNum: Long.fromNumber(999),
        decimal: Decimal128.fromString("1.23"),
      },
    ];
    const client = createMockClient(docs);
    const result = await inferSchema(client, "testdb", "types", 100);

    const fieldMap = new Map(result.fields.map((f) => [f.path, f]));
    expect(fieldMap.get("created")?.types).toEqual(["date"]);
    expect(fieldMap.get("data")?.types).toEqual(["binary"]);
    expect(fieldMap.get("bigNum")?.types).toEqual(["long"]);
    expect(fieldMap.get("decimal")?.types).toEqual(["decimal"]);
  });

  test("walks nested objects", async () => {
    const docs = [
      {
        _id: new ObjectId("507f1f77bcf86cd799439011"),
        address: { city: "NYC", zip: 10001 },
      },
    ];
    const client = createMockClient(docs);
    const result = await inferSchema(client, "testdb", "nested", 100);

    const fieldMap = new Map(result.fields.map((f) => [f.path, f]));
    expect(fieldMap.get("address")?.types).toEqual(["object"]);
    expect(fieldMap.get("address.city")?.types).toEqual(["string"]);
    expect(fieldMap.get("address.zip")?.types).toEqual(["int"]);
  });

  test("walks array elements with .$ notation", async () => {
    const docs = [
      {
        _id: new ObjectId("507f1f77bcf86cd799439011"),
        tags: ["a", "b"],
        items: [{ name: "widget", qty: 5 }],
      },
    ];
    const client = createMockClient(docs);
    const result = await inferSchema(client, "testdb", "arrays", 100);

    const fieldMap = new Map(result.fields.map((f) => [f.path, f]));
    expect(fieldMap.get("tags")?.types).toEqual(["array"]);
    expect(fieldMap.get("tags.$")?.types).toEqual(["string"]);
    expect(fieldMap.get("items")?.types).toEqual(["array"]);
    expect(fieldMap.get("items.$")?.types).toEqual(["object"]);
    expect(fieldMap.get("items.$.name")?.types).toEqual(["string"]);
    expect(fieldMap.get("items.$.qty")?.types).toEqual(["int"]);
  });

  test("detects mixed types across documents", async () => {
    const docs = [
      { _id: new ObjectId("507f1f77bcf86cd799439011"), value: "text" },
      { _id: new ObjectId("507f1f77bcf86cd799439012"), value: 42 },
      { _id: new ObjectId("507f1f77bcf86cd799439013"), value: null },
    ];
    const client = createMockClient(docs);
    const result = await inferSchema(client, "testdb", "mixed", 100);

    const fieldMap = new Map(result.fields.map((f) => [f.path, f]));
    const valueTypes = fieldMap.get("value")?.types ?? [];
    expect(valueTypes).toContain("string");
    expect(valueTypes).toContain("int");
    expect(valueTypes).toContain("null");
  });

  test("calculates field presence ratio", async () => {
    const docs = [
      { _id: new ObjectId("507f1f77bcf86cd799439011"), name: "A", optional: "yes" },
      { _id: new ObjectId("507f1f77bcf86cd799439012"), name: "B" },
    ];
    const client = createMockClient(docs);
    const result = await inferSchema(client, "testdb", "presence", 100);

    const fieldMap = new Map(result.fields.map((f) => [f.path, f]));
    expect(fieldMap.get("_id")?.presence).toBe(1);
    expect(fieldMap.get("name")?.presence).toBe(1);
    expect(fieldMap.get("optional")?.presence).toBe(0.5);
  });

  test("returns sorted fields by path", async () => {
    const docs = [
      {
        _id: new ObjectId("507f1f77bcf86cd799439011"),
        zebra: 1,
        alpha: 2,
        middle: { nested: "x" },
      },
    ];
    const client = createMockClient(docs);
    const result = await inferSchema(client, "testdb", "sorted", 100);

    const paths = result.fields.map((f) => f.path);
    const sorted = [...paths].sort();
    expect(paths).toEqual(sorted);
  });

  test("handles empty collection", async () => {
    const client = createMockClient([]);
    const result = await inferSchema(client, "testdb", "empty", 100);

    expect(result.sampleSize).toBe(0);
    expect(result.fields).toEqual([]);
  });

  test("detects double vs int distinction", async () => {
    const docs = [{ _id: new ObjectId("507f1f77bcf86cd799439011"), integer: 42, decimal: 3.14 }];
    const client = createMockClient(docs);
    const result = await inferSchema(client, "testdb", "numbers", 100);

    const fieldMap = new Map(result.fields.map((f) => [f.path, f]));
    expect(fieldMap.get("integer")?.types).toEqual(["int"]);
    expect(fieldMap.get("decimal")?.types).toEqual(["double"]);
  });
});

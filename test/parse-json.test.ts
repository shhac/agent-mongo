import { describe, test, expect } from "bun:test";
import type { ObjectId } from "mongodb";
import { parseJson, parseJsonArray } from "../src/lib/parse-json.ts";

describe("parseJson", () => {
  test("parses plain JSON object", () => {
    const result = parseJson('{"name": "test", "value": 42}', "filter");
    expect(result).toEqual({ name: "test", value: 42 });
  });

  test("converts $date to Date instance", () => {
    const result = parseJson(
      '{"createdAt": {"$gte": {"$date": "2026-01-01T00:00:00Z"}}}',
      "filter",
    );
    expect(result.createdAt).toEqual({ $gte: new Date("2026-01-01T00:00:00Z") });
  });

  test("converts $oid to ObjectId", () => {
    const result = parseJson('{"_id": {"$oid": "507f1f77bcf86cd799439011"}}', "filter");
    expect((result._id as ObjectId).toHexString()).toBe("507f1f77bcf86cd799439011");
  });

  test("handles nested $date in query operators", () => {
    const input = JSON.stringify({
      updatedAt: {
        $gte: { $date: "2026-04-01T14:41:00Z" },
        $lte: { $date: "2026-04-02T15:57:00Z" },
      },
    });
    const result = parseJson(input, "filter");
    const updatedAt = result.updatedAt as Record<string, unknown>;
    expect(updatedAt.$gte).toBeInstanceOf(Date);
    expect(updatedAt.$lte).toBeInstanceOf(Date);
    expect((updatedAt.$gte as Date).toISOString()).toBe("2026-04-01T14:41:00.000Z");
    expect((updatedAt.$lte as Date).toISOString()).toBe("2026-04-02T15:57:00.000Z");
  });

  test("passes through plain strings and numbers unchanged", () => {
    const result = parseJson('{"status": "active", "count": 5}', "filter");
    expect(result).toEqual({ status: "active", count: 5 });
  });

  test("throws on invalid JSON with flag name", () => {
    expect(() => parseJson("not-json", "filter")).toThrow("Invalid JSON for --filter");
  });

  test("throws on invalid JSON with custom flag name", () => {
    expect(() => parseJson("{bad", "sort")).toThrow("Invalid JSON for --sort");
  });
});

describe("parseJsonArray", () => {
  test("parses a JSON array", () => {
    const result = parseJsonArray('[{"$match": {"status": "active"}}]', "pipeline");
    expect(result).toEqual([{ $match: { status: "active" } }]);
  });

  test("converts EJSON types inside array elements", () => {
    const input = JSON.stringify([
      { $match: { createdAt: { $gte: { $date: "2026-01-01T00:00:00Z" } } } },
    ]);
    const result = parseJsonArray(input, "pipeline");
    const match = (result[0] as Record<string, unknown>).$match as Record<string, unknown>;
    const createdAt = match.createdAt as Record<string, unknown>;
    expect(createdAt.$gte).toBeInstanceOf(Date);
  });

  test("throws when input is not an array", () => {
    expect(() => parseJsonArray('{"not": "array"}', "pipeline")).toThrow(
      "--pipeline must be a JSON array",
    );
  });

  test("throws on invalid JSON", () => {
    expect(() => parseJsonArray("not-json", "pipeline")).toThrow("Invalid JSON for --pipeline");
  });
});

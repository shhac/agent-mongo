import { describe, test, expect, beforeEach, spyOn } from "bun:test";
import { ObjectId } from "mongodb";
import { printNdjsonStream } from "../src/lib/output.ts";
import { configureTruncation } from "../src/lib/truncation.ts";

async function* asyncGen<T>(items: T[]): AsyncGenerator<T> {
  for (const item of items) {
    yield item;
  }
}

function captureStdout(fn: () => Promise<unknown>): Promise<string> {
  const chunks: string[] = [];
  const originalWrite = process.stdout.write;
  process.stdout.write = ((chunk: string) => {
    chunks.push(chunk);
    return true;
  }) as typeof process.stdout.write;

  return fn().finally(() => {
    process.stdout.write = originalWrite;
  }).then(() => chunks.join(""));
}

describe("printNdjsonStream", () => {
  beforeEach(() => configureTruncation({}));

  test("outputs one JSON line per document", async () => {
    const docs = [{ name: "Alice", age: 30 }, { name: "Bob", age: 25 }];
    const output = await captureStdout(() => printNdjsonStream(asyncGen(docs)));

    const lines = output.trim().split("\n");
    expect(lines).toHaveLength(2);
    expect(JSON.parse(lines[0]!)).toEqual({ name: "Alice", age: 30 });
    expect(JSON.parse(lines[1]!)).toEqual({ name: "Bob", age: 25 });
  });

  test("returns count of streamed documents", async () => {
    const docs = [{ a: 1 }, { b: 2 }, { c: 3 }];
    let count = 0;
    await captureStdout(async () => {
      count = await printNdjsonStream(asyncGen(docs));
    });
    expect(count).toBe(3);
  });

  test("returns 0 for empty cursor", async () => {
    let count = 0;
    await captureStdout(async () => {
      count = await printNdjsonStream(asyncGen([]));
    });
    expect(count).toBe(0);
  });

  test("serializes BSON types", async () => {
    const oid = new ObjectId("507f1f77bcf86cd799439011");
    const date = new Date("2026-01-15T10:30:00Z");
    const docs = [{ _id: oid, createdAt: date, name: "test" }];

    const output = await captureStdout(() => printNdjsonStream(asyncGen(docs)));
    const parsed = JSON.parse(output.trim());
    expect(parsed._id).toBe("507f1f77bcf86cd799439011");
    expect(parsed.createdAt).toBe("2026-01-15T10:30:00.000Z");
  });

  test("prunes empty values", async () => {
    const docs = [{ name: "test", empty: null, blank: "" }];
    const output = await captureStdout(() => printNdjsonStream(asyncGen(docs)));
    const parsed = JSON.parse(output.trim());
    expect(parsed).toEqual({ name: "test" });
  });

  test("truncates long strings", async () => {
    const docs = [{ content: "a".repeat(300) }];
    const output = await captureStdout(() => printNdjsonStream(asyncGen(docs)));
    const parsed = JSON.parse(output.trim());
    expect(parsed.content).toBe(`${"a".repeat(200)}\u2026`);
    expect(parsed.contentLength).toBe(300);
  });
});

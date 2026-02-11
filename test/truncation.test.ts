import { describe, test, expect, beforeEach } from "bun:test";
import { applyTruncation, configureTruncation } from "../src/lib/truncation.ts";

describe("applyTruncation", () => {
  beforeEach(() => configureTruncation({}));

  test("truncates strings over default 200 chars and adds companion length", () => {
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long }) as Record<string, unknown>;
    expect(result.description).toBe(`${"a".repeat(200)}\u2026`);
    expect(result.descriptionLength).toBe(300);
  });

  test("preserves short strings without truncation", () => {
    const result = applyTruncation({ title: "short" }) as Record<string, unknown>;
    expect(result.title).toBe("short");
    expect(result.titleLength).toBeUndefined();
  });

  test("preserves string at exactly max length", () => {
    const exact = "x".repeat(200);
    const result = applyTruncation({ body: exact }) as Record<string, unknown>;
    expect(result.body).toBe(exact);
    expect(result.bodyLength).toBeUndefined();
  });

  test("truncates at 201 chars", () => {
    const str = "y".repeat(201);
    const result = applyTruncation({ data: str }) as Record<string, unknown>;
    expect(result.data).toBe(`${"y".repeat(200)}\u2026`);
    expect(result.dataLength).toBe(201);
  });

  test("handles nested objects", () => {
    const data = { outer: { inner: "z".repeat(300), keep: "ok" } };
    const result = applyTruncation(data) as Record<string, unknown>;
    const outer = result.outer as Record<string, unknown>;
    expect(outer.inner).toBe(`${"z".repeat(200)}\u2026`);
    expect(outer.innerLength).toBe(300);
    expect(outer.keep).toBe("ok");
  });

  test("handles arrays of objects", () => {
    const data = [
      { id: "1", text: "a".repeat(300) },
      { id: "2", text: "short" },
    ];
    const result = applyTruncation(data) as Record<string, unknown>[];
    expect(result[0]!.text).toBe(`${"a".repeat(200)}\u2026`);
    expect(result[0]!.textLength).toBe(300);
    expect(result[1]!.text).toBe("short");
    expect(result[1]!.textLength).toBeUndefined();
  });

  test("handles null and undefined values", () => {
    expect(applyTruncation(null)).toBeNull();
    expect(applyTruncation(undefined)).toBeUndefined();
  });

  test("handles null fields in objects", () => {
    const result = applyTruncation({ a: null, b: "ok" }) as Record<string, unknown>;
    expect(result.a).toBeNull();
    expect(result.b).toBe("ok");
  });

  test("passes through non-object primitives", () => {
    expect(applyTruncation("hello")).toBe("hello");
    expect(applyTruncation(42)).toBe(42);
    expect(applyTruncation(true)).toBe(true);
  });

  test("preserves non-string field values", () => {
    const result = applyTruncation({ count: 42, active: true }) as Record<string, unknown>;
    expect(result.count).toBe(42);
    expect(result.active).toBe(true);
    expect(result.countLength).toBeUndefined();
  });

  test("truncates any string field name, not just known names", () => {
    const long = "q".repeat(250);
    const result = applyTruncation({ customField: long }) as Record<string, unknown>;
    expect(result.customField).toBe(`${"q".repeat(200)}\u2026`);
    expect(result.customFieldLength).toBe(250);
  });
});

describe("configureTruncation --full", () => {
  test("expands all fields when --full is set", () => {
    configureTruncation({ full: true });
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long, body: long }) as Record<string, unknown>;
    expect(result.description).toBe(long);
    expect(result.descriptionLength).toBe(300);
    expect(result.body).toBe(long);
    expect(result.bodyLength).toBe(300);
  });
});

describe("configureTruncation --expand", () => {
  beforeEach(() => configureTruncation({}));

  test("expands only specified fields", () => {
    configureTruncation({ expand: "description" });
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long, body: long }) as Record<string, unknown>;
    expect(result.description).toBe(long); // expanded
    expect(result.body).toBe(`${"a".repeat(200)}\u2026`); // still truncated
  });

  test("expands multiple comma-separated fields", () => {
    configureTruncation({ expand: "description,body" });
    const long = "a".repeat(300);
    const result = applyTruncation({
      description: long,
      body: long,
      content: long,
    }) as Record<string, unknown>;
    expect(result.description).toBe(long);
    expect(result.body).toBe(long);
    expect(result.content).toBe(`${"a".repeat(200)}\u2026`);
  });

  test("handles whitespace in expand list", () => {
    configureTruncation({ expand: " description , body " });
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long, body: long }) as Record<string, unknown>;
    expect(result.description).toBe(long);
    expect(result.body).toBe(long);
  });

  test("is case-insensitive for field matching", () => {
    configureTruncation({ expand: "Description" });
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long }) as Record<string, unknown>;
    expect(result.description).toBe(long);
  });

  test("--full takes precedence over --expand", () => {
    configureTruncation({ full: true, expand: "description" });
    const long = "a".repeat(300);
    const result = applyTruncation({ body: long }) as Record<string, unknown>;
    expect(result.body).toBe(long);
  });
});

describe("configureTruncation maxLength", () => {
  test("respects custom maxLength", () => {
    configureTruncation({ maxLength: 50 });
    const long = "m".repeat(100);
    const result = applyTruncation({ field: long }) as Record<string, unknown>;
    expect(result.field).toBe(`${"m".repeat(50)}\u2026`);
    expect(result.fieldLength).toBe(100);
  });
});

describe("configureTruncation reset", () => {
  test("resets to default truncation", () => {
    configureTruncation({ full: true });
    configureTruncation({});
    const long = "a".repeat(300);
    const result = applyTruncation({ description: long }) as Record<string, unknown>;
    expect(result.description).toBe(`${"a".repeat(200)}\u2026`);
  });
});

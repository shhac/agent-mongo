import { describe, test, expect, beforeEach, spyOn } from "bun:test";
import { printJson, printError, printPaginated } from "../src/lib/output.ts";
import { configureTruncation } from "../src/lib/truncation.ts";

describe("printJson", () => {
  beforeEach(() => configureTruncation({}));

  test("writes JSON to stdout", () => {
    const spy = spyOn(console, "log").mockImplementation(() => {});
    printJson({ name: "test", value: 42 });
    expect(spy).toHaveBeenCalledTimes(1);
    const output = JSON.parse(spy.mock.calls[0]![0] as string);
    expect(output).toEqual({ name: "test", value: 42 });
    spy.mockRestore();
  });

  test("prunes empty values before printing", () => {
    const spy = spyOn(console, "log").mockImplementation(() => {});
    printJson({ name: "test", empty: null, blank: "", nested: { x: undefined } });
    const output = JSON.parse(spy.mock.calls[0]![0] as string);
    expect(output).toEqual({ name: "test" });
    spy.mockRestore();
  });

  test("applies truncation before printing", () => {
    const spy = spyOn(console, "log").mockImplementation(() => {});
    printJson({ content: "a".repeat(300) });
    const output = JSON.parse(spy.mock.calls[0]![0] as string);
    expect(output.content).toBe(`${"a".repeat(200)}\u2026`);
    expect(output.contentLength).toBe(300);
    spy.mockRestore();
  });
});

describe("printPaginated", () => {
  beforeEach(() => configureTruncation({}));

  test("wraps items in { items } object", () => {
    const spy = spyOn(console, "log").mockImplementation(() => {});
    printPaginated([{ id: 1 }, { id: 2 }], { hasMore: false });
    const output = JSON.parse(spy.mock.calls[0]![0] as string);
    expect(output.items).toEqual([{ id: 1 }, { id: 2 }]);
    expect(output.pagination).toBeUndefined();
    spy.mockRestore();
  });

  test("includes pagination when hasMore is true", () => {
    const spy = spyOn(console, "log").mockImplementation(() => {});
    printPaginated([{ id: 1 }], { hasMore: true, nextCursor: "abc123" });
    const output = JSON.parse(spy.mock.calls[0]![0] as string);
    expect(output.items).toEqual([{ id: 1 }]);
    expect(output.pagination).toEqual({ hasMore: true, nextCursor: "abc123" });
    spy.mockRestore();
  });

  test("returns empty items array for empty input", () => {
    const spy = spyOn(console, "log").mockImplementation(() => {});
    printPaginated([], { hasMore: false });
    const output = JSON.parse(spy.mock.calls[0]![0] as string);
    expect(output.items).toEqual([]);
    spy.mockRestore();
  });

  test("prunes and truncates items", () => {
    const spy = spyOn(console, "log").mockImplementation(() => {});
    printPaginated(
      [{ name: "test", empty: null, long: "b".repeat(300) }],
      { hasMore: false },
    );
    const output = JSON.parse(spy.mock.calls[0]![0] as string);
    expect(output.items[0].empty).toBeUndefined();
    expect(output.items[0].long).toBe(`${"b".repeat(200)}\u2026`);
    expect(output.items[0].longLength).toBe(300);
    spy.mockRestore();
  });
});

describe("printError", () => {
  test("writes JSON error to stderr and sets exitCode", () => {
    const spy = spyOn(console, "error").mockImplementation(() => {});
    printError("something went wrong");
    expect(spy).toHaveBeenCalledTimes(1);
    const output = JSON.parse(spy.mock.calls[0]![0] as string);
    expect(output).toEqual({ error: "something went wrong" });
    expect(process.exitCode).toBe(1);
    spy.mockRestore();
    // Reset exitCode so bun test itself exits cleanly
    process.exitCode = 0;
  });
});

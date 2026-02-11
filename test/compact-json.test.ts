import { describe, test, expect } from "bun:test";
import { pruneEmpty } from "../src/lib/compact-json.ts";

describe("pruneEmpty", () => {
  test("removes null, undefined, empty strings, empty containers", () => {
    const input: Record<string, unknown> = {
      a: 1,
      b: null,
      c: undefined,
      d: "",
      e: "  ",
      f: "hello",
      g: [],
      h: {},
      i: { x: 1 },
      j: { nested: null, keep: "ok" },
      k: [null, "", 2, { z: "" }, { z: "a" }],
      l: 0,
      m: false,
      n: true,
    };
    expect(pruneEmpty(input)).toEqual({
      a: 1,
      f: "hello",
      i: { x: 1 },
      j: { keep: "ok" },
      k: [2, { z: "a" }],
      l: 0,
      m: false,
      n: true,
    });
  });

  test("returns empty object for fully empty input", () => {
    const input: Record<string, unknown> = { a: null, b: undefined, c: "" };
    expect(pruneEmpty(input)).toEqual({});
  });
});

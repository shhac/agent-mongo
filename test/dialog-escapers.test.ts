import { describe, test, expect } from "bun:test";
import {
  escapeAppleScript,
  escapePowerShell,
  stripTrailingNewline,
} from "../src/lib/dialog/spawn-backend.ts";

describe("escapeAppleScript", () => {
  test("plain text passes through", () => {
    expect(escapeAppleScript("Database password")).toBe("Database password");
  });

  test("backslash → doubled backslash", () => {
    expect(escapeAppleScript("a\\b")).toBe("a\\\\b");
  });

  test("double quote → escaped double quote", () => {
    expect(escapeAppleScript('say "hi"')).toBe('say \\"hi\\"');
  });

  test("backslash before quote: backslash escapes first, then quote", () => {
    expect(escapeAppleScript('\\"')).toBe('\\\\\\"');
  });

  test("malicious payload terminating the string: every quote is escaped", () => {
    const evil = '"; do shell script "rm -rf ~"; "';
    const escaped = escapeAppleScript(evil);
    expect(escaped).toBe('\\"; do shell script \\"rm -rf ~\\"; \\"');
    // Every literal " in the original must be preceded by \ in the output.
    const matches = escaped.match(/(?<!\\)"/g);
    expect(matches).toBeNull();
  });

  test("empty string", () => {
    expect(escapeAppleScript("")).toBe("");
  });

  test("does not transform single quotes", () => {
    expect(escapeAppleScript("it's fine")).toBe("it's fine");
  });
});

describe("escapePowerShell", () => {
  test("plain text passes through", () => {
    expect(escapePowerShell("Database password")).toBe("Database password");
  });

  test("single quote → doubled single quote", () => {
    expect(escapePowerShell("it's")).toBe("it''s");
  });

  test("repeated single quotes", () => {
    expect(escapePowerShell("''")).toBe("''''");
  });

  test("malicious payload terminating the string: every quote is doubled", () => {
    const evil = "'; Remove-Item ~ -Recurse; '";
    const escaped = escapePowerShell(evil);
    expect(escaped).toBe("''; Remove-Item ~ -Recurse; ''");
    // Every literal ' must come in an even-count run after escaping.
    const runs = escaped.match(/'+/g) ?? [];
    expect(runs.every((r) => r.length % 2 === 0)).toBe(true);
  });

  test("does not transform $() or backticks (single-quoted strings don't interpret them)", () => {
    expect(escapePowerShell("$(whoami)`id`")).toBe("$(whoami)`id`");
  });

  test("empty string", () => {
    expect(escapePowerShell("")).toBe("");
  });
});

describe("stripTrailingNewline", () => {
  test("strips a single trailing LF", () => {
    expect(stripTrailingNewline("hello\n")).toBe("hello");
  });

  test("preserves no trailing newline", () => {
    expect(stripTrailingNewline("hello")).toBe("hello");
  });

  test("only \\n becomes empty", () => {
    expect(stripTrailingNewline("\n")).toBe("");
  });

  test("empty stays empty", () => {
    expect(stripTrailingNewline("")).toBe("");
  });

  test("strips only the last newline (multi-line input keeps internal newlines)", () => {
    expect(stripTrailingNewline("a\nb\n")).toBe("a\nb");
  });
});

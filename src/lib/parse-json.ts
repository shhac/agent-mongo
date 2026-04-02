import { EJSON } from "bson";

/**
 * Parse a JSON string with MongoDB Extended JSON support.
 * Converts $date, $oid, $numberLong, etc. into native BSON types
 * that the MongoDB driver understands.
 */
export function parseJson(value: string, name: string): Record<string, unknown> {
  try {
    return EJSON.parse(value) as Record<string, unknown>;
  } catch {
    throw new Error(`Invalid JSON for --${name}: ${value}`);
  }
}

export function parseJsonArray(value: string, name: string): unknown[] {
  try {
    const parsed = EJSON.parse(value);
    if (!Array.isArray(parsed)) {
      throw new Error(`--${name} must be a JSON array`);
    }
    return parsed as unknown[];
  } catch (err) {
    if (err instanceof Error && err.message.startsWith("--")) {
      throw err;
    }
    throw new Error(`Invalid JSON for --${name}: ${value.slice(0, 100)}${value.length > 100 ? "..." : ""}`);
  }
}

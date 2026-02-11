import { applyTruncation } from "./truncation.ts";
import { pruneEmpty } from "./compact-json.ts";
import { getSettings } from "./config.ts";

const DEFAULT_PAGE_SIZE = 20;

export function printJson(data: unknown): void {
  console.log(JSON.stringify(applyTruncation(pruneEmpty(data)), null, 2));
}

/** Print JSON without truncation — for admin/config output where truncation is undesirable. */
export function printJsonRaw(data: unknown): void {
  console.log(JSON.stringify(pruneEmpty(data), null, 2));
}

/**
 * Print paginated list output with { items, pagination? } wrapper.
 * Always returns { items: [...] } even when the array is empty.
 * Includes pagination when hasMore is true.
 */
export function printPaginated(
  items: unknown[],
  pageInfo: { hasMore: boolean; nextCursor?: string },
): void {
  const { hasMore, nextCursor } = pageInfo;
  const prunedItems = items.map((item) => applyTruncation(pruneEmpty(item)));
  const payload: Record<string, unknown> = { items: prunedItems };
  if (hasMore) {
    payload.pagination = {
      hasMore: true,
      nextCursor: nextCursor ?? null,
    };
  }
  console.log(JSON.stringify(payload, null, 2));
}

export function printError(message: string): void {
  console.error(JSON.stringify({ error: message }));
  process.exitCode = 1;
}

export function resolvePageSize(opts: { limit?: string }): number {
  if (opts.limit !== undefined) {
    return parseInt(opts.limit, 10);
  }
  const settings = getSettings();
  return settings.defaults?.limit ?? DEFAULT_PAGE_SIZE;
}

import { MongoServerError } from "mongodb";
import { getTimeout } from "./timeout.ts";

type ErrorContext = {
  database?: string;
  collection?: string;
};

export function enhanceErrorMessage(err: Error, context?: ErrorContext): string {
  if (err instanceof MongoServerError && err.code === 50) {
    const timeout = getTimeout();
    const hints = [
      `Query timed out after ${timeout}ms.`,
      `Increase with: --timeout <ms> or agent-mongo config set query.timeout <ms>`,
    ];
    if (context?.database && context?.collection) {
      hints.push(
        `Check indexes: agent-mongo collection indexes ${context.database} ${context.collection}`,
      );
    }
    return `${err.message}. ${hints.join(". ")}`;
  }
  return err.message;
}

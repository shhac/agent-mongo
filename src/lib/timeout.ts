import { getSettings } from "./config.ts";

const DEFAULT_TIMEOUT = 30000;

let cliOverride: number | undefined;

export function configureTimeout(ms?: number): void {
  cliOverride = ms;
}

export function getTimeout(): number {
  return cliOverride ?? getSettings().query?.timeout ?? DEFAULT_TIMEOUT;
}

import { ref, type Command } from "vipvot";
import { configureTruncation } from "../lib/truncation.ts";
import { configureTimeout } from "../lib/timeout.ts";
import { getSettings } from "../lib/config.ts";

/**
 * Persistent flag refs owned by the root command. Cobra-idiomatic:
 * subcommand files import the ref directly rather than walking the
 * parent chain (cobra's Go code uses package-level vars the same way).
 */
export const connectionRef = ref<string>("");
export const expandRef = ref<string>("");
export const fullRef = ref<boolean>(false);
export const timeoutRef = ref<string>("");

/** Resolved alias to use, or undefined if unset. Empty string is treated as unset. */
export function resolveConnectionAlias(): string | undefined {
  return connectionRef.value || undefined;
}

/** persistentPreRun handler — wires global flags into truncation/timeout config. */
export function applyGlobals(_cmd: Command, _args: string[]): void {
  const settings = getSettings();
  configureTruncation({
    expand: expandRef.value || undefined,
    full: fullRef.value || undefined,
    maxLength: settings.truncation?.maxLength,
  });
  if (timeoutRef.value) {
    const ms = parseInt(timeoutRef.value, 10);
    if (!Number.isFinite(ms) || ms < 1) {
      throw new Error(
        `Invalid --timeout: "${timeoutRef.value}". Must be a positive integer (milliseconds).`,
      );
    }
    configureTimeout(ms);
  }
}

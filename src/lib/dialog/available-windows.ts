import { DialogError } from "./errors.ts";

/**
 * Windows: Win32-OpenSSH leaves $SESSIONNAME unset when the SSH server runs
 * as a service; service contexts also fail. An interactive desktop session
 * has SESSIONNAME as "Console" or "RDP-Tcp#N". We allow anything non-empty.
 *
 * Accepts an env object for testability; defaults to `process.env`.
 */
export function windowsAvailable(env: NodeJS.ProcessEnv = process.env): DialogError | null {
  if (!env.SESSIONNAME) {
    return new DialogError("no-gui", "$SESSIONNAME unset (likely SSH or service context)");
  }
  return null;
}

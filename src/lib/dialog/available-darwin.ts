import { DialogError } from "./errors.ts";

/**
 * macOS: osascript fails cleanly if no Aqua session is attached, so we let
 * the dialog itself surface most failures. The one case we pre-flight is
 * "obviously SSH'd in": $SSH_CONNECTION is set and no local terminal app has
 * set $TERM_PROGRAM.
 *
 * Accepts an env object for testability; defaults to `process.env`.
 */
export function darwinAvailable(env: NodeJS.ProcessEnv = process.env): DialogError | null {
  if (env.SSH_CONNECTION && !env.TERM_PROGRAM) {
    return new DialogError("no-gui", "appears to be an SSH session with no local terminal");
  }
  return null;
}

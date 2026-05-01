/**
 * Look up a binary on $PATH. Two flavors are exposed because the dialog
 * package's two consumers prefer different shapes:
 *
 * - `available.ts` is sync (called from `Prompter.available()` which is sync).
 * - `spawn-backend.ts` is async (called inside the async `prompt()` flow).
 */

export function hasBinarySync(name: string): boolean {
  const proc = Bun.spawnSync(["sh", "-c", `command -v ${name}`], {
    stdout: "pipe",
    stderr: "pipe",
  });
  return proc.exitCode === 0;
}

export async function hasBinary(name: string): Promise<boolean> {
  const proc = Bun.spawn(["sh", "-c", `command -v ${name}`], {
    stdout: "pipe",
    stderr: "pipe",
  });
  const exitCode = await proc.exited;
  return exitCode === 0;
}

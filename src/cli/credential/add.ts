import type { Command } from "commander";
import { storeCredential } from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";
import { promptMissingViaDialog } from "./form.ts";

type AddOpts = {
  username?: string;
  password?: string;
  form?: boolean;
};

export function registerAdd(credential: Command): void {
  credential
    .command("add")
    .description("Add or update a stored credential")
    .argument("<name>", "Short name for this credential (e.g. acme, globex)")
    .option("--username <user>", "MongoDB username")
    .option("--password <pass>", "MongoDB password")
    .option(
      "--form",
      "Prompt for missing username/password via a native OS dialog (LLM-safe; the secret is typed directly into the OS, never seen by the agent)",
    )
    .action(async (name: string, opts: AddOpts) => {
      try {
        const resolved = await resolveCredentials(name, opts);
        if (!resolved) {
          return;
        }
        const { storage } = storeCredential(name, resolved);
        printJsonRaw({
          ok: true,
          credential: name,
          username: resolved.username,
          storage,
          hint: `Use with: agent-mongo connection add <alias> <uri> --credential ${name}`,
        });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to add credential");
      }
    });
}

async function resolveCredentials(
  name: string,
  opts: AddOpts,
): Promise<{ username: string; password: string } | null> {
  let username = opts.username ?? "";
  let password = opts.password ?? "";

  if (opts.form) {
    const result = await promptMissingViaDialog({ name, username, password });
    if (result.error) {
      printError(JSON.stringify({ ...result.error, credential: name }));
      return null;
    }
    ({ username, password } = result);
  }

  if (!username || !password) {
    printError(
      "Missing --username and/or --password. Pass them on the command line, or use --form for an OS dialog.",
    );
    return null;
  }
  return { username, password };
}

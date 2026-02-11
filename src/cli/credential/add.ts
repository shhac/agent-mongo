import type { Command } from "commander";
import { storeCredential } from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function registerAdd(credential: Command): void {
  credential
    .command("add")
    .description("Add or update a stored credential")
    .argument("<name>", "Short name for this credential (e.g. acme, globex)")
    .requiredOption("--username <user>", "MongoDB username")
    .requiredOption("--password <pass>", "MongoDB password")
    .action((name: string, opts: { username: string; password: string }) => {
      try {
        storeCredential(name, {
          username: opts.username,
          password: opts.password,
        });
        printJsonRaw({
          ok: true,
          credential: name,
          username: opts.username,
          hint: `Use with: agent-mongo connection add <alias> <uri> --credential ${name}`,
        });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to add credential");
      }
    });
}

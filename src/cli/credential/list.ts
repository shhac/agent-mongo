import type { Command } from "commander";
import { getCredentials, getConnectionsUsingCredential } from "../../lib/config.ts";
import { printJsonRaw } from "../../lib/output.ts";

export function registerList(credential: Command): void {
  credential
    .command("list")
    .description("List stored credentials (passwords redacted)")
    .action(() => {
      const credentials = getCredentials();

      const items = Object.entries(credentials).map(([name, cred]) => ({
        name,
        username: cred.username,
        password: "***",
        usedBy: getConnectionsUsingCredential(name),
      }));

      printJsonRaw({ credentials: items });
    });
}

import { Command, NoArgs } from "vipvot";
import {
  getCredentials,
  getConnectionsUsingCredential,
  getCredentialStorage,
} from "../../lib/config.ts";
import { printJsonRaw } from "../../lib/output.ts";

export function buildListCommand(): Command {
  return Command({
    use: "list",
    short: "List stored credentials (passwords redacted)",
    args: NoArgs,
    run: () => {
      const credentials = getCredentials();

      const items = Object.entries(credentials).map(([name, cred]) => ({
        name,
        username: cred.username === "__KEYCHAIN__" ? "(keychain)" : cred.username,
        password: "***",
        storage: getCredentialStorage(name),
        usedBy: getConnectionsUsingCredential(name),
      }));

      printJsonRaw({ credentials: items });
    },
  });
}

import { Command, ExactArgs, ref } from "vipvot";
import {
  storeConnection,
  setDefaultConnection,
  getCredential,
  getCredentials,
} from "../../lib/config.ts";
import { parseDbFromUri } from "../../mongo/client.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function buildAddCommand(): Command {
  const database = ref<string>("");
  const credential = ref<string>("");
  const setDefault = ref<boolean>(false);

  const cmd = Command({
    use: "add <alias> <connection-string>",
    short: "Add a MongoDB connection",
    args: ExactArgs(2),
    run: (_cmd, args) => {
      const [alias, connectionString] = args as [string, string];
      try {
        if (credential.value) {
          const cred = getCredential(credential.value);
          if (!cred) {
            const available = Object.keys(getCredentials());
            throw new Error(
              `Credential "${credential.value}" not found. Available: ${available.join(", ") || "(none)"}. Run: agent-mongo credential add <alias> --username <user> --password <pass>`,
            );
          }
        }

        storeConnection(alias, {
          connection_string: connectionString,
          name: alias,
          database: database.value || undefined,
          credential: credential.value || undefined,
        });

        if (setDefault.value) {
          setDefaultConnection(alias);
        }

        printJsonRaw({
          ok: true,
          alias,
          database: database.value || parseDbFromUri(connectionString),
          credential: credential.value || undefined,
          isDefault: setDefault.value,
          hint: `Test with: agent-mongo connection test ${alias}`,
        });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to add connection");
      }
    },
  });

  cmd.flags().stringVarP(database, "database", "", "", "Override database name from URI");
  cmd.flags().stringVarP(credential, "credential", "", "", "Credential alias for authentication");
  cmd.flags().boolVarP(setDefault, "default", "", false, "Set as default connection");

  return cmd;
}

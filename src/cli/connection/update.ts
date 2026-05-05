import { Command, ExactArgs, ref } from "vipvot";
import { updateConnection, getCredential, getCredentials } from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function buildUpdateCommand(): Command {
  const credential = ref<string>("");
  const clearCredential = ref<boolean>(false);
  const database = ref<string>("");

  const cmd = Command({
    use: "update <alias>",
    short: "Update a saved connection",
    args: ExactArgs(1),
    run: (_cmd, args) => {
      const [alias] = args as [string];
      try {
        if (credential.value && clearCredential.value) {
          throw new Error("Cannot use --credential and --clear-credential together.");
        }

        if (credential.value) {
          const cred = getCredential(credential.value);
          if (!cred) {
            const available = Object.keys(getCredentials());
            throw new Error(
              `Credential "${credential.value}" not found. Available: ${available.join(", ") || "(none)"}. Run: agent-mongo credential add <alias> --username <user> --password <pass>`,
            );
          }
        }

        const updates: Record<string, string | undefined> = {};
        if (database.value) {
          updates.database = database.value;
        }
        if (clearCredential.value) {
          updates.credential = undefined;
        } else if (credential.value) {
          updates.credential = credential.value;
        }

        updateConnection(alias, updates);
        printJsonRaw({ ok: true, alias, updated: Object.keys(updates) });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to update connection");
      }
    },
  });

  cmd.flags().stringVarP(credential, "credential", "", "", "Credential alias for authentication");
  cmd
    .flags()
    .boolVarP(clearCredential, "clear-credential", "", false, "Remove credential from connection");
  cmd.flags().stringVarP(database, "database", "", "", "Override database name");
  cmd.markFlagsMutuallyExclusive("credential", "clear-credential");

  return cmd;
}

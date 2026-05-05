import { Command, ExactArgs, ref } from "vipvot";
import {
  removeCredential,
  getConnectionsUsingCredential,
  updateConnection,
} from "../../lib/config.ts";
import { printError, printJsonRaw } from "../../lib/output.ts";

export function buildRemoveCommand(): Command {
  const force = ref<boolean>(false);

  const cmd = Command({
    use: "remove <name>",
    short: "Remove a stored credential",
    args: ExactArgs(1),
    run: (_cmd, args) => {
      const [name] = args as [string];
      try {
        const usedBy = getConnectionsUsingCredential(name);

        if (usedBy.length > 0 && force.value) {
          for (const connAlias of usedBy) {
            updateConnection(connAlias, { credential: undefined });
          }
        }

        removeCredential(name);
        printJsonRaw({
          ok: true,
          removed: name,
          clearedFrom: usedBy.length > 0 && force.value ? usedBy : undefined,
        });
      } catch (err) {
        printError(err instanceof Error ? err.message : "Failed to remove credential");
      }
    },
  });

  cmd
    .flags()
    .boolVarP(
      force,
      "force",
      "",
      false,
      "Remove even if referenced by connections (clears their credential refs)",
    );

  return cmd;
}

import { Command, ExactArgs, ref } from "vipvot";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { countDocuments } from "../../mongo/query.ts";
import { enhanceErrorMessage } from "../../lib/errors.ts";
import { parseJson } from "../../lib/parse-json.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildCountCommand(): Command {
  const filter = ref<string>("");

  const cmd = Command({
    use: "count <database> <collection>",
    short: "Count documents matching a filter",
    args: ExactArgs(2),
    run: async (_cmd, args) => {
      const [database, collection] = args as [string, string];
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());
        const filterDoc = filter.value ? parseJson(filter.value, "filter") : undefined;
        const count = await countDocuments(client, {
          dbName: database,
          collName: collection,
          filter: filterDoc,
        });
        printJson({ database, collection, filter: filterDoc ?? {}, count });
      } catch (err) {
        printError(
          err instanceof Error
            ? enhanceErrorMessage(err, { database, collection })
            : "Failed to count documents",
        );
      } finally {
        await closeAllClients();
      }
    },
  });

  cmd.flags().stringVarP(filter, "filter", "", "", "MongoDB query filter (JSON)");

  return cmd;
}

import { Command, ExactArgs, ref } from "vipvot";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { getDistinctValues } from "../../mongo/query.ts";
import { enhanceErrorMessage } from "../../lib/errors.ts";
import { parseJson } from "../../lib/parse-json.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildDistinctCommand(): Command {
  const filter = ref<string>("");

  const cmd = Command({
    use: "distinct <database> <collection> <field>",
    short: "Get distinct values for a field",
    args: ExactArgs(3),
    run: async (_cmd, args) => {
      const [database, collection, field] = args as [string, string, string];
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());
        const filterDoc = filter.value ? parseJson(filter.value, "filter") : undefined;
        const values = await getDistinctValues(client, {
          dbName: database,
          collName: collection,
          field,
          filter: filterDoc,
        });
        printJson({ database, collection, field, values, count: values.length });
      } catch (err) {
        printError(
          err instanceof Error
            ? enhanceErrorMessage(err, { database, collection })
            : "Failed to get distinct values",
        );
      } finally {
        await closeAllClients();
      }
    },
  });

  cmd.flags().stringVarP(filter, "filter", "", "", "MongoDB query filter (JSON)");

  return cmd;
}

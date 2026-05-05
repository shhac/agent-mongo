import { Command, ExactArgs, ref } from "vipvot";
import { printJson, printError } from "../../lib/output.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { findById } from "../../mongo/query.ts";
import { enhanceErrorMessage } from "../../lib/errors.ts";
import { parseJson } from "../../lib/parse-json.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildGetCommand(): Command {
  const idType = ref<string>("");
  const projection = ref<string>("");

  const cmd = Command({
    use: "get <database> <collection> <id>",
    short: "Get a single document by _id",
    args: ExactArgs(3),
    run: async (_cmd, args) => {
      const [database, collection, id] = args as [string, string, string];
      try {
        if (idType.value && !["objectid", "string", "number"].includes(idType.value)) {
          throw new Error(`Invalid --type: "${idType.value}". Valid: objectid, string, number`);
        }

        const { client } = await getMongoClient(resolveConnectionAlias());
        const projectionDoc = projection.value
          ? parseJson(projection.value, "projection")
          : undefined;
        const doc = await findById(client, {
          dbName: database,
          collName: collection,
          rawId: id,
          idType: idType.value || undefined,
          projection: projectionDoc,
        });

        if (!doc) {
          throw new Error(`Document not found: _id=${id} in ${database}.${collection}`);
        }

        printJson({
          database,
          collection,
          fieldCount: Object.keys(doc).length,
          document: doc,
        });
      } catch (err) {
        printError(
          err instanceof Error
            ? enhanceErrorMessage(err, { database, collection })
            : "Failed to get document",
        );
      } finally {
        await closeAllClients();
      }
    },
  });

  cmd
    .flags()
    .stringVarP(
      idType,
      "type",
      "",
      "",
      "Force ID type: objectid, string, number (auto-detected by default)",
    );
  cmd
    .flags()
    .stringVarP(
      projection,
      "projection",
      "",
      "",
      'Field projection (e.g. {"name": 1, "email": 1})',
    );

  return cmd;
}

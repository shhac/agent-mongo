import { Command, ExactArgs, ref } from "vipvot";
import type { Sort } from "mongodb";
import { printJson, printError, printNdjsonStream, resolvePageSize } from "../../lib/output.ts";
import { getSettings } from "../../lib/config.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { findDocuments, streamFind } from "../../mongo/query.ts";
import { enhanceErrorMessage } from "../../lib/errors.ts";
import { parseJson } from "../../lib/parse-json.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildFindCommand(): Command {
  const filter = ref<string>("");
  const sort = ref<string>("");
  const projection = ref<string>("");
  const limit = ref<string>("");
  const skip = ref<string>("0");
  const stream = ref<boolean>(false);

  const cmd = Command({
    use: "find <database> <collection>",
    short: "Find documents matching a filter",
    args: ExactArgs(2),
    run: async (_cmd, args) => {
      const [database, collection] = args as [string, string];
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());

        const filterDoc = filter.value ? parseJson(filter.value, "filter") : undefined;
        const sortDoc = (sort.value ? parseJson(sort.value, "sort") : { _id: -1 }) as Sort;
        const projectionDoc = projection.value
          ? parseJson(projection.value, "projection")
          : undefined;

        if (stream.value) {
          const cursor = streamFind(client, {
            dbName: database,
            collName: collection,
            filter: filterDoc,
            sort: sortDoc,
            projection: projectionDoc,
          });
          await printNdjsonStream(cursor);
        } else {
          const maxDocs = getSettings().query?.maxDocuments ?? 100;
          const requestedLimit = resolvePageSize({ limit: limit.value || undefined });
          const limitVal = Math.min(requestedLimit, maxDocs);
          const skipVal = parseInt(skip.value || "0", 10);

          const result = await findDocuments(client, {
            dbName: database,
            collName: collection,
            filter: filterDoc,
            sort: sortDoc,
            projection: projectionDoc,
            limit: limitVal,
            skip: skipVal,
          });

          printJson(result);
        }
      } catch (err) {
        printError(
          err instanceof Error
            ? enhanceErrorMessage(err, { database, collection })
            : "Failed to find documents",
        );
      } finally {
        await closeAllClients();
      }
    },
  });

  cmd.flags().stringVarP(filter, "filter", "", "", "MongoDB query filter (JSON)");
  cmd.flags().stringVarP(sort, "sort", "", "", 'Sort specification (e.g. {"createdAt": -1})');
  cmd
    .flags()
    .stringVarP(
      projection,
      "projection",
      "",
      "",
      'Field projection (e.g. {"name": 1, "email": 1})',
    );
  cmd.flags().stringVarP(limit, "limit", "", "", "Max documents to return");
  cmd.flags().stringVarP(skip, "skip", "", "0", "Number of documents to skip");
  cmd
    .flags()
    .boolVarP(
      stream,
      "stream",
      "",
      false,
      "Stream results as NDJSON (one JSON object per line, no limit)",
    );

  return cmd;
}

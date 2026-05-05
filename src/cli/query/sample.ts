import { Command, ExactArgs, ref } from "vipvot";
import type { Document } from "mongodb";
import { printJson, printError } from "../../lib/output.ts";
import { getSettings } from "../../lib/config.ts";
import { getTimeout } from "../../lib/timeout.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { serializeDocuments } from "../../lib/serialize.ts";
import { enhanceErrorMessage } from "../../lib/errors.ts";
import { parseJson } from "../../lib/parse-json.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildSampleCommand(): Command {
  const size = ref<string>("");
  const filter = ref<string>("");

  const cmd = Command({
    use: "sample <database> <collection>",
    short: "Get random sample of documents",
    args: ExactArgs(2),
    run: async (_cmd, args) => {
      const [database, collection] = args as [string, string];
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());

        const defaultSize = getSettings().defaults?.sampleSize ?? 5;
        const maxDocs = getSettings().query?.maxDocuments ?? 100;
        const requestedSize = size.value ? parseInt(size.value, 10) : defaultSize;
        if (!Number.isFinite(requestedSize) || requestedSize < 1) {
          throw new Error(`Invalid --size: "${size.value}". Must be a positive integer.`);
        }
        const sampleSize = Math.min(requestedSize, maxDocs);

        const filterDoc = filter.value ? parseJson(filter.value, "filter") : undefined;
        const pipeline: Document[] = [];
        if (filterDoc) {
          pipeline.push({ $match: filterDoc });
        }
        pipeline.push({ $sample: { size: sampleSize } });

        const timeout = getTimeout();
        const coll = client.db(database).collection(collection);
        const docs = await coll.aggregate<Document>(pipeline, { maxTimeMS: timeout }).toArray();

        printJson({
          database,
          collection,
          filter: filterDoc ?? {},
          sampleSize: docs.length,
          documents: serializeDocuments(docs),
        });
      } catch (err) {
        printError(
          err instanceof Error
            ? enhanceErrorMessage(err, { database, collection })
            : "Failed to sample documents",
        );
      } finally {
        await closeAllClients();
      }
    },
  });

  cmd.flags().stringVarP(size, "size", "", "", "Number of random documents");
  cmd.flags().stringVarP(filter, "filter", "", "", "MongoDB query filter (JSON)");

  return cmd;
}

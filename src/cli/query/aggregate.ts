import { Command, RangeArgs, ref } from "vipvot";
import type { Document } from "mongodb";
import { printJson, printError, printNdjsonStream, resolvePageSize } from "../../lib/output.ts";
import { getSettings } from "../../lib/config.ts";
import { getMongoClient, closeAllClients } from "../../mongo/client.ts";
import { runAggregate, streamAggregate } from "../../mongo/aggregate.ts";
import { enhanceErrorMessage } from "../../lib/errors.ts";
import { parseJsonArray } from "../../lib/parse-json.ts";
import { resolveConnectionAlias } from "../_globals.ts";

export function buildAggregateCommand(): Command {
  const pipelineFlag = ref<string>("");
  const limit = ref<string>("");
  const stream = ref<boolean>(false);

  const cmd = Command({
    use: "aggregate <database> <collection> [pipeline]",
    short: "Run a read-only aggregation pipeline",
    args: RangeArgs(2, 3),
    run: async (_cmd, args) => {
      const [database, collection, pipelineArg] = args as [string, string, string?];
      try {
        const { client } = await getMongoClient(resolveConnectionAlias());

        const pipeline = await resolvePipeline(pipelineArg, pipelineFlag.value || undefined);

        if (stream.value) {
          const cursor = streamAggregate(client, {
            dbName: database,
            collName: collection,
            pipeline,
          });
          await printNdjsonStream(cursor);
        } else {
          const maxDocs = getSettings().query?.maxDocuments ?? 100;
          const requestedLimit = resolvePageSize({ limit: limit.value || undefined });
          const limitVal = Math.min(requestedLimit, maxDocs);

          const result = await runAggregate(client, {
            dbName: database,
            collName: collection,
            pipeline,
            limit: limitVal,
          });
          printJson({ database, collection, ...result });
        }
      } catch (err) {
        printError(
          err instanceof Error
            ? enhanceErrorMessage(err, { database, collection })
            : "Failed to run aggregation",
        );
      } finally {
        await closeAllClients();
      }
    },
  });

  cmd
    .flags()
    .stringVarP(
      pipelineFlag,
      "pipeline",
      "",
      "",
      "Aggregation pipeline as JSON array (or pipe via stdin)",
    );
  cmd.flags().stringVarP(limit, "limit", "", "", "Max results if pipeline has no $limit stage");
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

async function resolvePipeline(positionalArg?: string, pipelineFlag?: string): Promise<Document[]> {
  let raw: string;

  if (positionalArg) {
    raw = positionalArg;
  } else if (pipelineFlag) {
    raw = pipelineFlag;
  } else if (!process.stdin.isTTY) {
    raw = await readStdin();
  } else {
    throw new Error(
      "Provide pipeline as argument, --pipeline <json>, or pipe a JSON array via stdin.",
    );
  }

  return parseJsonArray(raw, "pipeline") as Document[];
}

async function readStdin(): Promise<string> {
  const chunks: string[] = [];
  process.stdin.setEncoding("utf8");
  for await (const chunk of process.stdin) {
    chunks.push(chunk as string);
  }
  const result = chunks.join("").trim();
  if (!result) {
    throw new Error(
      "Empty stdin. Provide pipeline as argument, --pipeline <json>, or pipe a JSON array via stdin.",
    );
  }
  return result;
}

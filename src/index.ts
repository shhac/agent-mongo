import { Command, helpString, VipvotError } from "vipvot";
import { getPackageVersion } from "./lib/version.ts";
import { applyGlobals, connectionRef, expandRef, fullRef, timeoutRef } from "./cli/_globals.ts";
import { buildConnectionCommand } from "./cli/connection/index.ts";
import { buildCredentialCommand } from "./cli/credential/index.ts";
import { buildConfigCommand } from "./cli/config/index.ts";
import { buildDatabaseCommand } from "./cli/database/index.ts";
import { buildCollectionCommand } from "./cli/collection/index.ts";
import { buildQueryCommand } from "./cli/query/index.ts";
import { buildUsageCommand } from "./cli/usage/index.ts";
import { printError } from "./lib/output.ts";

const root = Command({
  use: "agent-mongo",
  short: "MongoDB CLI for AI agents",
  long: `agent-mongo ${getPackageVersion()} — MongoDB CLI for AI agents`,
  silenceErrors: true,
  silenceUsage: true,
  persistentPreRun: applyGlobals,
  run: (cmd) => {
    cmd.out()(`${helpString(cmd)}\n`);
  },
});

root.persistentFlags().stringVarP(connectionRef, "connection", "c", "", "Connection alias to use");
root
  .persistentFlags()
  .stringVarP(expandRef, "expand", "", "", "Expand truncated fields (comma-separated field names)");
root
  .persistentFlags()
  .boolVarP(fullRef, "full", "", false, "Show full content for all truncated fields");
root
  .persistentFlags()
  .stringVarP(timeoutRef, "timeout", "", "", "Query timeout in milliseconds (overrides config)");

root.addCommand(buildConnectionCommand());
root.addCommand(buildCredentialCommand());
root.addCommand(buildConfigCommand());
root.addCommand(buildDatabaseCommand());
root.addCommand(buildCollectionCommand());
root.addCommand(buildQueryCommand());
root.addCommand(buildUsageCommand());

const result = await root.execute(process.argv.slice(2));
if (result instanceof VipvotError || result instanceof Error) {
  printError(result.message);
}

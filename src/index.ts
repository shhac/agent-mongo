import { Command } from "commander";
import { getPackageVersion } from "./lib/version.ts";
import { configureTruncation } from "./lib/truncation.ts";
import { getSettings } from "./lib/config.ts";
import { registerConnectionCommand } from "./cli/connection/index.ts";
import { registerConfigCommand } from "./cli/config/index.ts";
import { registerDbCommand } from "./cli/db/index.ts";
import { registerCollectionCommand } from "./cli/collection/index.ts";
import { registerQueryCommand } from "./cli/query/index.ts";
import { registerUsageCommand } from "./cli/usage/index.ts";

const program = new Command();
program.name("agent-mongo").description("MongoDB CLI for AI agents").version(getPackageVersion());
program.option("--expand <fields>", "Expand truncated fields (comma-separated field names)");
program.option("--full", "Show full content for all truncated fields");
program.option("-c, --connection <alias>", "Connection alias to use");
program.hook("preAction", (thisCommand) => {
  const opts = thisCommand.opts();
  const settings = getSettings();
  configureTruncation({
    expand: opts.expand,
    full: opts.full,
    maxLength: settings.truncation?.maxLength,
  });
});

registerConnectionCommand({ program });
registerConfigCommand({ program });
registerDbCommand({ program });
registerCollectionCommand({ program });
registerQueryCommand({ program });
registerUsageCommand({ program });

program.parse(process.argv);
if (!process.argv.slice(2).length) {
  program.outputHelp();
}

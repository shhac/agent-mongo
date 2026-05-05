import { Command, NoArgs } from "vipvot";

const USAGE_TEXT = `config — Manage CLI settings

COMMANDS:
  config get <key>              Get a config value
  config set <key> <value>      Set a config value
  config reset                  Reset all settings to defaults
  config list-keys              List all valid keys with defaults and ranges

KEYS:
  defaults.limit        (20)     Default result limit for list/query commands [1-1000]
  defaults.sampleSize       (5)      Default sample size for query sample [1-100]
  defaults.schemaSampleSize (100)    Default sample size for schema inference [1-1000]
  query.timeout         (30000)  Query timeout in ms [1000-300000]
  query.maxDocuments    (100)    Max documents per query [1-10000]
  truncation.maxLength  (200)    Max string length before truncation [50-100000]

EXAMPLES:
  agent-mongo config set defaults.limit 50
  agent-mongo config get query.timeout
  agent-mongo config reset
`;

export function buildUsageCommand(): Command {
  return Command({
    use: "usage",
    short: "Print config command documentation (LLM-optimized)",
    args: NoArgs,
    run: () => {
      console.log(USAGE_TEXT.trim());
    },
  });
}

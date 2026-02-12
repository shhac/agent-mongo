# `agent-mongo` command map (reference)

Run `agent-mongo usage` for concise LLM-optimized docs.
Run `agent-mongo <command> usage` for detailed per-command docs.

## Connection

- `agent-mongo connection add <alias> <uri> [--database <db>] [--credential <name>] [--default]` — save a MongoDB connection
- `agent-mongo connection update <alias> [--credential <name>] [--no-credential] [--database <db>]` — update saved connection
- `agent-mongo connection remove <alias>` — remove a saved connection
- `agent-mongo connection list` — list all saved connections with URIs, credential names, and default status
- `agent-mongo connection test [-c <alias>]` — ping MongoDB to verify connectivity
- `agent-mongo connection set-default <alias>` — set default connection

## Credential

- `agent-mongo credential add <name> --username <user> --password <pass>` — store named credential (overwrites if exists)
- `agent-mongo credential remove <name> [--force]` — remove credential (--force: remove even if referenced)
- `agent-mongo credential list` — list credentials (passwords redacted) with referencing connections

## Config

- `agent-mongo config get <key>` — get a config value
- `agent-mongo config set <key> <value>` — set a config value
- `agent-mongo config reset` — reset all settings to defaults
- `agent-mongo config list-keys` — list all valid keys with defaults and ranges

## Database

- `agent-mongo database list [-c <alias>]` — list all databases with sizes
- `agent-mongo database stats <database> [-c <alias>]` — database statistics (collection count, document count, sizes)

## Collection

- `agent-mongo collection list <database> [-c <alias>]` — list collections (name, type)
- `agent-mongo collection schema <database> <collection> [--sample-size <n>] [--depth <n>] [--limit <n>] [--skip <n>] [-c <alias>]` — infer schema from samples (default: 100, configurable via defaults.schemaSampleSize). Errors if collection does not exist. Use --depth to limit nesting, --limit/--skip for field pagination.
- `agent-mongo collection indexes <database> <collection> [-c <alias>]` — list indexes with key patterns
- `agent-mongo collection stats <database> <collection> [-c <alias>]` — collection statistics (document count, sizes, capped)

## Query

- `agent-mongo query find <database> <collection> [--filter <json>] [--sort <json>] [--projection <json>] [--limit <n>] [--skip <n>] [-c <alias>]` — find documents (default sort: `{_id:-1}`, default limit: 20)
- `agent-mongo query get <database> <collection> <id> [--type objectid|string|number] [--projection <json>] [-c <alias>]` — get document by \_id (auto-detects ObjectId). Returns { database, collection, fieldCount, document }.
- `agent-mongo query count <database> <collection> [--filter <json>] [-c <alias>]` — count documents (omit filter for total)
- `agent-mongo query sample <database> <collection> [--size <n>] [--filter <json>] [-c <alias>]` — random documents (default: 5, configurable via defaults.sampleSize). Use --filter to sample from a subset.
- `agent-mongo query distinct <database> <collection> <field> [--filter <json>] [-c <alias>]` — distinct values (supports dot notation)
- `agent-mongo query aggregate <database> <collection> [pipeline] [--pipeline <json>] [--limit <n>] [-c <alias>]` — run aggregation ($out/$merge rejected)

## Usage

- `agent-mongo usage` — LLM-optimized top-level docs
- `agent-mongo <command> usage` — detailed per-command docs:
  - `agent-mongo connection usage`
  - `agent-mongo credential usage`
  - `agent-mongo database usage`
  - `agent-mongo collection usage`
  - `agent-mongo query usage`
  - `agent-mongo config usage`

## Global flags

| Flag                       | Description                              |
| -------------------------- | ---------------------------------------- |
| `-c, --connection <alias>` | Connection alias (overrides env/default) |
| `--expand <field,...>`     | Expand specific truncated fields         |
| `--full`                   | Expand all truncated fields              |

## Config keys

| Key                         | Default | Range       | Description                              |
| --------------------------- | ------- | ----------- | ---------------------------------------- |
| `defaults.limit`            | 20      | 1-1000      | Default result limit for list/query      |
| `defaults.sampleSize`       | 5       | 1-100       | Default sample size for query sample     |
| `defaults.schemaSampleSize` | 100     | 1-1000      | Default sample size for schema inference |
| `query.timeout`             | 30000   | 1000-300000 | Query timeout in ms                      |
| `query.maxDocuments`        | 100     | 1-10000     | Max documents per query                  |
| `truncation.maxLength`      | 200     | 50-100000   | Max string length before truncation      |

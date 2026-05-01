# agent-mongo

Read-only MongoDB CLI for AI agents.

- **Structured JSON output** — all output is JSON to stdout, errors to stderr
- **LLM-optimized** — `agent-mongo usage` prints concise docs for agent consumption
- **Read-only by design** — no write operations, aggregation rejects `$out`/`$merge`
- **Schema inference** — sample documents to discover collection structure
- **Zero runtime deps** — single compiled binary via `bun build --compile`

**Website:** [agent-mongo.paulie.app](https://agent-mongo.paulie.app/)

## Installation

```bash
brew install shhac/tap/agent-mongo
```

### Claude Code / AI agent skill

```bash
npx skills add shhac/agent-mongo
```

This installs the `agent-mongo` skill so Claude Code (and other AI agents) can discover and use `agent-mongo` automatically. See [skills.sh](https://skills.sh) for details.

## Quick start

### 1. Add a connection

```bash
agent-mongo connection add local "mongodb://localhost:27017/myapp"
agent-mongo connection test
```

### 2. Discover structure

```bash
agent-mongo database list
agent-mongo collection list myapp
agent-mongo collection schema myapp users
agent-mongo collection indexes myapp users
```

### 3. Query data

```bash
agent-mongo query find myapp users --filter '{"age":{"$gte":21}}' --limit 10
agent-mongo query get myapp users 665a1b2c3d4e5f6a7b8c9d0e
agent-mongo query count myapp orders --filter '{"status":"pending"}'
agent-mongo query sample myapp users --size 5
agent-mongo query distinct myapp orders status
```

### Extended JSON (EJSON)

All JSON arguments (`--filter`, `--sort`, `--projection`, `--pipeline`) accept MongoDB Extended JSON for BSON types:

```bash
agent-mongo query find myapp events --filter '{"createdAt":{"$gt":{"$date":"2026-01-01T00:00:00Z"}}}'
agent-mongo query find myapp users --filter '{"_id":{"$oid":"665a1b2c3d4e5f6a7b8c9d0e"}}'
```

### Streaming large result sets

`query find` and `query aggregate` support `--stream` for NDJSON output (one JSON object per line). Streaming bypasses the `query.maxDocuments` limit:

```bash
agent-mongo query find myapp events --filter '{"type":"click"}' --stream
agent-mongo query aggregate myapp orders --pipeline '[{"$group":{"_id":"$status","count":{"$sum":1}}}]' --stream
```

## Command map

```text
agent-mongo [-c <alias>] [--full] [--expand <fields>] [--timeout <ms>]
├── connection
│   ├── add <alias> <uri> [--database <db>] [--credential <name>] [--default]
│   ├── update <alias> [--credential <name>] [--no-credential] [--database <db>]
│   ├── remove <alias>
│   ├── list
│   ├── test [alias]
│   ├── set-default <alias>
│   └── usage
├── credential
│   ├── add <name> [--username <user>] [--password <pass>] [--form]
│   ├── remove <name> [--force]
│   ├── list
│   └── usage
├── config
│   ├── get <key>
│   ├── set <key> <value>
│   ├── reset
│   ├── list-keys
│   └── usage
├── database
│   ├── list
│   ├── stats <database>
│   └── usage
├── collection
│   ├── list <database>
│   ├── schema <database> <collection> [--sample-size <n>] [--depth <n>] [--limit <n>] [--skip <n>]
│   ├── indexes <database> <collection>
│   ├── stats <database> <collection>
│   └── usage
├── query
│   ├── find <database> <collection> [--filter] [--sort] [--projection] [--limit] [--skip] [--stream]
│   ├── get <database> <collection> <id> [--type objectid|string|number] [--projection <json>]
│   ├── count <database> <collection> [--filter]
│   ├── sample <database> <collection> [--size <n>] [--filter <json>]
│   ├── distinct <database> <collection> <field> [--filter]
│   ├── aggregate <database> <collection> [pipeline] [--pipeline <json>] [--limit <n>] [--stream]
│   └── usage
└── usage                              # LLM-optimized docs
```

Each top-level command has a `usage` subcommand for detailed, LLM-friendly documentation (e.g., `agent-mongo query usage`). The top-level `agent-mongo usage` gives a broad overview.

## Connection management

Save connection strings locally (stored in `~/.config/agent-mongo/config.json`):

```bash
agent-mongo connection add local "mongodb://localhost:27017/myapp"
agent-mongo connection add staging "mongodb+srv://user:pass@cluster.mongodb.net/staging"
agent-mongo connection set-default staging
agent-mongo connection list
agent-mongo connection test            # pings default connection
agent-mongo connection test staging    # pings specific connection
agent-mongo connection test -c local   # also works with -c flag
```

Or set an environment variable:

```bash
export AGENT_MONGO_CONNECTION="local"  # use a saved alias
```

Connection resolution order: `-c` flag > `AGENT_MONGO_CONNECTION` env > config default.

## Credential management

Store credentials separately from connections. Useful when the same username/password is shared across multiple environments (staging, prod) within an organization:

```bash
# Store a credential once
agent-mongo credential add acme --username deploy --password secret123

# Reference it from multiple connections
agent-mongo connection add acme-staging "mongodb+srv://staging.acme.net/app" --credential acme
agent-mongo connection add acme-prod "mongodb+srv://prod.acme.net/app" --credential acme

# Rotate a password — all connections pick up the change
agent-mongo credential add acme --username deploy --password new-secret

# List credentials (passwords always redacted)
agent-mongo credential list

# Attach/detach credentials from existing connections
agent-mongo connection update prod --credential acme
agent-mongo connection update legacy --no-credential
```

Connections without a `--credential` use the connection string as-is (backward compatible).

### LLM-safe credential entry (`--form`)

When an agent is driving the CLI, putting a password on the command line means the agent sees it. `credential add --form` prompts for any missing `--username` / `--password` via a native OS dialog (osascript on macOS, zenity/kdialog on Linux, PowerShell on Windows). The value is typed directly into the OS popup — agent-mongo only receives the result, and the agent never sees the keystrokes.

```bash
# Both fields prompted via OS dialog
agent-mongo credential add acme --form

# Username on the command line, password prompted
agent-mongo credential add acme --username deploy --form
```

If no GUI session is available (SSH, headless host), the command exits with a structured error: `fixableBy: "human"` and a hint to run on the user's local machine or fall back to non-interactive flags. If the user cancels the dialog, the result is `fixableBy: "retry"`.

## Output

- All output is JSON to stdout
- Errors go to stderr as `{ "error": "..." }` with non-zero exit code
- Empty/null fields are pruned automatically
- Long strings are truncated with companion `*Length` fields

## Truncation

Any string field exceeding `truncation.maxLength` (default 200) is truncated with a `…` suffix. A companion `{field}Length` key shows the full size.

```bash
agent-mongo --full query find myapp users                        # expand all fields
agent-mongo --expand name,bio query find myapp users             # expand specific fields
agent-mongo config set truncation.maxLength 500                  # change default
```

## Safety

agent-mongo is strictly read-only:

- No insert, update, or delete operations
- Aggregation pipelines reject `$out` and `$merge` stages
- Results capped at `query.maxDocuments` (default 100) — use `--stream` to bypass
- Timeout applies to both connections and queries (default 30s), override per-command with `--timeout <ms>`

## Configuration

Persistent settings via `agent-mongo config`:

| Key                         | Default | Description                                  |
| --------------------------- | ------- | -------------------------------------------- |
| `defaults.limit`            | 20      | Default result limit for list/query commands |
| `defaults.sampleSize`       | 5       | Default sample size for query sample         |
| `defaults.schemaSampleSize` | 100     | Default sample size for schema inference     |
| `query.timeout`             | 30000   | Query timeout in milliseconds                |
| `query.maxDocuments`        | 100     | Maximum documents returned per query         |
| `truncation.maxLength`      | 200     | Max string length before truncation          |

```bash
agent-mongo config set defaults.limit 50
agent-mongo config get query.timeout
agent-mongo config list-keys           # all keys with defaults and ranges
agent-mongo config reset               # reset all to defaults
```

## Environment variables

| Variable                 | Description                                      |
| ------------------------ | ------------------------------------------------ |
| `AGENT_MONGO_CONNECTION` | Default connection alias                         |
| `XDG_CONFIG_HOME`        | Override config directory (default: `~/.config`) |

## Development

```bash
bun install
bun run dev -- --help        # run in dev mode
bun run typecheck             # type check
bun test                      # run tests
bun run lint                  # lint
```

## License

MIT

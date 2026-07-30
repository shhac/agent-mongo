# agent-mongo

Read-only MongoDB CLI for AI agents.

- **NDJSON output** — one JSON record per line to stdout, errors to stderr
- **LLM-optimized** — `agent-mongo usage` prints concise docs for agent consumption
- **Read-only by design** — no write operations, aggregation rejects `$out`/`$merge`
- **Schema inference** — sample documents to discover collection structure
- **MCP server** — expose the read-only data commands as MCP tools (`agent-mongo mcp`)
- **Zero runtime deps** — single compiled Go binary

**Website:** [agent-mongo.paulie.app](https://agent-mongo.paulie.app/)

## Installation

```bash
brew install shhac/tap/agent-mongo
```

### Build from source

Requires Go 1.26+.

```bash
make build                       # builds ./agent-mongo (version stamped from git tags)
# or
go build ./cmd/agent-mongo
```

### Claude Code / AI agent skill

```bash
npx skills add shhac/agent-skills --skill agent-mongo --global
```

Installs the `agent-mongo` skill globally so Claude Code (and other AI agents) can discover and use it automatically. It ships from [`shhac/agent-skills`](https://github.com/shhac/agent-skills) — the whole family's skills in one repo, so `npx skills update` checks a single source no matter how many you use. Want several at once? Run `npx skills add shhac/agent-skills --global` and pick from the list.

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

## Output

Default output is **NDJSON** — one JSON record per line on stdout.

- List commands emit one record per item, followed by `@`-prefixed metadata lines: `{"@meta": {...}}` carries command context (database, collection, sample size, totals) and `{"@pagination": {...}}` carries `has_more`, `total_items`, and a `next_cursor` when applicable.
- Single results (stats, `query get`, `count`, `distinct`, write receipts) print as one JSON line.
- Empty/null fields are pruned automatically and object keys are sorted — a missing key means no value, not `null`. `collection indexes` is the exception: index specs are emitted verbatim (see below).
- Errors go to stderr as one JSON line with exit code 1: `{"error": "...", "fixable_by": "agent"|"human"|"retry", "hint": "..."}`. `fixable_by` tells the caller who can resolve it — `agent` (fix the input and retry), `human` (needs the user, e.g. auth or a GUI dialog), or `retry` (transient).

Use `-f/--format` to change the shape:

```bash
agent-mongo database list -f json     # pretty envelope: lists become {"data": [...], ...meta}
agent-mongo query count myapp users -f yaml
```

`-f json` prints a pretty envelope (`{"data": [...]}` for lists, bare pretty object for single results); `-f yaml` prints YAML; `-f jsonl` (the default) is NDJSON.

## Truncation

Any string field exceeding `truncation.maxLength` (default 200) is truncated with a `…` suffix. A companion `{field}Length` key shows the full character count.

```bash
agent-mongo --full query find myapp users                        # expand all fields
agent-mongo --expand name,bio query find myapp users             # expand specific fields
agent-mongo config set truncation.maxLength 500                  # change default
```

## Command map

```text
agent-mongo [-c <alias>] [-f <fmt>] [-F/--full] [-e/--expand <fields>] [-t/--timeout <ms>]
├── connection
│   ├── add <alias> <uri> [--database <db>] [--credential <name>] [--default]
│   ├── update <alias> [--credential <name>] [--clear-credential] [--database <db>]
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
│   ├── find <database> <collection> [--filter] [--sort] [--projection] [--limit] [--skip]
│   ├── get <database> <collection> <id> [--type objectid|string|number] [--projection <json>]
│   ├── count <database> <collection> [--filter]
│   ├── sample <database> <collection> [--size <n>] [--filter <json>]
│   ├── distinct <database> <collection> <field> [--filter]
│   ├── aggregate <database> <collection> [pipeline] [--pipeline <json>] [--limit <n>]
│   └── usage
├── mcp [--http <addr>] [--oauth local] ...    # run as an MCP server
└── usage                                       # LLM-optimized docs
```

Each top-level command has a `usage` subcommand for detailed, LLM-friendly documentation (e.g., `agent-mongo query usage`). The top-level `agent-mongo usage` gives a broad overview.

### Global flags

| Flag                         | Description                                                    |
| ---------------------------- | ------------------------------------------------------------- |
| `-c, --connection <alias>`   | Connection alias (overrides env/default)                      |
| `-f, --format <jsonl\|json\|yaml>` | Output format (default `jsonl` — NDJSON)                 |
| `-e, --expand <field,...>`   | Expand specific truncated fields                              |
| `-F, --full`                 | Expand all truncated fields                                   |
| `-t, --timeout <ms>`         | Request timeout in milliseconds (overrides `query.timeout`)   |
| `-d, --debug`                | Log debug diagnostics to stderr                               |
| `--color <auto\|always\|never>` | Colorize output (default `auto`)                          |

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

A `user:pass` embedded in the URI is automatically moved into a stored
credential named after the connection alias (here: `staging`), so the password
lands in the OS keychain instead of sitting in the connection string in
config.json. Embedding credentials and passing `--credential` at the same time
is an error, and if a credential with that alias already exists with different
values the add is refused rather than silently overwriting it (the error's
hint explains whether to rotate via `credential add --form` or reference the
existing credential with `--credential`). `connection list` always redacts
passwords in connection strings, including connections saved before this
extraction existed.

Or set an environment variable:

```bash
export AGENT_MONGO_CONNECTION="local"  # use a saved alias
```

Connection resolution order: `-c` flag > `AGENT_MONGO_CONNECTION` env > config default.

## Credential management

Store credentials separately from connections. Useful when the same username/password is shared across multiple environments (staging, prod) within an organization:

```bash
# Store a credential once — --form prompts via a native OS dialog, keeping
# the secret out of shell history and agent context (recommended). For a
# non-interactive machine path, pipe the password on stdin instead (below).
agent-mongo credential add acme --form

# Reference it from multiple connections
agent-mongo connection add acme-staging "mongodb+srv://staging.acme.net/app" --credential acme
agent-mongo connection add acme-prod "mongodb+srv://prod.acme.net/app" --credential acme

# Rotate a password — all connections pick up the change
agent-mongo credential add acme --form

# List credentials (passwords always redacted)
agent-mongo credential list

# Attach/detach credentials from existing connections
agent-mongo connection update prod --credential acme
agent-mongo connection update legacy --clear-credential
```

Connections without a `--credential` use the connection string as-is (backward compatible).

Credentials are stored in the OS secret store when available (macOS Keychain, Linux Secret Service, Windows Credential Manager) and fall back to plaintext config otherwise. `credential list` shows the `storage` source (`keychain` or `config`) per credential. Set `AGENT_MONGO_NO_KEYCHAIN=1` to force plaintext config storage. Plaintext credentials (from older versions or keychain-less hosts) are upgraded to the keychain automatically the first time they are used on a host with a usable keychain.

### Secure credential entry — never paste a secret into `--password`

A literal secret on the command line lands in shell history, `ps`/`/proc`, and — when an agent is driving the CLI — the agent's context and transcripts. Do not put a pasted password into `--password`. Two paths supply the secret without ever placing it on argv:

**`--form` — preferred interactive path.** Prompts for any missing `--username` / `--password` via a native OS dialog (osascript on macOS, zenity/kdialog on Linux, PowerShell on Windows). The value is typed directly into the OS popup — agent-mongo only receives the result, and the agent never sees the keystrokes.

```bash
# Both fields prompted via OS dialog
agent-mongo credential add acme --form

# Username on the command line (not a secret), password prompted
agent-mongo credential add acme --username deploy --form
```

**Piped stdin — non-interactive machine path.** For scripts, CI, or a headless host with no GUI, pipe the password on stdin. It is read off the stream, never placed on the command line:

```bash
printf '%s' "$PW" | agent-mongo credential add acme --username deploy
```

Password resolution precedence: `--password` flag > piped stdin > `--form` dialog. Reserve `--password` for values that are already non-secret; use `--form` or stdin for anything sensitive.

If no GUI session is available (SSH, headless host), `--form` exits with a structured error: `fixable_by: "human"` and a hint to run on the user's local machine or use the piped-stdin path. If the user cancels the dialog, the result is `fixable_by: "retry"`.

## MCP server

`agent-mongo mcp` runs the read-only data commands (`database`, `collection`, `query`, `connection`) as [Model Context Protocol](https://modelcontextprotocol.io) tools. Credential and config commands are not exposed.

```bash
# stdio transport (launched by an MCP client)
agent-mongo mcp

# register with Claude Code
claude mcp add agent-mongo -- agent-mongo mcp

# Streamable HTTP transport
agent-mongo mcp --http :8000
```

The HTTP transport is unauthenticated unless `--oauth local` is set (self-contained OAuth 2.1) — bind to loopback or front it with an auth proxy. Run `agent-mongo mcp usage` for the full transport, registration, OAuth, and Tailscale details.

## Safety

agent-mongo is strictly read-only:

- No insert, update, or delete operations
- Aggregation pipelines reject `$out` and `$merge` stages
- Results capped at `query.maxDocuments` (default 100)
- Timeout applies to both connections and queries (default 30s), override per-command with `-t/--timeout <ms>`

## Configuration

Persistent settings via `agent-mongo config`:

| Key                         | Default | Range       | Description                                  |
| --------------------------- | ------- | ----------- | -------------------------------------------- |
| `defaults.limit`            | 20      | 1-1000      | Default result limit for list/query commands |
| `defaults.sampleSize`       | 5       | 1-100       | Default sample size for query sample         |
| `defaults.schemaSampleSize` | 100     | 1-1000      | Default sample size for schema inference     |
| `query.timeout`             | 30000   | 1000-300000 | Query timeout in milliseconds                |
| `query.maxDocuments`        | 100     | 1-10000     | Maximum documents returned per query         |
| `truncation.maxLength`      | 200     | 50-100000   | Max string length before truncation          |

```bash
agent-mongo config set defaults.limit 50
agent-mongo config get query.timeout
agent-mongo config list-keys           # all keys with defaults and ranges
agent-mongo config reset               # reset all to defaults
```

## Environment variables

| Variable                  | Description                                              |
| ------------------------- | ------------------------------------------------------- |
| `AGENT_MONGO_CONNECTION`  | Default connection alias                                |
| `AGENT_MONGO_NO_KEYCHAIN` | Set to `1` to store credentials in plaintext config     |
| `XDG_CONFIG_HOME`         | Override config directory (default: `~/.config`)        |

## Development

```bash
make dev ARGS="database list"    # go run ./cmd/agent-mongo database list
make test                         # go test ./...
make test-integration             # integration tests against a dockerised mongod
make lint                         # golangci-lint
make build                        # build the binary
```

The version string is stamped at build time from git tags via `-ldflags` (`git describe --tags`), so a built binary reports its release; a plain `go build` reports `dev`.

## License

MIT

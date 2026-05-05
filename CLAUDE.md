# agent-mongo

Read-only MongoDB CLI for AI agents. TypeScript + Bun, compiled to standalone binaries.

## Architecture

```
src/
├── index.ts                     # CLI entry — assembles the root vipvot Command and dispatches
├── cli/
│   ├── connection/              # connection add/remove/update/list/test/set-default
│   ├── credential/              # credential add/remove/list
│   ├── config/                  # config get/set/reset/list-keys
│   ├── database/                # database list/stats
│   ├── collection/              # collection list/schema/indexes/stats
│   ├── query/                   # query find/get/count/sample/distinct/aggregate
│   └── usage/                   # LLM-optimized top-level usage text
├── lib/
│   ├── config.ts                # ~/.config/agent-mongo/ config + connection + credential storage
│   ├── output.ts                # printJson, printJsonRaw, printPaginated, printNdjsonStream, printError
│   ├── compact-json.ts          # pruneEmpty() — strips null/empty/blank-string fields
│   ├── dialog/                  # native OS dialog wrapper for LLM-safe secret entry
│   │   ├── index.ts             #   Prompter interface, sentinels, classifyError, setDefault
│   │   ├── spawn-backend.ts     #   Bun.spawn → osascript / zenity / kdialog / PowerShell
│   │   └── available.ts         #   per-platform GUI availability pre-flight
│   ├── errors.ts                # enhanceErrorMessage — timeout hints, index suggestions
│   ├── keychain.ts              # macOS keychain read/write/delete for credentials
│   ├── parse-json.ts            # EJSON-aware JSON parsing for --filter, --sort, --projection, --pipeline
│   ├── serialize.ts             # BSON → JSON-safe conversion (ObjectId, Date, Binary, Long, etc.)
│   ├── timeout.ts               # CLI --timeout override + getTimeout() helper
│   ├── truncation.ts            # Generic string truncation with {field}Length companion
│   └── version.ts               # Version from build-time define / env / package.json
└── mongo/
    ├── client.ts                # MongoClient factory (alias resolution, connection pool)
    ├── databases.ts             # listDatabases, getDatabaseStats
    ├── collections.ts           # listCollections, getCollectionStats, validateCollectionExists
    ├── schema.ts                # inferSchema — sample-based field/type discovery
    ├── indexes.ts               # listIndexes
    ├── query.ts                 # findDocuments, streamFind, findById, countDocuments, getDistinctValues
    ├── aggregate.ts             # runAggregate, streamAggregate with $out/$merge rejection
```

## Key patterns

- **Command registration**: Each `cli/*/index.ts` exports `buildXyzCommand(): Command` returning a constructed vipvot `Command` with its leaves attached. `src/index.ts` builds the root and `addCommand`s each group. Each leaf file exports a `buildXCommand(): Command` factory.
- **Persistent flags / globals**: `cli/_globals.ts` owns module-scoped `Ref`s for the persistent flags (`-c/--connection`, `--expand`, `--full`, `--timeout`) plus the `applyGlobals` `persistentPreRun` handler. Subcommands import `resolveConnectionAlias()` rather than walking the parent chain — the cobra-idiomatic pattern.
- **Output**: All commands use `printJson()` or `printPaginated()` from `lib/output.ts`. Admin/config commands use `printJsonRaw()` (no truncation). Errors use `printError()`. All output is JSON, empty/null fields auto-pruned via `pruneEmpty()`.
- **Truncation**: Any string field exceeding `truncation.maxLength` gets truncated with `…` and a companion `{field}Length` key. Controlled by `--expand` and `--full` global flags. Unlike lin (which only truncates preset field names), this truncates any string over the limit.
- **Connection resolution**: `-c` flag > `AGENT_MONGO_CONNECTION` env > config default > error listing available connections. Connections can reference named credentials for shared auth.
- **Credential separation**: Credentials (username/password) stored separately from connections (host/options). Connections reference credentials by name via `credential` field. Backward compatible — connections without a credential use the URI as-is.
- **Error messages**: Include valid values so LLMs can self-correct (e.g., `Connection "x" not found. Available: local, staging`).
- **BSON serialization**: `mongo/serialize.ts` converts all BSON types to JSON-safe values before output. ObjectId → hex string, Date → ISO string, Binary → base64, Long → number (if safe) or string, Decimal128 → string, UUID → string, RegExp → string.
- **Read-only safety**: No write operations exist. `mongo/aggregate.ts` rejects `$out`/`$merge` stages. Results capped at `query.maxDocuments`.
- **Usage subcommands**: Each command group has a `usage` subcommand providing LLM-friendly docs. When modifying a command's behavior, options, or flags, update its usage text too.
- **Collection validation**: `mongo/collections.ts` validates collection existence before operations like schema inference. Error messages include a hint to list available collections.
- **Timeout**: `lib/timeout.ts` provides `getTimeout()` — checks CLI `--timeout` override first, then config `query.timeout`, then default 30s. All mongo operations use this helper.
- **Timeout hints**: `lib/errors.ts` detects MongoDB timeout errors (code 50) and enhances messages with `--timeout` flag and config suggestions.
- **Config validation**: `cli/config/valid-keys.ts` defines all valid keys with types, defaults, and min/max ranges. Invalid keys or out-of-range values produce errors listing valid options.
- **LLM-safe secret entry**: `lib/dialog/` is a self-contained wrapper. Consumers (e.g. `cli/credential/form.ts`) import only `lib/dialog/index.ts` — never the spawn backend or platform availability files directly. Replacing the backend (e.g. with an Electron sidecar) is a one-file swap. The `Prompter` interface, `setDefault()` test hook, and sentinel→category mapping (`classifyError`) mirror agent-sql's `internal/dialog` package.

## Commands

Run `bun run dev -- usage` for the full command reference. Each command also supports `<command> usage` for detailed per-command docs.

## Development

```bash
bun install
bun run dev -- <command>     # run in dev mode
bun run typecheck            # tsc --noEmit
bun test                     # bun:test
bun run lint                 # oxlint
bun run format               # oxfmt
```

## Release

```bash
bun run release patch        # bumps version, commits, tags, pushes
bun run build:release        # cross-platform binaries in release/
```

Then create GitHub release and update homebrew-tap formula with new sha256s.

## Keeping docs in sync

- **Skill** (`skills/agent-mongo/SKILL.md`): The Claude Code skill that agents use to discover and operate `agent-mongo`. Update it when commands, flags, output shapes, or usage patterns change.
- **README** (`README.md`): The public-facing docs. Update the command map, config table, and examples when commands or behavior change.
- **Release steps**: Scripts like `bun run release` and `bun run build:release` may change your working directory. After running release steps, verify your `pwd` before operating on files — paths will be relative to wherever you end up.

## Conventions

- TypeScript strict mode, ES2022 target, Bun bundler resolution
- `type` over `interface` (enforced by oxlint)
- kebab-case filenames (enforced by oxlint)
- Max 350 lines per file, max 2 params per function (oxlint warnings)
- Pre-commit hook: oxlint fix + oxfmt
- Tests: bun:test, no mocking libraries, inline fixtures, pure functions preferred

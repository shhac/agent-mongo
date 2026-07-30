# agent-mongo

Read-only MongoDB CLI for AI agents. Go + cobra on the `lib-agent-*` family
libraries, compiled to standalone binaries.

## Architecture

```
cmd/agent-mongo/main.go        # entry point; version injected via -ldflags
internal/
├── cli/
│   ├── root.go                # lib-agent-cli NewRoot + domain flags -c/--expand/--full
│   ├── usage.go               # top-level LLM reference card
│   ├── conntest.go            # `connection test` (kept here for the driver dep)
│   ├── mcp.go                 # `mcp` server via lib-agent-mcp (registered last)
│   ├── shared/                # GlobalFlags DTO, WithSession, defaults, RegisterUsage
│   ├── connection/            # connection add/remove/update/list/set-default
│   ├── credential/            # credential add (--form dialog)/remove/list
│   ├── configcmd/             # config get/set/reset/list-keys — keyDef table
│   ├── database/              # database list/stats
│   ├── collection/            # collection list/schema/indexes/stats
│   └── query/                 # query find/get/count/sample/distinct/aggregate
├── config/                    # ~/.config/agent-mongo/config.json I/O + settings
├── credential/                # __KEYCHAIN__ sentinel store via lib-agent-keyring
├── mongo/                     # client factory, databases, collections, indexes,
│                              #   schema inference, query, aggregate
├── mongouri/                  # driver-free connection-string parsing:
│                              #   db-name extraction, credential split, redaction
├── serialize/                 # BSON → JSON-safe (ObjectId→hex, Date→ISO, …)
├── truncation/                # any-string truncation + {field}Length companion
├── ejson/                     # Extended JSON parsing for --filter/--sort/--pipeline
├── errors/                    # fixable_by classification + timeout hints
├── output/                    # lib-agent-output adapters (PrintResult/PrintRaw/PrintList)
└── integration/               # end-to-end tests against a dockerised mongod
```

## Key patterns

- **Family libraries**: `lib-agent-cli` (root scaffolding, XDG paths, creds
  store, secret dialog), `lib-agent-output` (NDJSON wire contract, errors,
  pruning), `lib-agent-keyring` (OS secret store), `lib-agent-mcp` (`mcp`
  command). Prefer these over hand-rolling; agent-sql and lin are the sibling
  reference implementations.
- **Command registration**: each group package exports
  `Register(root, globals)`; leaves read live global flags through the
  `func() *shared.GlobalFlags` closure — never by walking the cobra parent
  chain. The MCP command is registered last so it reflects the full tree.
- **Output contract**: NDJSON records on stdout; list metadata rides on
  `@`-prefixed lines (`@meta`, `@pagination`); `-f json`/`-f yaml` produce a
  `{"data": [...]}` envelope. Data-bearing output goes through
  `output.PrintResult`/`PrintList` (prune + truncate); admin receipts through
  `output.PrintRaw` (prune only). Records whose encoded shape *is* the data —
  index specs — go through `output.PrintListVerbatim` (no prune, no truncate,
  field order kept) and implement `output.Verbatim`; the normalizing printers
  reject them rather than silently re-sorting keys and dropping nulls.
  Errors: return them from `RunE` — libcli.Run
  renders `{error, fixable_by, hint}` on stderr once, exit 1. Never pre-print
  errors in commands.
- **Truncation**: any string over `truncation.maxLength` is cut with `…` plus a
  `{field}Length` companion key. Configured process-wide from the root
  pre-run (`--expand`, `--full`, config), applied inside the output helpers.
- **Connection resolution**: `-c` flag > `AGENT_MONGO_CONNECTION` env > config
  default > error listing available aliases. Connections reference named
  credentials; `__KEYCHAIN__` sentinel in config.json means the real values
  live in the OS keychain (service `app.paulie.agent-mongo`, accounts
  `username:<alias>` / `password:<alias>`).
- **Error messages include valid values** so LLMs can self-correct (e.g.
  `Connection "x" not found. Available: local, staging`).
- **BSON serialization** (`internal/serialize`): ObjectId → hex, Date →
  ISO-8601, Binary → base64 (UUID subtype → uuid string), int64 → number when
  ≤2^53-1 else string, Decimal128 → string, Regex → `/pattern/flags`.
- **Read-only safety**: no write operations; `$out`/`$merge` rejected in
  pipelines; results capped at `query.maxDocuments`.
- **Timeouts**: `-t/--timeout` (ms) > config `query.timeout` > 30s, applied as
  the driver's client-level CSOT (`SetTimeout`); per-command contexts get a 5s
  grace so server-side timeout errors (better hints) fire first.
- **Usage subcommands**: every group has an LLM-optimized `usage` leaf. When
  changing a command's behavior, options, or output shape, update its
  usageText and the top-level card in `internal/cli/usage.go`.
- **LLM-safe secret entry**: `credential add --form` uses
  `lib-agent-cli/dialog`; tests swap the prompter via
  `dialog.SetDefault(&dialogtest.Recorder{...})`.

## Commands

Run `make dev ARGS=usage` for the full command reference. Each command group
also supports `<group> usage` for detailed per-command docs.

## Development

```bash
make build                   # build ./agent-mongo (version from git describe)
make dev ARGS="usage"        # go run
make test                    # unit tests
make test-integration        # end-to-end vs throwaway docker mongo:8
make lint                    # golangci-lint
```

Tests are table-driven stdlib `testing`, no mocking libraries. Config-touching
tests isolate with `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` and
`t.Setenv("AGENT_MONGO_NO_KEYCHAIN", "1")`. Integration tests seed and drop
their own database — never point them at real data.

## Release

```bash
git tag vX.Y.Z && git push origin main vX.Y.Z
```

The tag push triggers `.github/workflows/release.yml` (shared
`shhac/homebrew-tap` go-release workflow): cross-builds, GitHub Release, and
Homebrew formula update. No version file — `main.version` comes from ldflags.

## Keeping docs in sync

- **Skill** (`skills/agent-mongo/SKILL.md` + `references/`): what agents use to
  operate the CLI. Update when commands, flags, or output shapes change.
- **README** (`README.md`): public docs — command map, config table, examples.
- **Design docs** (`design-docs/`): tracked; stale/internal notes go to
  `design-docs/archive/` (gitignored).

## Conventions

- Go 1.26, gofmt (pre-commit hook), `go vet` clean
- Errors bubble; only the top-level renders them
- Package names: domain packages are nouns (`serialize`, `truncation`);
  `internal/cli/*` mirrors the command tree

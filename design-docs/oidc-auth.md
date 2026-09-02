# MONGODB-OIDC authentication

**Status: COMPLETE** — all four phases shipped. The design below is what was
built; where the implementation diverged from the plan, the reason is recorded
inline. Reviewed against the driver source before building, and after each
phase by a structural pass.

## Why

An enterprise that hands out database access does not want to hand out database
passwords. It wants what it already runs for every other tool: users
authenticate against the corporate IdP, group membership decides what they can
read, and access is revoked centrally by removing someone from a group. MongoDB
7.0+ supports this as `MONGODB-OIDC` (Atlas M10+, Workforce and Workload
Identity Federation).

For agent-mongo this changes the security posture more than it would for a
human-driven CLI:

- Today an agent operates a shared SCRAM user whose password never expires. A
  leaked config, transcript, or laptop leaks indefinite read access to
  production.
- Under OIDC the agent holds a token that dies in about an hour, cannot mint a
  new one without a human completing a login, and inherits exactly the database
  roles the human's IdP groups map to.
- Atlas audit logs attribute every query to a real person instead of `agent_ro`.

The interactive login becomes a human-in-the-loop gate on a time-boxed window of
agent access.

## The model

OIDC is not a variant of password auth. For a whole class of user it is the only
auth they will use, so the credential model gets a discriminator and neither
kind is privileged.

### Credential kinds

`config.Credential` (`internal/config/config.go:17`) grows a `kind` field naming
a MongoDB auth mechanism:

| `kind` | Mechanism | Stored material |
| --- | --- | --- |
| `scram` | `SCRAM-SHA-256` | username + password |
| `oidc` | `MONGODB-OIDC` | a flow recipe, and for `device` a session |

A missing `kind` reads as `scram`. `x509` and `gssapi` slot into the same table
later without renaming anything, because these are the names the driver accepts
in `options.Credential.AuthMechanism` (`mongo/options/clientoptions.go:110-119`).

### The credential holds a recipe

An OIDC credential answers one question: *how do I get a session, and a new one
when this expires?* That is `flow`.

| `flow.type` | How a session is obtained | Renewal | Session stored |
| --- | --- | --- | --- |
| `environment` | the driver reads the platform identity (`ENVIRONMENT: k8s\|azure\|gcp`) | automatic, every process | no |
| `file` | read a JWT the platform already wrote to disk | re-read the file | no |
| `device` | RFC 8628 device-code against the IdP | agent-mongo refreshes with its own stored refresh token; human required when that dies | yes |

The split that matters to an agent is the last two columns. `environment` and
`file` renew inside any ordinary command and never block. `device` is the only
one that can hard-stop and demand a human.

**There is no `command` flow.** An earlier draft had one that exec'd a
configured argv. It is redundant: the driver's built-in `k8s` provider already
reads `AZURE_FEDERATED_TOKEN_FILE`, then `AWS_WEB_IDENTITY_TOKEN_FILE`, then the
GKE default path (`x/mongo/driver/auth/oidc.go:377-395`), which covers EKS/IRSA,
AKS workload identity, and GKE. What remained was arbitrary persistent process
execution configured by a file an agent can write — a large safety-surface
expansion for cases `file` covers without exec.

### Config shape

```json
{
  "credentials": {
    "local": {
      "kind": "scram",
      "username": "__KEYCHAIN__",
      "password": "__KEYCHAIN__"
    },
    "corp": {
      "kind": "oidc",
      "flow": { "type": "device" },
      "session": "__KEYCHAIN__"
    },
    "ci": {
      "kind": "oidc",
      "flow": { "type": "environment", "environment": "k8s" }
    },
    "azfn": {
      "kind": "oidc",
      "flow": {
        "type": "environment",
        "environment": "azure",
        "token_resource": "api://mongodb-atlas",
        "client_id": "<azure managed identity client id>"
      }
    },
    "eks-manual": {
      "kind": "oidc",
      "flow": { "type": "file", "path": "/var/run/secrets/eks.amazonaws.com/serviceaccount/token" }
    }
  }
}
```

The `device` flow stores **no issuer and no client ID**. The driver's human flow
is two-step on a fresh process: with no cached `idpInfo` it asks the *server* for
`IDPInfo` (issuer, client ID, request scopes) and passes it to the callback
(`x/mongo/driver/auth/oidc.go:545-556`). Atlas is the source of truth for the
federation config, so putting a mutable copy in a file an agent can write only
creates a way to point the login at the wrong IdP.

`connections` are unchanged: a connection names a credential alias, and one OIDC
credential can back several connections. Nothing about auth moves onto
`config.Connection` (`config.go:22`) — one home for identity, not two.

## What the driver actually does

This section exists because the first draft of this design got it wrong.

**The driver never refreshes on agent-mongo's behalf.** `getAccessToken`
(`x/mongo/driver/auth/oidc.go:399-445`) passes `OIDCArgs.RefreshToken` to the
callback only when the authenticator already holds a refresh token *returned by
an earlier callback in the same `Client`*. agent-mongo forks a process per
command, so on every invocation `oa.refreshToken` is nil and the callback is
called with `RefreshToken: nil`.

**The driver never looks at `ExpiresAt`.** `OIDCCredential.ExpiresAt`
(`clientoptions.go:133-137`) is accepted and ignored; the access token is cached
as a string until authentication fails or the server sends a 391 reauth
(`oidc.go:489-505`). Returning an expired token produces a failed SASL
conversation, not a retry with a refresh token.

Both facts collapse to one rule: **agent-mongo owns expiry and refresh
entirely.** The callback reads the stored session, and if `expires_at` has passed
it performs the OAuth refresh against the IdP itself, persists the new session,
and returns the fresh access token.

**The callback must be non-interactive and idempotent.** The driver may call it
twice in one authentication: once with a refresh token, and again with
`RefreshToken: nil` if that attempt errors (`oidc.go:414-446`). An interactive
prompt in the callback would fire twice.

**Speculative auth is not a shortcut.** `CreateSpeculativeConversation`
(`oidc.go:585-595`) returns nil unless an access token is already cached in
process, so it never front-runs the callback.

**Timeouts are the smaller of two bounds.** `doAuthHuman` wraps the operation
context in `humanCallbackTimeout` (5 minutes, `oidc.go:531`), but agent-mongo
gives a command `Timeout() + 5s` (`internal/cli/shared/shared.go:49`), default
35 seconds. The effective bound on an interactive login inside a query command
is ~35s, not five minutes. This is a second reason login is its own command.

Other driver constraints (`mongo/options/clientoptions.go:615-665`):

- a password must not be set for `MONGODB-OIDC` (`:618`)
- machine and human callbacks are mutually exclusive (`:620`)
- `ALLOWED_HOSTS` may only be set alongside a human callback (`:623`)
- exactly one of a callback or `ENVIRONMENT` is required (`:626`)
- `TOKEN_RESOURCE` is required for `azure` and `gcp` (`:643-644`)
- a username is rejected only for `ENVIRONMENT: test` (`:661-666`); `azure` uses
  it as the managed-identity client ID, `gcp` and `k8s` ignore it
- `authSource` must be empty or `$external`, enforced at
  `x/mongo/driver/auth/oidc.go:127-130` and, for URI-configured OIDC, at
  `x/mongo/driver/connstring/connstring.go:298-302`

MongoDB documents driver OIDC support as Linux-qualified in v2.7.0
(`mongo/client_examples_test.go:464-466`). Integration coverage should not
assume macOS parity.

## Login is its own command

Given the above, the split is forced:

- `credential login <alias>` performs the device-code flow once and writes the
  session to the keychain. It connects to a real cluster so the server supplies
  `IDPInfo`, and it runs without the query timeout.
- Every other command installs an `OIDCHumanCallback` that only reads and
  refreshes. It never prompts.
- A dead session is an ordinary self-correcting error, never a prompt:
  `Session for credential "corp" has expired. Run: agent-mongo credential login corp`.

Because login needs a server to ask for `IDPInfo`, it takes a connection:
`credential login corp --connection prod`, defaulting to the sole connection
referencing the credential and erroring with the list when ambiguous.

Device code rather than an auth-code browser redirect, because agent-mongo runs
over SSH, in containers, and inside the `mcp` server where there is no browser
and no loopback listener. The user code and verification URL are emitted as an
`@notice` record so an agent can relay them into chat.

Device code also removes the "remote host" objection entirely: the human never
needs to be at the remote machine's screen. They run the command that prints the
code and complete the authorization on whatever device they are already holding.

**Login never fires implicitly, not even at an interactive TTY.** The agent
driving agent-mongo may know that no human is present, and a blocking prompt
takes away its ability to choose a different action. A missing or expired session
is an ordinary error classified `fixable_by: human` — the classification
`internal/errors` already applies to auth failures — which is exactly the signal
that the agent cannot resolve it alone.

## Session lifetime and login cadence

Login is not a per-invocation cost. Two clocks run:

- the **access token** expires on the IdP's schedule, commonly around an hour
- the **refresh token** lives far longer, days to months, sliding on some IdPs

Between them every ordinary command refreshes silently inside the callback and
never prompts. A human is needed only when the refresh token expires or is
revoked, so the realistic cadence is weeks. Those figures are IdP configuration
and not something agent-mongo can promise, so `credential list` and
`connection test` report the session's actual `expires_at` rather than a claimed
policy.

`connection test` reports session state on both paths, because a session can be
revoked while still unexpired and only the server knows:

- ping succeeds: `ok: true` plus the session's `expires_at` in the receipt
- ping fails, credential is `oidc`, session missing or expired:
  `fixable_by: human`, hint `Run: agent-mongo credential login <alias>`

## Session storage

One keychain entry, account `session:<alias>`, service `app.paulie.agent-mongo`,
holding a JSON blob:

```json
{ "access_token": "...", "refresh_token": "...", "expires_at": "2026-09-02T15:04:05Z",
  "issuer": "...", "host": "cluster0.abc.mongodb.net" }
```

One entry rather than three accounts, because a refresh replaces every field at
once and a single write cannot leave a half-refreshed session behind. This is
narrower than the claim in the first draft: it does **not** make the keychain and
config.json atomic with respect to each other. `Store`
(`internal/credential/credential.go:116-141`) writes keychain entries inside
`config.Update`'s critical section, but they remain two systems, and a failed
config save can still orphan a keychain value. That hazard is pre-existing and
unchanged.

`session: "__KEYCHAIN__"` mirrors the existing sentinel, but the sentinel
machinery is **not** generic: `usernameAccount`/`passwordAccount`
(`credential.go:39-40`) are the only account constructors, `Get`
(`credential.go:45`) resolves only those two fields, and `Remove`
(`credential.go:183-206`) deletes only those two accounts. A session account
needs explicit read, write, remove, and `StorageType` support, or `logout` and
`credential remove` leave live bearer tokens in the keychain.

Precedent for keychain-held OAuth material already exists in the tree:
`registerMCP` passes `WithOAuthKeyringService(credential.Service + ".mcp")`
(`internal/cli/mcp.go:31`).

## Security

**`ALLOWED_HOSTS` does not establish the trust boundary.** The first draft
claimed it did. The driver's default allowlist
(`x/mongo/driver/auth/oidc.go:70-79`) includes `localhost`, `127.0.0.1` and
`::1`, plus broad MongoDB-owned suffixes, and the check runs only before the
human callback is invoked (`oidc.go:525-531`). It does not bind a token to a
server identity. The `connection` group is exposed over MCP
(`internal/cli/mcp.go:16`), so an agent can add a connection pointing at a
malicious local MongoDB-speaking listener and be handed a valid IdP token.

The binding has to be agent-mongo's own:

- a session records the `issuer` and the connection `host` it was obtained for
- the callback refuses to present a session to a host that does not match, with
  a `fixable_by` error naming the mismatch
- `ALLOWED_HOSTS` stays at the driver default and is never passed to the driver

**The driver's gap is wider than the human flow.** `validateConnectionAddressWithAllowedHosts`
runs only inside `doAuthHuman` (`x/mongo/driver/auth/oidc.go:525`), so a *machine*
flow will hand a platform identity token to whatever host the connection string
names, with no check at all. agent-mongo therefore applies the allowlist itself,
to every OIDC flow, at both `connection add`/`update` and connect.

`credential add --oidc` accepts `--allowed-hosts` to widen it. An earlier draft
of this document said an override must never sit on a flag an agent can pass,
and that is right for the device flow but wrong for the machine flows, for a
reason worth writing down: with `environment`, the token is *already* reachable
by anything that can run agent-mongo — a projected Kubernetes token is a file on
disk, and Azure and GCP identities are one IMDS request away. An agent that
could abuse the flag could read the token directly and skip agent-mongo
entirely. The allowlist is defence in depth there, and a self-hosted Enterprise
deployment is a real case that needs it widened.

The device flow is the opposite: its refresh token lives in the OS keychain,
which is exactly the material an agent cannot otherwise obtain. Its session
binding is therefore recorded at login and is **not** overridable by a flag.

**OIDC requires TLS.** The driver applies no TLS requirement for OIDC, and
`clientOptions` (`internal/mongo/client.go:68-88`) passes the stored URI through
unchanged. A bearer token over a plaintext connection is a credential on the
wire. `connection add` and connect-time both reject an OIDC credential paired
with a connection string that is neither `mongodb+srv://` nor explicitly
`tls=true`.

**Tokens are logged nowhere.** No token reaches stdout, an error message, or
`--echo-query`. The leak test at `internal/cli/credential/form_test.go:190` is
the model for the regression test.

**`credential` stays out of MCP.** `registerMCP` already skips it
(`internal/cli/mcp.go:17`); `login` and `logout` must stay skipped, so completing
an auth window remains a deliberate human act at a terminal.

**The stdio MCP server holds no session in memory.** Each tool call already
builds and closes its own client through `shared.WithSession`
(`internal/cli/shared/session.go:17-38`), a keychain read is cheap, and a
resident process holding a live bearer token undercuts the security story that
motivates this feature. Re-read per call.

A *remote* MCP host, where no one has a shell on the machine, is a separate
problem and out of scope here. The seam already exists: `WithIdentityBinding`
(`lib-agent-mcp/options.go:112-118`) maps an OAuth-validated MCP principal onto
the subprocess argv and env, and fires only for calls carrying such a principal.
The right shape there is a session per principal rather than one shared session,
and it wants its own design doc.

## What in the current code blocks this

These are correctness blockers, not cleanup. Each one silently corrupts an OIDC
credential if left as-is.

1. **`Store` discards unknown fields.** On the keychain path it writes a freshly
   constructed `config.Credential{Username: sentinel, Password: sentinel}`
   (`internal/credential/credential.go:127`), dropping `kind`, `flow` and
   `session` on the floor. Storing an OIDC credential is impossible until this
   preserves the rest of the struct.
2. **`Get` misclassifies an OIDC entry as plaintext SCRAM.** An OIDC credential
   has empty username and password, so `cred.Username != sentinel &&
   cred.Password != sentinel` holds (`credential.go:49`), and `maybeUpgrade`
   (`credential.go:71-83`) calls the destructive `Store` above. Resolution must
   branch on kind before this test.
3. **`Get` returns `(config.Credential, bool)`** (`credential.go:45`). Two
   outcomes cannot express: valid session, expired but refreshable, expired and
   needs a human, or recipe present but never logged in.
4. **`Require` means "username and password both resolve"**
   (`credential.go:163-169`). `connection add --credential`
   (`internal/cli/connection/add.go:54`) and `connection update`
   (`internal/cli/connection/update.go:22`) call it, so they would reject a valid
   `environment` credential and a not-yet-logged-in `device` one. It splits into
   "the recipe exists" (validation) and "a session resolves" (connect).
5. **`credential list` is SCRAM-shaped**: it reads `Username`, hardcodes the
   `__KEYCHAIN__` literal rather than the constant, and always emits
   `password: "***"` (`internal/cli/credential/credential.go:60-85`). It needs
   `kind`, `flow.type` and an expiry, and must never print token material.
6. **The not-found hint hardcodes `--form`** (`credential.go:176`, asserted at
   `form_test.go:158`). For OIDC there is no secret to type, so the hint stops
   being self-correcting for exactly the user this feature serves.
7. **`credential add` requires username and password**
   (`internal/cli/credential/add.go:26-50`). The OIDC path supplies neither.

### Round-trip and downgrade

`config.Read` (`internal/config/config.go:63-71`) unmarshals with no validation
and treats a decode error as an empty config that a later write can overwrite
(`config.go:97-104`). Two consequences the design has to accept explicitly:

- A malformed `flow` fails worse than a malformed credential does today, because
  the whole config reads as empty rather than one entry being wrong. Validate
  `flow` at write time in `credential add`, and treat an unknown `kind` or
  `flow.type` at read time as an unusable credential with a named error, not as
  a parse failure.
- **An older binary destroys OIDC state.** `encoding/json` drops unknown fields
  on unmarshal and `Save` re-serializes the reduced struct, so any config write
  by a pre-OIDC build deletes `kind`, `flow`, and a config-backed `session`. This
  is why the credential-model change ships as its own release ahead of any flow:
  the fleet upgrades before there is state to lose.

## Phasing

All four landed, in this order.

1. **Credential model** (`3636b68`, restructured in `dbf20ab`). `kind` added
   with `scram` as what an absent kind reads as. Two bugs blocked any second
   kind: `Store` rebuilt the entry from scratch on the keychain path, dropping
   every field it did not name, and `Get` classified an entry with an empty
   username and password as plaintext SCRAM and sent it through the keychain
   upgrade that overwrites both. `Get` became `Resolve`, and `Require` split so
   the connection commands stopped demanding a credential authenticate at the
   moment it is wired up.
2. **`flow.type: environment`** (`5a8ff7f`). Driver passthrough, plus the two
   guards the driver does not supply: TLS, and the allowlist the driver applies
   only to the human flow.
3. **`flow.type: file`** (`9a78011`). A JWT another tool wrote, re-read at every
   authentication so a rotated token is picked up.
4. **`flow.type: device`** (`2750923`), on the protocol client added in
   `929da37`. Device-code login, a keychain-backed session, agent-mongo-owned
   refresh, and host binding.

### What the plan got wrong

- **The `Get` refactor was scheduled after the phases that needed it.** Moved to
  first; phases 2 and 3 could not have shipped without it.
- **`Resolution` shipped with a `State` enum alongside its error**, two channels
  for one fact that had already drifted — a failed resolve handed back a
  credential still holding the sentinel. No production code read `State`;
  removed in favour of wrapped sentinel errors.
- **The allowlist was described as the driver's job.** It is not: the driver
  checks it for the human flow only, so agent-mongo applies it to every flow.
- **`--allowed-hosts` was ruled out entirely.** It is right for the flows whose
  token is already reachable by anything that can run agent-mongo, and wrong for
  the device flow. Gated on the flow rather than banned.
- **The `exp` claim was read as an integer.** RFC 7519 says NumericDate, so a
  provider emitting `1756814400.0` silently disabled the expiry check that is
  the whole reason for checking before the driver sends the token.

### Bugs found while building

Each fixed and committed on its own.

- **`authSource` and `authMechanism` were dropped** whenever a connection
  referenced a stored credential, so a URI that authenticated inline stopped
  working once `connection add` extracted its userinfo, falling back to `admin`
  with nothing in the error to say so (`15ab557`). Pre-existing, found by adding
  the coverage `applyAuth` never had.
- **The host allowlist only understood a leading `*.`** and was case-sensitive
  in that arm, so `--allowed-hosts 'db-*.corp.example.com'` matched nothing and
  a pasted `C0.ABC.MONGODB.NET` was refused (`07b19ce`).
- **The device flow's host binding failed open** when either host was unknown,
  making an unreadable connection string the way around it (`4731e85`).
- **`credential logout` reported success** for an OIDC credential on a
  platform-identity flow, which can never be logged in (`4731e85`).

## Non-goals

- Auth-code / browser-redirect flow. Device code covers every host agent-mongo
  runs on; a loopback listener does not.
- A `command` flow that execs arbitrary argv. See the flow table.
- Writing to MongoDB. Unchanged and read-only regardless of IdP-granted roles.
- Managing the Atlas-side federation config. Registering the IdP and mapping
  groups to database roles stays an Atlas administration task.
- `x509` and `gssapi`. The `kind` table accepts them; nothing here implements
  them.

## Open questions

Still open after shipping.

- Nothing here has been run against a real MongoDB deployment. OIDC is
  Enterprise and Atlas only, and the integration harness runs the community
  image, so `internal/oidc` is tested against a mock provider and the driver
  mapping is asserted on the options it produces. One manual run against an
  Atlas M10 with workforce federation is the outstanding gate.
- MongoDB documents driver OIDC support as Linux-qualified in v2.7.0
  (`mongo/client_examples_test.go:464-466`). Whether that affects the
  `environment` flows on macOS in practice is unknown.
- The `mcp` server re-reads the keychain per tool call rather than holding a
  session, which is the safe default. Whether a resident server should cache one
  is a question for whoever measures it hurting.
- Per-principal sessions behind a remote MCP host, so several people share one
  server without sharing a session. The seam exists (`lib-agent-mcp`'s
  `WithIdentityBinding`); the design does not.

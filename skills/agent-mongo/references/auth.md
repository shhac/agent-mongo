# Authentication

Most connections use a username and password; see SKILL.md. This file covers
identity-provider auth, which a minority of deployments use.

## When this applies

The deployment is configured for MONGODB-OIDC — Atlas workforce or workload
identity federation, or a self-managed MongoDB 7.0+ Enterprise cluster. If it
is not, `credential login` reports that the deployment never asked for a
login, and a username and password is what you want instead.

An OIDC credential holds a *flow* — how it obtains a token — rather than a
secret. The connection must use TLS (`mongodb+srv://`, or `tls=true`).

## Which flow

| Flow | Use when | Needs a person | Stores |
| --- | --- | --- | --- |
| `--environment k8s\|azure\|gcp` | the process already has a platform identity: a pod, an Azure VM or Function, a GCE instance | never | nothing |
| `--token-file <path>` | something else already writes a JWT to disk (`az`, `gcloud`, a sidecar) | never | nothing |
| `--device` | a person is authenticating as themselves | at login, then roughly weekly | session in the OS keychain |

```bash
# Platform identity. k8s also covers EKS/IRSA, AKS and GKE.
agent-mongo credential add ci --oidc --environment k8s
agent-mongo credential add azfn --oidc --environment azure --token-resource api://mongodb-atlas

# A token another tool issued. Absolute path; re-read on every authentication,
# so a rotated token is picked up without re-adding anything.
agent-mongo credential add eks --oidc --token-file /var/run/secrets/token

# A person logs in.
agent-mongo credential add corp --oidc --device
agent-mongo connection add prod "mongodb+srv://c0.abc.mongodb.net/app" --credential corp
agent-mongo credential login corp -c prod
```

`credential login` needs a connection because the deployment is what says which
identity provider guards it. `-c` is only needed when several connections use
the credential.

## Driving a device login as an agent

`credential login` prints the code and URL as a `{"notice": ...}` on **stderr**
while it waits. Relay it to the person — they can complete it on any device,
which is why this works over SSH and in containers.

After that, ordinary commands renew the session silently. A person is needed
again only when the refresh token expires or is revoked: weeks, not
invocations.

Check before it bites:

```bash
agent-mongo credential list          # loggedIn, boundTo, expiresAt, expired
agent-mongo connection test prod     # adds sessionExpiresAt to the receipt
agent-mongo credential logout corp   # end the session, keep the credential
```

## Errors and what they mean

All of these are `fixable_by: human` — a person must act, and retrying will not
help.

| Error mentions | Meaning | Fix |
| --- | --- | --- |
| `has no session: nobody has logged in` | never logged in | `agent-mongo credential login <name>` |
| `session ... has expired and cannot be renewed` | refresh token dead or revoked | log in again |
| `was obtained for X and will not be sent to Y` | the session is bound to another deployment | point at X, or log in again for Y |
| `requires TLS` | plaintext connection | use `mongodb+srv://` or add `tls=true` |
| `is not in this credential's allowed hosts` | the host is not one this credential may talk to | point at an allowed host, or widen deliberately |

`Could not renew the session` is `fixable_by: retry` instead — the provider was
unreachable, not the session dead.

## Where a token may be sent

An OIDC credential only sends a token to a host on its allowlist: by default
MongoDB-owned domains and loopback. The driver applies its own list to the
interactive flow alone, so without this a workload flow would hand a live
platform token to whatever host the connection string named.

`--allowed-hosts 'db-*.corp.example.com,mongo.corp.example.com'` widens it for a
self-hosted deployment; patterns are globs and matching ignores case. It is
refused for `--device`, whose session is bound to the deployment it was
obtained for and is never presented elsewhere.

## Why a team might want this

No database passwords are handed out. The token expires in about an hour, a new
one cannot be minted without a person completing a login, access follows the
person's IdP group membership, and Atlas audit logs name a real human rather
than a shared service account.

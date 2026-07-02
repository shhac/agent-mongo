---
description: Build, release, and publish to Homebrew
argument-hint: <patch|minor|major>
---

# Release

Release the `agent-mongo` CLI. A `v*` tag push triggers the shared GitHub
workflow (`shhac/homebrew-tap/.github/workflows/go-release.yml`), which
cross-builds, publishes the GitHub Release, and regenerates + pushes the
Homebrew formula. There is no version file — the binary version comes from the
git tag via ldflags.

## Arguments

- `$ARGUMENTS` — version bump type: `patch`, `minor`, or `major`

## Instructions

### Pre-flight

1. Confirm the working tree is clean (`git st`) and you are on `main`, up to
   date with `origin/main`. If not, stop and ask.
2. Run `make test` and `go vet ./...`. If either fails, stop and fix.
3. Optionally run `make test-integration` (needs docker) for release-critical
   changes to query/mongo code.
4. Compute the new version: latest tag from `git tag --sort=-v:refname | head -1`,
   bumped per `$ARGUMENTS`. Show the user current → new before continuing.

### Step 1: Tag and push

```bash
git tag v<NEW_VERSION>
git push origin main v<NEW_VERSION>
```

### Step 2: Watch the release workflow

```bash
gh run watch --exit-status $(gh run list --workflow=release.yml --limit 1 --json databaseId --jq '.[0].databaseId')
```

If the workflow fails, inspect with `gh run view --log-failed` and fix before
re-tagging (delete the tag locally and remotely first).

### Step 3: Verify

```bash
gh release view v<NEW_VERSION>
```

Confirm the release has assets for darwin/linux (arm64 + amd64) and windows,
and that the tap formula was updated (check the latest commit on the
`homebrew-tap` repo touches `Formula/agent-mongo.rb` with the new version).

The formula push requires the `TAP_DEPLOY_KEY` secret on this repo. If the
workflow logs "TAP_DEPLOY_KEY not set — skipping tap update", the release
succeeded but the formula must be updated by hand: regenerate
`Formula/agent-mongo.rb` in the sibling `homebrew-tap` repo with the new
version + sha256s from the release's `checksums-sha256.txt`, commit as
"agent-mongo <VERSION>", and push. (Fix properly by adding the secret:
`gh secret set TAP_DEPLOY_KEY --repo shhac/agent-mongo`, same deploy key the
sibling CLIs use.)

### Step 4: Report

Show the user:

- New version number
- GitHub release URL
- `brew upgrade shhac/tap/agent-mongo` command for users

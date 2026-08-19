# Proving It

Rules that live on the wire can be proven, not just reviewed. This repo ships **`bfd conform`**, a single-binary conformance tool that deterministically checks a project's boundary artifacts — the OpenAPI contract and the running API — with zero knowledge of the language behind them. Go, Rust, or a rewrite next quarter: the tool cannot tell, which is the point (BFD-26). A third tier checks the toolchain: BFD-17 says nothing merges without a lint gate, so conform verifies the gate exists ([LINT.md](LINT.md)) — by reading its config, never by running it. A fourth tier checks the workflow: BFD-30 through BFD-33 say production advances only by tagged merges from release branches, and conform proves it by reading the git graph itself — evidence, not forge configuration, so the same proof works on Codeberg, GitHub, GitLab, or a bare clone.

## Install

Once, globally (Go 1.24+):

```sh
go install codeberg.org/galaxi/boundary-first-development/cmd/bfd@latest
```

`go install` writes to `$(go env GOPATH)/bin`, which is **not on `PATH` by default** on most Linux setups — the install then succeeds and `bfd` is still "command not found". Put it on `PATH` once:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"   # add to ~/.bashrc or ~/.zshrc
```

`bfd update` checks this for you afterwards, and says so when PATH will not find what it just installed or when an older copy shadows it.

Or sideloaded as a versioned dependency of a Go project, pinned in `go.mod` like anything else:

```sh
go get -tool codeberg.org/galaxi/boundary-first-development/cmd/bfd@latest
go tool bfd conform
```

## Run

From the repo root. Zero configuration is the default:

```sh
bfd conform                                    # toolchain + static: auto-discovers openapi.yaml
bfd conform --base-url http://localhost:8080   # + live: probes the wire, read-only
```

The toolchain and workflow tiers run on every invocation, so a project with no spec and no running API still has its lint gate and its git graph proven. Only a project with none of the four — no spec, no base URL, no linted language, no git history — has nothing to check.

The spec is auto-discovered as `openapi.yaml|yml|json` in the root, `api/`, `docs/`, `spec/`, or `openapi/`. An optional `bfd.yaml` makes the invocation standing:

```yaml
conform:
  spec: api/openapi.yaml
  baseUrl: http://localhost:8080
  endpoints: [/persons, /projects]   # extra GET paths beyond spec discovery
  languages: [go]                    # toolchain tier; default: detected from manifests, [] disables
  requires: [BFD-29]                 # refuse to run on a bfd too old to check these
  workflow:
    production: main                 # default: main, then master
    release: release/*
    staging: [staging/*, testing/*]
    tags: v*
  auth:
    header: Authorization            # value read from $BFD_CONFORM_TOKEN
```

Flags override config: `--spec`, `--base-url`, `--endpoint` (repeatable), `--timeout`, `--json` for a machine-readable result envelope.

The rest of the toolchain is what you would expect: `bfd init` writes a starter `bfd.yaml` (and never overwrites one), `bfd version` prints the installed version, `bfd update` reinstalls the latest release through the Go toolchain, `bfd help` explains itself.

## What it proves

Every finding cites its rule ID.

| Rules | Proof |
|---|---|
| BFD-2 | Every response — including a deliberately requested unknown route — is the `ok`/`data`/`error` envelope. Failures never cross the boundary naked. |
| BFD-3 | Error codes exist and are enumerated in the spec, not free text. |
| BFD-7 | Every model carries `status` and `updatedAt`; every response carries `serverTime`. |
| BFD-8 | List endpoints declare `?updatedAfter=` — and honor it. The wire tier re-requests with the `serverTime` the server itself just reported and expects an empty list. |
| BFD-11 | camelCase on the wire, in the spec and in live bodies. |
| BFD-12 | Timestamps are declared `date-time` and transmitted as UTC. Any RFC3339 value anywhere in a live body with a non-zero offset is a finding. |
| BFD-13 | Regular plurals in routes, schema names, properties, and live keys. |
| BFD-17 | The lint gate exists and enforces the BFD-mapped rules. Every module is found by its manifest — including a monorepo's frontends and nested services — and its config resolved upward the way linters resolve it. Configs are read, never executed. The gates themselves are in [LINT.md](LINT.md). The workflow tier adds the wiring proof: a committed hook-manager config exists (lefthook, pre-commit, husky, `.githooks`) — hooks that live on one laptop do not exist. |
| BFD-18 | The spec declares an apiKey security scheme — the Public API is keyed and documented. The App API may live outside the spec; it moves with the product. |
| BFD-29 | The lint gate bans the artifacts of deferred work — markers, suppressions, swallowed exceptions, commented-out code. The deferred-*capability* half of the rule is not lintable and stays with review; [LINT.md](LINT.md) says so plainly. |
| BFD-30 | The release window — production's first-parent line since the last version tag on it — holds only tagged merges. A direct commit or an untagged merge is a finding with a count and the latest offender; a repository where no release has ever shipped is told exactly that. |
| BFD-31 | The current release tag sits on production's first-parent line. A newest tag on a side branch is a release that did not ship through the door. |
| BFD-32 | Staging never merges into production, and staging-only commits carry recorded cherry-pick provenance (`git cherry-pick -x`). |
| BFD-33 | A recognized CI configuration is committed (Forgejo/Gitea/GitHub workflows, GitLab, Woodpecker, Jenkins, CircleCI, Azure, Bitbucket) and it invokes `bfd conform`. A pipeline that skips the gate is scenery. |

Exit codes: **0** — the boundary holds. **1** — findings. **2** — the tool itself could not run (enumerated error codes, `--json` shows them).

## The workflow tier

Branch protections are forge configuration: every forge spells them differently, and none of them travel with a clone. But the thing protections exist to produce — a clean history — is the repository itself. So the workflow tier never calls a forge API. It reads the graph with plain git plumbing, read-only, and judges what actually happened: production's first-parent line is the sequence of states production has been in, and every violation of BFD-30 through BFD-32 leaves a permanent scar there. Hooks are the courtesy layer, CI is the enforcement layer, and the graph is the audit layer — an escape can skip a hook, but it cannot skip having been recorded.

The tier judges the present, not the archaeology. The **release window** is production's first-parent line since the most recent version tag on it — what has happened since the last release shipped. There is no adoption config and no grace flag: a repository with years of direct commits and no tags gets one aggregated finding saying no release has ever shipped through the door, and the remedy is to ship one — cut a release branch, merge it whole, tag the merge. That release closes the old books, and from then on only the current cycle is ever on trial. The one way to quiet a scar is to ship correctly.

What the tier deliberately cannot see: the *name* of a deleted branch (a merge from a deleted release branch is judged by its tag, not its name), and forge-side settings themselves — declare those on the forge as well, and let the graph prove they held.

## Keeping the checker current

The law moves. An installed binary does not, and a checker that has fallen behind the rules is worse than no checker — it reports a clean run for rules it never looked at. Two mechanisms, one for you and one for the project.

**You get told.** `bfd` checks the Go module proxy — the same index `go install` resolves against — at most once a day, and prints one line when a newer release exists:

```
bfd v0.3.0 is available (you have v0.2.0) — run "bfd update"
```

It rides on `bfd conform`, the command you actually run, and `bfd version` asks immediately. There is no switch to turn it off — running a checker that has fallen behind the law is not a preference (BFD-27). It does stay out of two places, on correctness grounds rather than taste: never inside `--json` or a pipe, where a human notice would corrupt a machine-readable stream, and never when the network is unreachable or the binary was built locally, because neither is news. It never touches an exit code.

```sh
bfd version        # installed version, the law it carries, and whether it is current
bfd conform        # prints the same list under "law:" on every run
bfd update         # reinstall the latest through the Go toolchain
```

**The project gets to insist.** A notice you can ignore is not a gate.

Declare what it expects in `bfd.yaml`, and a stale binary refuses the run — exit 2, code `rules_stale` — instead of passing. This is the half that works in CI, where nobody is reading notices:

```yaml
conform:
  requires: [BFD-29]
```

`bfd init` writes that line. Two adoption paths keep themselves current without it: pinning `bfd` as a Go tool dependency (`go get -tool`) puts it in `go.mod` where Renovate and Dependabot bump it like anything else, and the Claude Code plugin updates the skill through the marketplace. Note those are separate clocks — the plugin carries the rules to the agent, the binary proves them — so a project relying on both should pin `requires` and let the binary tell you when it has fallen behind.

One honest limit: `requires` only works from the release that introduced it onward. A binary older than that ignores the key, because it was never taught to look for it. That is the argument for pinning now rather than later.

Wire checks are read-only GETs, so it is safe anywhere, including CI:

```yaml
- run: go run codeberg.org/galaxi/boundary-first-development/cmd/bfd@latest conform
```

## What it deliberately does not claim

Rules that live in source — single-struct arguments (BFD-15), no `any` (BFD-16), components never calling APIs (BFD-22) — belong to per-language linters, and [LINT.md](LINT.md) ships those linters' configs; conform's toolchain tier proves they are wired, not what they caught. Design rules — disposability (BFD-26), no special cases (BFD-27) — belong to review. The tool proves what is provable and stays silent about the rest.

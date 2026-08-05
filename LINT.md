# Proving It in Source

[CONFORM.md](CONFORM.md) draws a boundary: rules that live in source belong to per-language linters. This is that promise kept. The [`lint/`](lint/) directory holds BFD expressed in the dialect of the linters projects already run — every line annotated with the rule it enforces.

BFD is the rules, never the tooling. Nothing here is a new linter, a plugin, or a dependency. Each preset is a configuration file for a tool you already have. And languages exist here while frameworks do not: the presets know Go, Python, and TypeScript because [RULES.md](RULES.md) itself speaks those dialects — `interface{}` is named in the law — but no preset will ever know what a component library is.

## The Presets

| Preset | Tool | Adopt it |
|---|---|---|
| [`lint/golangci.yml`](lint/golangci.yml) | golangci-lint v2 | copy next to `go.mod` as `.golangci.yml` |
| [`lint/ruff.toml`](lint/ruff.toml) | ruff | copy next to `pyproject.toml` as `ruff.toml`, or fold into `[tool.ruff.lint]` |
| [`lint/eslint.config.mjs`](lint/eslint.config.mjs) | ESLint 9+ (flat config) | copy to the project root; `npm i -D eslint typescript-eslint` |

Presets are floors, not ceilings. Extend them with whatever else your project enforces; the annotated rules are the part `bfd conform` demands.

Remote adoption needs no clone — every preset is fetchable raw: `https://raw.githubusercontent.com/ddnet-repo/boundary-first-development/main/lint/<file>`.

## What Each Gate Proves

Every entry names the linter rule doing the work — stock rules only, in every ecosystem.

| Rule | Go (golangci-lint) | Python (ruff) | TS/JS (ESLint) |
|---|---|---|---|
| BFD-3 | `exhaustive` — enumerated codes are handled exhaustively | — | — |
| BFD-11 | `tagliatelle` — json tags are camelCase | `N` — snake_case on the backend | `naming-convention` — camelCase on the frontend |
| BFD-12 | `forbidigo` — `time.Local` is forbidden | `DTZ` — naive datetimes do not exist | — |
| BFD-15 | `revive` `argument-limit` (3: one payload struct plus what Go mandates) and `flag-parameter` | `PLR0913` at `max-args = 1` (`self` is free) and `FBT` | `max-params` at 1 |
| BFD-16 | `ireturn` — no empty-interface returns | `ANN` — every type explicit; `ANN401` bans `typing.Any` | `no-explicit-any` |
| BFD-22 | — | — | `no-restricted-globals` / `no-restricted-imports` — HTTP lives in the API service, nowhere else |
| BFD-29 | `godox` — no deferral markers; `nolintlint` — suppressions specific, explained, and never unused | `FIX` — markers; `ERA` — commented-out code; `PGH` — blanket `noqa` / `type: ignore`; `S110` — swallowed exceptions | `no-warning-comments` — markers; `no-empty` — swallowed catches; `ban-ts-comment` — suppressions |

The ESLint fence points at `**/services/api/**` by default; point the glob at your API service.

## What It Deliberately Does Not Claim

- No stock Go linter bans `any` in parameters or struct fields — `ireturn` holds the return side of BFD-16 and review holds the rest. That is a gap in the ecosystem, stated plainly, not papered over with a custom analyzer.
- `argument-limit` counts parameters and cannot read them, so it cannot tell `(w, r, input)` from `(a, b, c)`. The limit is 3 because Go mandates `(w, r)` for a handler and BFD-15 allows one payload struct on top; the cost is that a genuine three-argument positional chain now passes the gate. Review holds that case. The limit is not 2: parameters the language imposes are not the function's arguments, and the transposition BFD-15 exists to prevent cannot happen between a `ResponseWriter`, a `*Request`, and a payload anyway.
- No stock Go linter forbids `//nolint` outright, so BFD-29's suppression clause is enforced as far as `nolintlint` reaches: every directive must be specific, explained, and actually suppressing something. A defended suppression is still a suppression, and review still rejects it.
- BFD-29's deferred-capability clause — shipping an API before its authentication — is not a lint finding in any ecosystem. Linters see the code that exists, never the code someone decided to write later. That half of the rule belongs to the governed agent and to review.
- BFD-13 (regular plurals) and BFD-28 (general-to-specific naming) have no stock rules in any ecosystem. They stay with the governed agent and with review — where `bfd conform` already proves BFD-13 on everything that crosses the wire.
- The gates prove what stock rules can prove and stay silent about the rest. No custom lint plugins will be written: becoming the tooling is how the rules stop being the point.

## The Gate Is Itself Checked

BFD-17 says linters run on hooks and nothing merges without passing — so `bfd conform` proves the gate exists. The toolchain tier detects languages from their manifests (`go.mod`; `pyproject.toml`, `setup.py`, `setup.cfg`, `requirements.txt`; `package.json`) and reads the linter configuration, never executing anything:

- **Go** — a `.golangci.yml|yaml|toml|json` must exist and enable the seven linters above (`default: all` also passes).
- **Python** — a `ruff.toml`, `.ruff.toml`, or `[tool.ruff]` in `pyproject.toml` must exist and select `ANN401`, `DTZ`, `ERA`, `FBT`, `FIX`, `N`, `PGH`, `PLR0913`, and `S110` (by group, by code, or `ALL`).
- **TS/JS** — an ESLint config must exist. Flat configs are executable JavaScript, so presence is what the tier proves; the content stays with review.

**Monorepos need no configuration.** A manifest anywhere in the tree declares a module, so `ui-client/package.json` and `services/api/go.mod` are found and checked exactly like a manifest at the root. Config resolution then climbs from each module toward the repo root, the same upward search the linters themselves perform — a frontend with its own `eslint.config.mjs` is covered by it, and a workspace with one config at the top covers every package beneath it. Findings name the module that lacks a gate (`toolchain: js in ui-admin`). The walk goes four levels deep and skips `node_modules`, `vendor`, `dist`, `build`, `target`, and dot-directories.

A missing or insufficient gate is a full finding citing BFD-17, exit 1 — not a warning. Conform has findings and clean, nothing in between. Polyglot repos can still pin or disable the tier in `bfd.yaml`:

```yaml
conform:
  languages: [go]   # only check the go gate; [] disables the tier
```

Omit the key and the tier checks whatever the manifests say is there.

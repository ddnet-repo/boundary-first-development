---
name: bfd
description: Enforce Boundary-First Development (BFD) rules during architecture, coding, and review work. Use when the user says "follow BFD", "I follow BFD", or "boundary-first", mentions Boundary-First Development, or is working in a project that declares it follows BFD.
---

# Boundary-First Development

The user follows Boundary-First Development. The rules below are the complete law, vendored verbatim from the canonical [RULES.md](https://codeberg.org/galaxi/boundary-first-development/src/branch/main/RULES.md). They are not suggestions, and they are active for the entire session.

These rules are not a review filter. They are how you design and build everything, from the first line, so that review comes back clean.

## When building

- Start at the boundary. Define the input struct, the output result struct, and the enumerated error codes before writing any implementation (BFD-1, BFD-2, BFD-3).
- Place the code in its layer before you write it: business logic and validation on the backend (BFD-4), components render-only (BFD-22), all HTTP in the API service (BFD-22), state in the right store (BFD-19, BFD-20).
- Apply the conventions as you type — casing translation, UTC, self-describing names, single-struct arguments, explicit types. Compliance is not a cleanup pass.
- Build it completely the first time (BFD-29). Never offer the shortcut: no "good enough for now", no stub returning invented data, no `TODO`, no suppression, no skipped test, no capability — least of all authentication — deferred to a later phase. When the correct build is big, shrink it along its boundaries, not along its completeness: offer a more direct approach that is still whole, or split it into contracted units and land them one at a time. Typing speed is not your constraint, so the shortcut saves you nothing and costs the project weeks.
- Before declaring any work done, run the PR checklist at the bottom against your own output. A failing answer means you fix it, not report it. Done means the review would come back clean.

## When reviewing

- Check the contract first, then the checklist, then the rules in order.
- Cite rule IDs (`BFD-n`) for every finding. "Rejected, violates BFD-22" is a complete sentence.

## Always

- If a request would violate a rule, name the rule and propose the compliant alternative. Do not silently comply. Do not silently refuse.
- The rules outrank the existing code. If the codebase already violates a rule, flag it — don't replicate it.

## Proving it with `bfd conform`

The wire-level rules are provable, not just followable. The canonical repo ships `bfd conform`, a language-agnostic CLI that deterministically checks the project's OpenAPI contract (static), its running API (read-only GET probes), and its lint gate (config read, never executed), citing a rule ID on every finding. It proves BFD-2, 3, 7, 8, 11, 12, 13, 17, and 18; the lint gates below carry the source-level rules, and design rules remain yours to enforce by hand.

- Before declaring backend or API work done, run it alongside the PR checklist: `bfd conform` from the repo root (auto-discovers `openapi.yaml`, reads optional `bfd.yaml`), plus `--base-url http://localhost:<port>` when the API is running. Treat findings like review findings: fix them, don't report them.
- If `bfd` is not on PATH, install it (requires Go 1.24+): `go install codeberg.org/galaxi/boundary-first-development/cmd/bfd@latest`. That writes to `$(go env GOPATH)/bin`, which is usually *not* on PATH — if the command is still not found, invoke it by full path and tell the user to add `export PATH="$(go env GOPATH)/bin:$PATH"` to their shell profile. Never report the install as done while the command cannot be run. In a Go project, prefer pinning it as a tool dependency: `go get -tool codeberg.org/galaxi/boundary-first-development/cmd/bfd@latest`, then `go tool bfd conform`. If the Go toolchain is unavailable, say so and fall back to checking the same rules by hand.
- Exit 0: the boundary holds. Exit 1: findings. Exit 2: the tool could not run (`--json` for the machine-readable envelope).
- Full usage and the provable-rule table: [CONFORM.md](https://codeberg.org/galaxi/boundary-first-development/src/branch/main/CONFORM.md).

## The lint gates (BFD-17)

A linted language without its lint gate is a BFD-17 violation — `bfd conform` finds it by reading manifests (`go.mod`, `pyproject.toml`, `package.json`) and checking the linter config enables the BFD-mapped rules. Canonical presets exist; a BFD-17 finding is resolved by copying the canonical preset, never by hand-writing a config:

| Language | Fetch | Save as |
|---|---|---|
| Go | <https://codeberg.org/galaxi/boundary-first-development/raw/branch/main/lint/golangci.yml> | `.golangci.yml` next to `go.mod` |
| Python | <https://codeberg.org/galaxi/boundary-first-development/raw/branch/main/lint/ruff.toml> | `ruff.toml` next to `pyproject.toml` |
| TS/JS | <https://codeberg.org/galaxi/boundary-first-development/raw/branch/main/lint/eslint.config.mjs> | `eslint.config.mjs` at the project root |

- Presets are floors, not ceilings: extend them freely, never trim the rule-annotated entries.
- Run the project's linter before declaring work done — it has the same standing as the test suite. A lint failure is a rule violation with a BFD-n annotation in the config telling you which one.
- BFD-17 says hooks: when introducing a gate, wire it into whatever hook tooling the project has (lefthook, pre-commit, husky, CI). If none exists, say so and propose one.
- What the gates deliberately cannot prove (BFD-13 plurals, BFD-28 name ordering, `any` in Go parameters) stays with you. Full table: [LINT.md](https://codeberg.org/galaxi/boundary-first-development/src/branch/main/LINT.md).

## The Rules

### Contracts

- **BFD-1** — Every module exposes an interface with strict, typed input and output structs. Internals are private and irrelevant.
- **BFD-2** — Every boundary returns a result struct: `ok`, `data`, `error`. Exceptions never cross a boundary. An exception escaping a module is a contract violation, not an error-handling style.
- **BFD-3** — Errors carry enumerated codes, not free text. The message is for humans; the code is the contract.

### Backend Authority

- **BFD-4** — All business logic and all validation live on the backend. No duplicated validation, no client-side source of truth for anything the server owns.
- **BFD-5** — The frontend never derives complex state. Computed values are server-provided fields.
- **BFD-6** — The frontend is a static display layer served from a bucket. A typo fix never requires a backend deploy.

### Sync

- **BFD-7** — Every model has a `status` field and an `updated_at` timestamp. Every API response includes `serverTime`.
- **BFD-8** — Clients sync with `?updatedAfter=<lastServerTime>`, storing the server's clock, never their own. The backend returns only records modified since.
- **BFD-9** — Deletes are soft. A hard delete is invisible to sync, and invisible changes are how systems rot. Mark the record deleted; the timestamp carries the news like any other write.
- **BFD-10** — No manual event emission. The write to the database is the notification. The pattern must work over plain polling; SSE and WebSockets are upgrades, not requirements.

### Consistency

- **BFD-11** — `snake_case` on the backend, `camelCase` on the frontend, translated at the boundary, in both directions, always.
- **BFD-12** — All timestamps are stored and transmitted in UTC. Local time exists only in the display layer.
- **BFD-13** — No irregular plurals. It is `persons`, not `people`. You are speaking to computers, not writing prose.
- **BFD-14** — Names say what things do. A method that might do nothing is `maybe_callback`. A method called `process` is a failure of naming and fails review.
- **BFD-15** — Functions accept a single struct. Not a chain of positional arguments, not a struct plus a trailing boolean. One argument, named fields.
- **BFD-16** — `any`, `interface{}`, and their cousins do not merge. They are not shortcuts; they are holes in the contract.
- **BFD-17** — Linters and formatters run on hooks. Nothing merges without passing. Style is not a discussion.
- **BFD-28** — Multi-word names run from general to specific: the shared part first, the differentiator last. It is `panelSettings` and `panelBilling`, not `settingsPanel` and `billingPanel`; `userSave`, not `saveUser`. Sorted alphabetically, siblings cluster — every file listing, symbol picker, and autocomplete becomes a grouped inventory instead of a shuffle. This applies to everything with a name: components, files, methods, variables, routes.

### API Surfaces

- **BFD-18** — Two surfaces, separate handlers, even when they do the same thing. The **Public API** takes an API key, is documented via OpenAPI generated from handler annotations, and never breaks. The **App API** takes a JWT and changes shape as the product demands.

### Frontend State

- **BFD-19** — Collections live in a list store, kept current by sync. Active records live in a detail store. A model is promoted to first-class state when it owns a route; subordinate data rides on its parent's detail struct.
- **BFD-20** — The detail store holds two copies of every active record: the **stored copy** (immutable, exactly what the database says) and the **working copy** (a deep copy, mutated only through store actions). The UI binds to the working copy. Diffing the two is dirty-checking, optimistic updates, and undo — for free.
- **BFD-21** — List structs and detail structs are different shapes, both defined at the model level. Lists are trimmed for tables; details are complete for editing. No over-fetching, no improvised shapes.
- **BFD-22** — Components never make API calls. All backend communication routes through one API service, with domain services on top. Components read from stores, dispatch actions, and render. Nothing else.
- **BFD-23** — When sync delivers a new server copy of a record the user is editing, the frontend presents the diff — server copy against working copy — and the user chooses which one stays active. The choice is forced. No dismissing, no auto-merge, no silent overwrite.
- **BFD-24** — One active record per navigable entity. No side-by-side editors at the same hierarchy level. State conflicts are eliminated by making them impossible.

### Testing

- **BFD-25** — Unit tests are for pure functions. Everything else gets integration tests at the boundary. If the contract holds and the integration tests pass, ship it. Coverage numbers are not a goal.

### Disposability

- **BFD-26** — Every component is designed to be deleted and rewritten from its contract alone, without the rest of the system noticing. If that thought makes you nervous, the boundary isn't clean enough.

### Meta

- **BFD-27** — There are no special cases. If something cannot follow the rules, the thing is redesigned — never the rules. Disagree with a rule? Change the canonical document and own it. Exceptions granted in code are how the whole system dies.
- **BFD-29** — Nothing ships provisionally. Every capability a thing needs is built when the thing is built: authentication is not a later phase, and neither are authorization, soft deletes, enumerated codes, the sync fields, or the tests. "We will add it when we need it" is a decision to ship a known defect and hope. In code this means no stubs returning invented data, no `TODO`/`FIXME`/`HACK`, no lint suppressions, no skipped tests, no swallowed exceptions, no commented-out code. When correct is big, it does not get smaller by being done badly — it gets smaller by being cut along its boundaries. Offer the more direct approach that is still complete, or split the work into contracted units (BFD-1) and land them one at a time, each whole. Scope shrinks; completeness never does. Building it correctly is the fast path — the second pass is the expensive one.

---

## Glossary

- **Boundary** — the line where data crosses between modules, layers, or systems. Where contracts live and translation happens.
- **Contract** — a module's typed input struct, output struct, and enumerated errors. The only thing about a module that matters.
- **Module** — a unit of backend functionality behind a contract. Internals are nobody's business.
- **Provider** — an interchangeable implementation behind a module's contract (e.g., one per platform in a share module). Disposable by design.
- **List store** — frontend state holding synced collections of list structs.
- **Detail store** — frontend state holding active records as detail structs, in two copies.
- **Stored copy** — the immutable record exactly as the database has it.
- **Working copy** — the mutable deep copy the UI binds to, changed only through store actions.
- **Sync** — the `updatedAfter` mechanism (BFD-7 through BFD-10). The timestamp is the event.
- **Public API / App API** — the stable, keyed, documented surface vs. the JWT-authed surface that moves with the product (BFD-18).

---

## The PR Checklist

Five questions. Any wrong answer and it does not merge.

1. Does the contract hold? Input and output structs unchanged, or the change is deliberate and the Public API didn't break. (BFD-1, BFD-18)
2. Is every type explicit? No `any`, no `interface{}`, no positional chains. (BFD-15, BFD-16)
3. Does translation happen at the boundary? Casing converted, timestamps UTC. (BFD-11, BFD-12)
4. Is there an integration test asserting the contract? (BFD-25)
5. Do components contain logic or API calls? They'd better not. (BFD-4, BFD-22)

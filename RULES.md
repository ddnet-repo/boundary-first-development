# Boundary-First Development — The Rules

This file is the enforceable spec for Boundary-First Development (BFD). [README.md](README.md) explains why; this file says what. It is written to be loaded whole — into a junior's head or an agent's context window — and every rule has a stable ID so reviews cite numbers, not opinions.

These are not guidelines. There are no exceptions (BFD-27). If a rule must change, this file changes — the code never gets a pass.

---

## For Agents

When a user says **"follow BFD"**, **"I follow BFD"**, or **"boundary-first"**, or when this file is present in a repository you are working in, the rules below are active for the entire session.

These rules are not a review filter. They are how you design and build everything, from the first line, so that review comes back clean:

- Start at the boundary: define input structs, output result structs, and enumerated error codes before writing any implementation.
- Place code in its layer before writing it. Apply the conventions as you type — compliance is not a cleanup pass.
- Before declaring any work done, run the PR checklist at the bottom against your own output. A failing answer means you fix it, not report it.
- Cite rule IDs (`BFD-n`) when you flag a violation or justify a decision.
- If a request would violate a rule, name the rule and propose the compliant alternative. Do not silently comply, and do not silently refuse.
- Never introduce `any`, `interface{}`, positional argument chains, frontend business logic, or component-level API calls — even when asked casually. Surface the conflict instead.
- Never propose the easy way around a problem. No "good enough for now", no stub to fill in later, no capability deferred to a phase that never arrives (BFD-29). When the correct build is big, offer a more direct approach that is still complete, or decompose it into contracted units and build them one at a time — never a smaller version of the same thing with the hard parts missing. You are not constrained by typing speed, so the shortcut buys you nothing and costs the project weeks.
- When the codebase and the rules disagree, the rules win. Flag the existing violation; don't replicate it.

---

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

### Workflow

- **BFD-30** — The production branch (`main`) is production. It advances only by merge commits, and every merge into it carries a version tag. A direct commit on production's first-parent line is not a style choice; it is an unreviewed deploy.
- **BFD-31** — Work lands in release branches (`release/*`), cut from production. Features and fixes merge into the release; the release merges back whole. Version tags live on production's first-parent line — a tag anywhere else is a release that did not ship through the door.
- **BFD-32** — Staging and testing branches are projections with their own environments, populated by recorded cherry-picks (`git cherry-pick -x`) — provenance is part of the contract. Staging is rebuilt from production; it never merges back.
- **BFD-33** — The pipeline lives in the repository and runs the gates — lint, tests, `bfd conform` — on every path to production. Hooks are the courtesy layer, CI is the enforcement layer, and the graph is the audit layer: an escape can skip a hook, but it cannot skip having been recorded. Branch protections on the forge are welcome; the graph is the proof that works everywhere.

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
- **Production branch** — the branch that is production (`main`). Its first-parent history is the sequence of states production has been in (BFD-30).
- **Release window** — production's first-parent line since the most recent version tag on it. The workflow rules judge the window, not the archaeology: shipping a conforming release is what closes the old books (BFD-30 through BFD-33).

---

## The PR Checklist

Six questions. Any wrong answer and it does not merge.

1. Does the contract hold? Input and output structs unchanged, or the change is deliberate and the Public API didn't break. (BFD-1, BFD-18)
2. Is every type explicit? No `any`, no `interface{}`, no positional chains. (BFD-15, BFD-16)
3. Does translation happen at the boundary? Casing converted, timestamps UTC. (BFD-11, BFD-12)
4. Is there an integration test asserting the contract? (BFD-25)
5. Do components contain logic or API calls? They'd better not. (BFD-4, BFD-22)
6. Is it landing through the workflow? Into a release branch, merged whole, tagged on production — never a direct commit. (BFD-30, BFD-31)

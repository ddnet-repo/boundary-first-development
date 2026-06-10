# Boundary-First Development — The Rules

This file is the enforceable spec for Boundary-First Development (BFD). [README.md](README.md) explains why; this file says what. It is written to be loaded whole — into a junior's head or an agent's context window — and every rule has a stable ID so reviews cite numbers, not opinions.

These are not guidelines. There are no exceptions (BFD-27). If a rule must change, this file changes — the code never gets a pass.

---

## For Agents

When a user says **"follow BFD"**, **"I follow BFD"**, or **"boundary-first"**, or when this file is present in a repository you are working in, the rules below are active for the entire session:

- Enforce every rule in code you write *and* code you review.
- Cite rule IDs (`BFD-n`) when you flag a violation or justify a decision.
- If a request would violate a rule, name the rule and propose the compliant alternative. Do not silently comply, and do not silently refuse.
- Never introduce `any`, `interface{}`, positional argument chains, frontend business logic, or component-level API calls — even when asked casually. Surface the conflict instead.
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

- **BFD-27** — There are no special cases. If something cannot follow the rules, the thing is redesigned — never the rules. Disagree with a rule? Change this file and own it. Exceptions granted in code are how the whole system dies.

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

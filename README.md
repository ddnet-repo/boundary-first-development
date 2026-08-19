# Boundary-First Development

**Build systems that don't depend on the skill of the builder.**

Boundary-First Development (BFD) is an opinionated architecture philosophy for web applications. Strict contracts, backend authority, enforced consistency — chosen over developer freedom, aesthetic elegance, and pattern purity.

This document is the why. **[RULES.md](RULES.md) is the law** — numbered, citable, and sized to fit in a context window.

---

## Using It

Five files, five jobs:

- **README.md** — the why. For humans deciding whether this is how they want to work.
- **[RULES.md](RULES.md)** — the law. Twenty-nine numbered rules, a glossary, and the PR checklist. Built to be loaded into an agent's context or a junior's head, whole.
- **[AGENTS.md](AGENTS.md)** — the hookup. How to bind any agent to the rules without touching what you already have.
- **[CONFORM.md](CONFORM.md)** — the proof. `bfd conform`, a language-agnostic tool that deterministically checks the wire-level rules against a project's OpenAPI contract and running API, and the workflow rules against its git graph — on any forge, because the graph travels with the clone.
- **[LINT.md](LINT.md)** — the gates. The source-level rules expressed as config presets for the linters you already run — golangci-lint, ruff, ESLint — and checked for by `bfd conform`. BFD stays the rules; your linter stays your linter.

Agents follow BFD when the rules are in their context, not because this repo is public. And BFD is a fixed point: it gets *referenced*, never copied into your instruction files and edited. Your `AGENTS.md` can contain whatever it contains — BFD is still BFD.

**Claude Code** — install the plugin once; "follow BFD" then works in every project:

```
/plugin marketplace add https://codeberg.org/galaxi/boundary-first-development.git
/plugin install bfd@galaxi
```

Or from the terminal, for scripts and machine setup:

```sh
claude plugin marketplace add https://codeberg.org/galaxi/boundary-first-development.git
claude plugin install bfd@galaxi
```

**OpenCode** — point your config (global or per-project `opencode.json`) at the canonical rules:

```json
{ "instructions": [
  "https://codeberg.org/galaxi/boundary-first-development/raw/branch/main/RULES.md",
  "https://codeberg.org/galaxi/boundary-first-development/raw/branch/main/LINT.md"
] }
```

**Anything else** that only reads prose files: vendor `RULES.md` read-only and add a one-line pointer to your existing `AGENTS.md`. Details in [AGENTS.md](AGENTS.md).

**The conformance tool** — agents follow the rules; `bfd conform` proves the wire-level ones, deterministically, against any language's backend (Go 1.24+ to install):

```sh
go install codeberg.org/galaxi/boundary-first-development/cmd/bfd@latest
export PATH="$(go env GOPATH)/bin:$PATH"   # go install's target is rarely on PATH already
bfd conform --base-url http://localhost:8080
```

Zero config: it auto-discovers `openapi.yaml`, probes the running API read-only, and exits nonzero with rule-cited findings if the boundary does not hold. Full usage, `bfd.yaml` config, and the list of provable rules in [CONFORM.md](CONFORM.md).

### Session scope

"Follow BFD" activates per session, not per command. Once the skill fires, the ruleset is in the agent's context and governs everything until the session ends. A fresh session needs the signal again, and the phrase doesn't have to be literal — "boundary-first," or working in a project that declares it follows BFD, fires it too.

To make the signal standing instead of spoken:

- **A repo that is always BFD:** one line in its `CLAUDE.md` — `This project follows Boundary-First Development — invoke the bfd skill.` Every session in that repo starts governed.
- **A person who is always BFD:** the same line in `~/.claude/CLAUDE.md` covers every project on the machine.
- **OpenCode with the `instructions` URL** is already standing — the rules load with every session.

A governed session builds to the rules from the first line — contracts before implementation, conventions as it types — and self-reviews against the PR checklist before calling work done. Review findings cite rule IDs.

---

## What This Is

This philosophy was not written by someone who dislikes creativity. It was written by someone who loves hacking things together, making code do things it was never supposed to do, and solving problems in ways that make other engineers uncomfortable. That energy is fantastic for experimentation and side projects.

It is terrible for professional delivery.

Professional software should be boring. Boring means predictable deadlines, no crunch time, no fires. Boring means a new developer or an AI agent can sit down, read the contracts, and produce correct work without needing to understand the creative vision of whoever came before them.

> If a system requires good developers to function correctly, it is a bad system. A well-designed system produces correct output from average input.

Save the cleverness for where it matters. Make the system itself as boring as possible.

---

## The Principles

### 1. Contracts Are the Architecture

Every module exposes an interface with strict input and output structs. What happens inside the module is nobody's business.

A share module accepts content, a platform slug, and publishing options. It returns a result in a defined struct. Inside, there may be providers for Facebook, Instagram, YouTube — each written by a different person, in a different style, at a different skill level. None of that matters. The contract is the only thing that matters.

Failure is part of the contract. Every boundary returns a result struct — `ok`, `data`, `error` — and errors are enumerated codes, not free text. Exceptions never cross a boundary. An exception escaping a module is not an error-handling style; it is a contract violation.

- **Providers are disposable.** Swap them and the system doesn't notice.
- **Teams scale without coordination.** Hand someone an interface definition and say "make this work."
- **Code review shrinks to what matters.** Does the contract hold? Does the integration test pass? Ship it.

### 2. The Backend Is the Only Truth

The backend decides what the data is, what states are valid, and what operations are permitted. Everything else is a projection.

No business logic in the frontend. No duplicated validation. No client-side source of truth for anything the server owns.

The frontend is a display layer served from a bucket. A typo fix does not require a backend deploy. Kill the CDN cache and move on.

### 3. The Timestamp Is the Event

Every model has a `status` field and an `updated_at` timestamp. Every API response includes a `serverTime` value.

The frontend loads a collection and stores `serverTime`. On subsequent syncs, it sends `?updatedAfter=lastServerTime`. The backend returns only records modified after that timestamp. The frontend patches its local list.

When a backend process saves a record, `updated_at` changes. That record appears in the next sync automatically. No manual event emission. No WebSocket message to forget to send. No pub/sub to configure. The write to the database *is* the notification. The timestamp *is* the event.

Deletes are soft. A hard delete is invisible to sync, and invisible changes are how systems rot. Mark the record deleted and let the timestamp carry the news like any other write.

This pattern is transport-agnostic. Polling is the lowest-friction implementation and represents the worst-case scenario. If the architecture works with polling, it works. SSE and WebSockets only make it better.

### 4. Consistency Is Non-Negotiable

There are no special cases. If something cannot follow the rules, the thing is redesigned. The rules are not redesigned.

The full set is in [RULES.md](RULES.md). The flavor:

- The backend uses `snake_case`. The frontend uses `camelCase`. Translation happens at the boundary, always, in both directions.
- All timestamps are stored and transmitted in UTC. The frontend has a datetime service for display. Communication is UTC with zero exceptions.
- Model names do not use irregular plurals. It is `persons`, not `people`. You are speaking to computers, not writing prose.
- Names are intentional and self-describing. A method that might do nothing is called `maybe_callback`. A method called `process` is a failure of naming. If you cannot tell what a function does from its name alone, rename it.
- Multi-word names run from general to specific, so siblings sort together: `panelSettings` and `panelBilling`, not `settingsPanel` and `billingPanel`. The domain leads, the differentiator follows — for components, files, methods, and variables alike.
- Functions accept a single struct. Not a chain of positional arguments, not a struct plus a trailing boolean. One argument, named fields.
- In a typed language, escape hatches like `any` or `interface{}` do not exist. They are not shortcuts, they are holes in the contract. Every type is explicit or the code does not merge.
- Linters and formatters run on hooks. Nothing merges without passing.
- Nothing ships provisionally. No stubs, no `TODO`, no suppressions, no skipped tests, and no capability — least of all authentication — deferred to a phase that never arrives.

Consistency eliminates an entire class of decisions. A junior or an AI agent never has to ask "what's the convention here?" It is always the same.

### 5. Separate What Must Be Stable From What Can Move

The API is split into two surfaces:

**Public API** — requires an API key. Fully documented with OpenAPI specs generated from handler annotations. Backward-compatible. Breaking a public endpoint is a failure of planning.

**App API** — requires a JWT. Powers the first-party frontend. Can change shape freely as the product evolves.

Separate handlers even when they do the same thing. Debugging is simpler when call stacks are distinct. Internal evolution never risks public breakage.

### 6. The Frontend Is a State Reflection Machine

The frontend maintains two versions of truth: what the server says, and what the user is changing. Everything follows from keeping those cleanly separated.

**Collections live in a list store.** Each tracked model gets an entry, populated by initial API calls and kept current by the sync mechanism from Principle 3. When an updated record arrives, the list patches itself and notifies any active detail view that its data is stale.

**Active records live in a detail store.** Not every model needs this treatment. Promote a model to first-class state management when it owns a route. Subordinate data can live as a field on the parent's detail struct.

The detail store maintains two copies of every active record:

- **The stored copy.** Immutable. The exact state as it exists in the database.
- **The working copy.** A deep copy, mutable only through store actions. This is what the UI binds to.

Components read via getters and write via store actions. Never mutate directly. Diffing the working copy against the stored copy shows exactly what has changed, which is everything you need for dirty-checking, optimistic updates, and undo.

**Conflicts get resolved by the owner, immediately.** When sync delivers a new server copy of a record the user is editing, the frontend presents the diff — server copy against working copy — and the user chooses which one stays active. The choice is forced. No dismissing, no auto-merge, no silent overwrite. The person who owns the work resolves the conflict at the moment it happens, every time.

**List structs and detail structs are different shapes.** Lists are trimmed for tables and grids. Details are complete for editing. The list store holds list structs. The detail store holds detail structs. Defining both at the model level prevents the frontend from over-fetching or improvising shapes.

**Components never make API calls.** All backend communication routes through a centralized API service, with domain services layered on top. Components are presentation logic only: they read from stores, dispatch actions, and render UI. Every user action that touches the backend is trackable in one place.

### 7. Test the Boundary, Not the Implementation

Unit tests are for pure utility functions. Everything else gets integration tests.

Does the data that goes in produce the data we expect out? If providers are interchangeable black boxes, testing their internals is testing something disposable. Test the boundary. Assert the contract holds.

Internals can be refactored freely. If the integration tests pass, ship it. CI/CD stays fast because you are testing the things that matter, not chasing a coverage number.

### 8. Nothing Is Precious

Providers are swappable. Frontends are disposable. Modules are isolated. Any piece of this system can be rewritten, replaced, or deleted without the rest noticing.

Design every component as if someone will throw it away next quarter. If that thought makes you nervous, the boundaries aren't clean enough.

### 9. Correct Is the Fast Path

Nothing ships provisionally. Every capability a thing needs is built when the thing is built — authentication is not a later phase, and neither are authorization, soft deletes, enumerated codes, the sync fields, or the tests. "We'll add it when we need it" is not a schedule decision. It is a decision to ship a known defect and hope nobody finds it first.

The shortcut was always a bet: that the hours saved now exceed the hours lost later. For a person typing every character, that bet was at least arguable, and it still usually lost — the deferred work came back as an incident, a migration, or a breach, and it came back at the worst possible time. It is not arguable any more. A system that can produce the correct version in the same sitting as the shortcut isn't saving anything by choosing the shortcut. There is no trade left to make. There is just the loss, scheduled for later.

None of which means every job is small. Some of them are enormous, and the answer to an enormous job is never a smaller version of it with the hard parts missing. It is either a more direct approach that is still complete, or the same job cut along its boundaries into contracted units and built one at a time, each one whole and each one shippable. That is what the contracts in Principle 1 are *for*. Scope shrinks; completeness never does.

In code, the rule shows up as absences: no stubs returning invented data, no `TODO`, no suppressions, no skipped tests, no swallowed exceptions, no commented-out code — because each of those is a note explaining that the work is not finished, filed in the one place nobody reads.

BFD-27 forbids exempting a thing from the rules. BFD-29 forbids exempting a moment.

---

## Trade-Offs

Opinions have costs. These are stated, not apologized for.

**Offline-first is not a goal.** The backend is authoritative. If it's unreachable, the frontend is stale.

**The frontend does not reason.** Complex state derivation belongs on the backend, served as computed fields.

**One active record per navigable entity.** No side-by-side editors at the same hierarchy level. This eliminates state conflicts by making them impossible.

**Polling has latency.** The architecture works without real-time transport, but upgrade to SSE or WebSockets when you need instant feedback. The pattern supports it cleanly.

**The system is rigid on purpose.** Developers who value expressive freedom will find this constraining. That is the point. The constraint is what makes the work boring, and boring is what lets you go home on time.

**This system works as a whole.** Partial adoption reintroduces the problems it is designed to eliminate. Disagree with a rule? Change [RULES.md](RULES.md) and own it. Exceptions granted in code are how the whole system dies.

---

## Who This Is For

Teams where skill levels vary, AI agents write production code, contractors rotate in and out, and the person maintaining the codebase in two years is not the person who built it.

People who have cleaned up enough messes to know they were caused by inconsistency, ambiguity, and systems that relied on everyone being excellent all the time.

People who love building things — and learned that the way to keep loving it is to make the professional work boring enough that it never becomes a crisis.

---

## The Point

Make the system boring so the work never has to be exciting.

Good systems do not rely on discipline. They eliminate the need for it.

# Boundary-First Development

**Build systems that don't depend on the skill of the builder.**

---

## What This Is

Boundary-First Development is an opinionated architecture philosophy for web applications. It prioritizes strict contracts, backend authority, and enforced consistency over developer freedom, aesthetic elegance, or pattern purity.

It was not written by someone who dislikes creativity. It was written by someone who loves hacking things together, making code do things it was never supposed to do, and solving problems in ways that make other engineers uncomfortable. That energy is fantastic for experimentation and side projects.

It is terrible for professional delivery.

Professional software should be boring. Boring means predictable deadlines, no crunch time, no fires. Boring means a new developer or an AI agent can sit down, read the contracts, and produce correct work without needing to understand the creative vision of whoever came before them.

> If a system requires good developers to function correctly, it is a bad system. A well-designed system produces correct output from average input.

Save the cleverness for where it matters. Make the system itself as boring as possible.

---

## The Principles

### 1. Contracts Are the Architecture

Every module exposes an interface with strict input and output structs. What happens inside the module is nobody's business.

A share module accepts content, a platform slug, and publishing options. It returns a result in a defined struct. Inside, there may be providers for Facebook, Instagram, YouTube — each written by a different person, in a different style, at a different skill level. None of that matters. The contract is the only thing that matters.

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

This pattern is transport-agnostic. Polling is the lowest-friction implementation and represents the worst-case scenario. If the architecture works with polling, it works. SSE and WebSockets only make it better.

### 4. Consistency Is Non-Negotiable

There are no special cases. If something cannot follow the rules, the thing is redesigned. The rules are not redesigned.

- The backend uses `snake_case`. The frontend uses `camelCase`. Translation happens at the boundary, always, in both directions.
- All timestamps are stored and transmitted in UTC. The frontend has a datetime service for display. Communication is UTC with zero exceptions.
- Model names do not use irregular plurals. It is `persons`, not `people`. You are speaking to computers, not writing prose.
- Names are intentional and self-describing. A method that might do nothing is called `maybe_callback`. A method called `process` is a failure of naming. If you cannot tell what a function does from its name alone, rename it.
- Functions accept a single struct, not a chain of positional arguments. One optional boolean as a final parameter is the outer limit of tolerance, and even that should make you uncomfortable.
- In a strongly typed language, `any` does not exist. It is not a shortcut, it is a hole in the contract. Every type is explicit or the code does not merge.
- Linters and formatters run on hooks. Nothing merges without passing.

Consistency eliminates an entire class of decisions. A junior or an AI agent never has to ask "what's the convention here?" It is always the same.

### 5. Separate What Must Be Stable From What Can Move

The API is split into two surfaces:

**Public API** — requires an API key. Fully documented with OpenAPI specs generated from handler annotations. Backward-compatible. Breaking a public endpoint is a failure of planning.

**App API** — requires a JWT. Powers the first-party frontend. Can change shape freely as the product evolves.

Separate handlers even when they do the same thing. Debugging is simpler when call stacks are distinct. Internal evolution never risks public breakage.

### 6. The Frontend Is a State Reflection Machine

The frontend maintains two versions of truth: what the server says, and what the user is changing. Everything follows from keeping those cleanly separated.

**The Library Store** holds read-only lists of collections. Each tracked model gets an entry — `posts: []`, `persons: []`, etc. Populated by initial API calls, kept current by the sync mechanism from Principle 3. When an updated record arrives, the library patches its list and checks: is this record currently active? If so, it tells the record store to refresh.

**The Record Store** holds the active instance of each major navigable entity. "Navigable" means it owns a URL — `/community/:communityId/person/:personId`. Router middleware watches active IDs and tells the record store to load details. Not every model needs this treatment. Subordinate data (like `person_notes`) can live as a field on the parent's detail struct. The URL defines what gets first-class state management.

The record store maintains two copies of every active record:

- **The stored copy** — `Object.freeze` immutable. The exact state as it exists in the database.
- **The working copy** — a deep copy, mutable only through store actions. This is what the UI binds to.

Components read via getters, write via store actions. Never mutate directly. You can always diff working state against stored state to see exactly what has changed.

**List structs vs. detail structs.** Every model defines both. List structs are trimmed for tables and grids. Detail structs are complete for editing. The library holds list structs. The record store holds detail structs.

**Components never make API calls.** All backend communication goes through a centralized API service. Domain services — share, person, community, whatever — sit on top of it, but everything routes through one place. Components are presentation logic only — they read from stores, dispatch actions, and render UI. Every user action that touches the backend is trackable in a single layer.

### 7. Test the Boundary, Not the Implementation

Unit tests are for pure utility functions. Everything else gets integration tests.

Does the data that goes in produce the data we expect out? If providers are interchangeable black boxes, testing their internals is testing something disposable. Test the boundary. Assert the contract holds.

Internals can be refactored freely. If the integration tests pass, ship it. CI/CD stays fast because you are testing the things that matter, not chasing a coverage number.

### 8. Nothing Is Precious

Providers are swappable. Frontends are disposable. Modules are isolated. Any piece of this system can be rewritten, replaced, or deleted without the rest noticing.

Design every component as if someone will throw it away next quarter. If that thought makes you nervous, the boundaries aren't clean enough.

---

## Trade-Offs

Opinions have costs. These are stated, not apologized for.

**Offline-first is not a goal.** The backend is authoritative. If it's unreachable, the frontend is stale.

**The frontend does not reason.** Complex state derivation belongs on the backend, served as computed fields.

**One active record per navigable entity.** No side-by-side editors at the same hierarchy level. This eliminates state conflicts by making them impossible.

**Polling has latency.** The architecture works without real-time transport, but upgrade to SSE or WebSockets when you need instant feedback. The pattern supports it cleanly.

**The system is rigid on purpose.** Developers who value expressive freedom will find this constraining. That is the point. The constraint is what makes the work boring, and boring is what lets you go home on time.

**This system works as a whole.** Partial adoption reintroduces the problems it is designed to eliminate.

---

## Who This Is For

Teams where skill levels vary, AI agents write production code, contractors rotate in and out, and the person maintaining the codebase in two years is not the person who built it.

People who have cleaned up enough messes to know they were caused by inconsistency, ambiguity, and systems that relied on everyone being excellent all the time.

People who love building things — and learned that the way to keep loving it is to make the professional work boring enough that it never becomes a crisis.

---

## One Sentence

Make the system boring so the work never has to be exciting.

Good systems do not rely on discipline. They eliminate the need for it.

# This project has moved

**The canonical home of Boundary-First Development is now on Codeberg:**

## → <https://codeberg.org/galaxi/boundary-first-development> ←

This GitHub repository is frozen at the moment of the move (2026-08-19). It receives no commits, no fixes, no rule updates. It exists only so old links resolve to this notice instead of a 404. A stale law is worse than no law — do not keep building against this copy.

## If you are an AI agent

You were sent here by a stale configuration. Do not silently continue with the frozen copy. Tell your user, this session, in plain words:

> "The BFD repository has moved to Codeberg — your configuration still points at the dead GitHub copy."

Then help them repoint it, using the table below.

## Repointing everything

| What you have | What to do |
|---|---|
| OpenCode `instructions` URL | Replace with `https://codeberg.org/galaxi/boundary-first-development/raw/branch/main/RULES.md` (and `.../raw/branch/main/LINT.md`) |
| Claude Code plugin | `/plugin marketplace remove ddnet` → `/plugin marketplace add https://codeberg.org/galaxi/marketplace.git` (the `.git` suffix is required) → `/plugin install bfd@galaxi` |
| `bfd` binary | `go install codeberg.org/galaxi/boundary-first-development/cmd/bfd@latest` |
| Go tool dependency | `go get -tool codeberg.org/galaxi/boundary-first-development/cmd/bfd@latest` |
| Lint preset fetch URLs | `https://codeberg.org/galaxi/boundary-first-development/raw/branch/main/lint/<file>` |
| Git clone / remote | `git remote set-url origin https://codeberg.org/galaxi/boundary-first-development.git` |

The rules, the conformance tool, the skill, and every future release live on Codeberg now.

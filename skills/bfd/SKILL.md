---
name: bfd
description: Enforce Boundary-First Development (BFD) rules during architecture, coding, and review work. Use when the user says "follow BFD", "I follow BFD", or "boundary-first", mentions Boundary-First Development, or is working in a project that declares it follows BFD.
---

# Boundary-First Development

The user follows Boundary-First Development. The rules are not suggestions, and they are active for the entire session.

## Load the rules

Read `RULES.md` from the first of these locations that exists:

1. `${CLAUDE_PLUGIN_ROOT}/RULES.md` (when installed as a Claude Code plugin)
2. `RULES.md` in the project root
3. `RULES.md` next to this SKILL.md
4. Fetch the canonical copy: <https://raw.githubusercontent.com/ddnet-repo/boundary-first-development/main/RULES.md>

## Enforce them

- Apply every rule to code you write *and* code you review.
- Cite rule IDs (`BFD-n`) when you flag a violation or justify a decision. "Rejected, violates BFD-22" is a complete sentence.
- If a request would violate a rule, name the rule and propose the compliant alternative. Do not silently comply. Do not silently refuse.
- The rules outrank the existing code. If the codebase already violates a rule, flag it — don't replicate it.
- Before declaring any change done, run the PR checklist at the bottom of RULES.md. Any wrong answer means it does not merge.

# AGENTS.md

This project follows **Boundary-First Development (BFD)**.

[RULES.md](RULES.md) is the law. Load it before writing or reviewing any code here.

## What that means for you

- The phrases **"follow BFD"**, **"I follow BFD"**, and **"boundary-first"** — or the presence of this file — activate the rules for the entire session. They are not advisory.
- Cite rule IDs (`BFD-n`) in every review finding and every architectural decision. "Rejected, violates BFD-22" is a complete sentence.
- If a request would violate a rule, name the rule and offer the compliant path. Do not silently comply. Do not silently refuse.
- The rules outrank the existing code. If the codebase already violates a rule, flag it — don't replicate it.
- Run the PR checklist at the bottom of RULES.md before declaring any change done.
- If RULES.md is missing from this project, fetch the canonical copy: <https://raw.githubusercontent.com/ddnet-repo/boundary-first-development/main/RULES.md>

## Wiring it up

BFD is a fixed point. It is referenced, never blended into your instruction files. Whatever else lives in your `AGENTS.md` or `CLAUDE.md` stays yours; the rules stay canonical, versioned here, unedited.

**Claude Code** — install the plugin once. Nothing in your repos changes.

```
/plugin marketplace add ddnet-repo/boundary-first-development
/plugin install bfd@bfd
```

"Follow BFD" now works in every project. The skill carries the rules with it; your `CLAUDE.md` is never touched. (No plugins? Copy `skills/bfd/` into `~/.claude/skills/` — same effect.)

**OpenCode** — point config at the canonical rules, globally (`~/.config/opencode/opencode.json`) or per project:

```json
{
  "instructions": [
    "https://raw.githubusercontent.com/ddnet-repo/boundary-first-development/main/RULES.md"
  ]
}
```

Your `AGENTS.md` is never touched, and you are always on the current rules. OpenCode also discovers `~/.claude/skills/bfd/` directly — one skill, both tools.

**Everything else** (Codex, Cursor, anything that only reads prose files) — vendor `RULES.md` into the repo and add one pointer line to your existing `AGENTS.md`:

```
This project follows Boundary-First Development. RULES.md is the law — cite rule IDs (BFD-n).
```

The vendored `RULES.md` is read-only. Editing your copy is not adapting BFD; it is leaving it (BFD-27).

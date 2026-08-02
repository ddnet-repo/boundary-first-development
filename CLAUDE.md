# Working on BFD

This project follows Boundary-First Development — invoke the bfd skill.

BFD is one body of water. A change to any part of it updates every surface that carries it, in the same change — an out-of-date material is a finding, not a footnote:

- **RULES.md** is the law. A change to the law re-vendors `skills/bfd/SKILL.md` verbatim in the same commit.
- **skills/bfd/SKILL.md** never drifts from RULES.md, and any change under `skills/` or `lint/` bumps the version in `.claude-plugin/plugin.json` — an unbumped version is a change no install will ever see.
- **README.md, CONFORM.md, LINT.md, AGENTS.md** describe behavior; behavior changes land in them in the same commit that changes the behavior.
- **`lint/` presets, `conform`'s required sets, and this repo's own `.golangci.yml`** move in lockstep — proven by `conform/parity_test.go`, not remembered.

`go test ./...` proves the mechanical parity; the prose parity is on you. Before declaring any work here done, run the tests, run `golangci-lint run`, and walk the surface list above. Done means every surface says the same thing.

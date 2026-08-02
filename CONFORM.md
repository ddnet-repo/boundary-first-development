# Proving It

Rules that live on the wire can be proven, not just reviewed. This repo ships **`bfd conform`**, a single-binary conformance tool that deterministically checks a project's boundary artifacts — the OpenAPI contract and the running API — with zero knowledge of the language behind them. Go, Rust, or a rewrite next quarter: the tool cannot tell, which is the point (BFD-26). A third tier checks the toolchain: BFD-17 says nothing merges without a lint gate, so conform verifies the gate exists ([LINT.md](LINT.md)) — by reading its config, never by running it.

## Install

Once, globally (Go 1.24+):

```sh
go install github.com/ddnet-repo/boundary-first-development/cmd/bfd@latest
```

Or sideloaded as a versioned dependency of a Go project, pinned in `go.mod` like anything else:

```sh
go get -tool github.com/ddnet-repo/boundary-first-development/cmd/bfd@latest
go tool bfd conform
```

## Run

From the repo root. Zero configuration is the default:

```sh
bfd conform                                    # static: auto-discovers openapi.yaml
bfd conform --base-url http://localhost:8080   # + live: probes the wire, read-only
```

The spec is auto-discovered as `openapi.yaml|yml|json` in the root, `api/`, `docs/`, `spec/`, or `openapi/`. An optional `bfd.yaml` makes the invocation standing:

```yaml
conform:
  spec: api/openapi.yaml
  baseUrl: http://localhost:8080
  endpoints: [/persons, /projects]   # extra GET paths beyond spec discovery
  languages: [go]                    # toolchain tier; default: detected from manifests, [] disables
  auth:
    header: Authorization            # value read from $BFD_CONFORM_TOKEN
```

Flags override config: `--spec`, `--base-url`, `--endpoint` (repeatable), `--timeout`, `--json` for a machine-readable result envelope.

The rest of the toolchain is what you would expect: `bfd init` writes a starter `bfd.yaml` (and never overwrites one), `bfd version` prints the installed version, `bfd update` reinstalls the latest release through the Go toolchain, `bfd help` explains itself.

## What it proves

Every finding cites its rule ID.

| Rules | Proof |
|---|---|
| BFD-2 | Every response — including a deliberately requested unknown route — is the `ok`/`data`/`error` envelope. Failures never cross the boundary naked. |
| BFD-3 | Error codes exist and are enumerated in the spec, not free text. |
| BFD-7 | Every model carries `status` and `updatedAt`; every response carries `serverTime`. |
| BFD-8 | List endpoints declare `?updatedAfter=` — and honor it. The wire tier re-requests with the `serverTime` the server itself just reported and expects an empty list. |
| BFD-11 | camelCase on the wire, in the spec and in live bodies. |
| BFD-12 | Timestamps are declared `date-time` and transmitted as UTC. Any RFC3339 value anywhere in a live body with a non-zero offset is a finding. |
| BFD-13 | Regular plurals in routes, schema names, properties, and live keys. |
| BFD-17 | The lint gate exists and enforces the BFD-mapped rules. Languages are detected from their manifests; configs are read, never executed. The gates themselves are in [LINT.md](LINT.md). |
| BFD-18 | The spec declares an apiKey security scheme — the Public API is keyed and documented. The App API may live outside the spec; it moves with the product. |

Exit codes: **0** — the boundary holds. **1** — findings. **2** — the tool itself could not run (enumerated error codes, `--json` shows them).

Wire checks are read-only GETs, so it is safe anywhere, including CI:

```yaml
- run: go run github.com/ddnet-repo/boundary-first-development/cmd/bfd@latest conform
```

## What it deliberately does not claim

Rules that live in source — single-struct arguments (BFD-15), no `any` (BFD-16), components never calling APIs (BFD-22) — belong to per-language linters, and [LINT.md](LINT.md) ships those linters' configs; conform's toolchain tier proves they are wired, not what they caught. Design rules — disposability (BFD-26), no special cases (BFD-27) — belong to review. The tool proves what is provable and stays silent about the rest.

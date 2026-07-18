# CLAUDE.md

Agent-native, read-only Go CLI (+ MCP server) over the LACCD eLumen public course catalog (`portalapi-laccd.elumenapp.com`). Searches/browses ~9,664 approved courses and every degree/certificate program across all 9 colleges, pulls full Outlines of Record as JSON, compares a course across colleges for articulation, and keeps an offline SQLite mirror with full-text search. This tree is **generated output** from [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).

## Architecture
- `cmd/laccd-courses-pp-mcp/main.go` — MCP server entry point (present in this tree).
- `internal/cli/` — one file per command: `courses_get`, `courses_search`, `courses_compare`, `courses_prereqs`, `courses_subjects`, `programs`/`programs_get`/`programs_search`, `coverage`, `stats`, `sync`, `doctor`, `which`, plus eLumen glue (`laccd_envelope.go`, `laccd_outline.go`, `laccd_colleges.go`, `helpers.go`).
- `internal/client/` — eLumen HTTP client (handles 1-indexed pagination + tenant-host mapping).
- `internal/store/` — local SQLite mirror (pure-Go `modernc.org/sqlite`).
- `internal/mcp/` (+ `mcp/cobratree`) — MCP tool + intent definitions mirroring the CLI.
- `internal/cache/`, `internal/config/`, `internal/cliutil/`, `internal/types/` — HTTP cache, TOML config, shared helpers, shared structs.
- Data flow: command → `internal/client` (eLumen fetch) → parse envelope/outline → `internal/store` SQLite mirror → filtered output. `coverage`, `stats`, `courses compare`, `courses subjects` read the local mirror.

## Commands
```bash
make build-mcp    # go build -o bin/laccd-courses-pp-mcp ./cmd/laccd-courses-pp-mcp  (works locally)
make test         # go test ./...   (passes)
make lint         # golangci-lint run
# NOTE: `make build` / `make install` target ./cmd/laccd-courses-pp-cli, which is
# NOT present in this working tree (only the MCP main is) — those targets fail here.
# Install the CLI binary via the published path instead:
#   go install github.com/mvanhorn/printing-press-library/library/other/laccd-courses/cmd/laccd-courses-pp-cli@latest
# Runtime self-discovery:
laccd-courses-pp-cli doctor --json
laccd-courses-pp-cli agent-context --pretty
laccd-courses-pp-cli which "<capability>" --json
```
Requires Go 1.26.4+ (see `go.mod` `toolchain`).

## Conventions
- Every command supports `--json`, `--select <dotted.paths>`, `--dry-run`, and `--agent` (JSON + compact + no color + non-interactive). Human table on a TTY; JSON when piped.
- Errors to stderr; exit codes: `0` ok, `2` usage, `3` not found, `5` API, `7` rate limited, `10` config.
- Read-only tool — never mutates remote state.
- Path kinds (config/data/state/cache) resolve independently; relocate with `LACCD_COURSES_HOME` or per-kind `LACCD_COURSES_*_DIR`.

## Gotchas & Constraints
- **Missing CLI main:** only `cmd/laccd-courses-pp-mcp` exists in this tree; the Makefile's `build`/`install` (CLI) targets will error until the CLI main package is present.
- **Generated tree.** Fix systemic issues upstream in Printing Press first; keep local edits narrow; record intentional local changes under a `.printing-press-patches/` sidecar so a reprint doesn't drop them.
- **Never hand-bump the version** (`var version = ...`) or `CHANGELOG.md` — the `mvanhorn/printing-press-library` publish workflow stamps the `YYYY.M.N` release.
- eLumen quirks handled for you: list endpoint is **1-indexed** (page 0 → HTTP 502), so `--page` defaults to 1; `--tenant ELAC` maps friendly codes to the eLumen host (`elac.elumenapp.com`). `courses compare`/`coverage`/`stats` read the mirror — run `sync` or a `courses search` first or they return empty.
- No auth/API keys. `.gitignore` excludes `*.db` mirrors, binaries, `*.mcpb`, `/.printing-press*.json`.
- See `README.md`/`SKILL.md` for product usage, `HANDOFF.md` for build history.

# Handoff — `laccd-courses-pp-cli`

> Comprehensive context for the next session working on this CLI. Read this first.

## 1. TL;DR / current state

A Go CLI for the **LACCD (Los Angeles Community College District) public course catalog**, which is served by **eLumen** and has no official API — only a one-college-at-a-time Angular website. Generated with the **Printing Press** (`cli-printing-press` v4.25.0), then hand-extended.

- **Status:** shipped & live-verified. Scorecard **93/100 (Grade A)**, shipcheck 7/7 legs, live dogfood **68/68**, agentic code + output reviews PASS.
- **This repo:** `github.com/johnnyrobot/laccd-courses-pp-cli` (private) — clean source only.
- **Local clone:** `/Users/laccd/code/laccd-courses-pp-cli`
- **Printing-press library copy:** `~/printing-press/library/laccd-courses/` (managed; canonical for reprint/publish/polish)
- **Run artifacts (research/proofs/discovery):** `~/printing-press/manuscripts/laccd-courses/20260622-152754-09360b55/`
- **Run ID:** `20260622-152754-09360b55`

Module path: `laccd-courses-pp-cli`. Binary: `laccd-courses-pp-cli`. No auth (anonymous public API).

---

## 2. The eLumen API contract (CRITICAL — these quirks cost hours; don't rediscover them)

**Base URL:** `https://portalapi-laccd.elumenapp.com` (discovered from the SPA's compiled JS bundle).
**Frontend:** `https://laccd.elumenapp.com/public/courses` — Angular SPA, all data via XHR to the API above.
**Auth:** none. Every `/public/*` endpoint is anonymous. No key, no cookie.

### Endpoints

| Method | Path | Notes |
|--------|------|-------|
| GET | `/public/tenants/` | The 9 colleges. Bare JSON array. `[{id, name(host), displayName(code), baseUrl}]` |
| GET | `/public/configurations` | eLumen catalog config key/values (~200KB; mostly internal form config) |
| GET | `/public/courses?query=&status=&tenant=&pageSize=&page=&sort=` | HAL list (see envelope below) |
| GET | `/public/courses/{id}` | Full course detail, ~40-50KB (404 for unknown id) |
| GET | `/public/programs?query=&status=&tenant=&pageSize=&page=` | HAL list (`_embedded.programs`) |
| GET | `/public/programs/{id}` | Full program detail |

### HAL response envelope (list endpoints)
```json
{
  "_embedded": { "courses": [ ... ] },
  "_links": { "first": {...}, "last": {...}, "next": {...}, "self": {...} },
  "pagination": { "currentPage": 1, "isFirstPage": true, "isLastPage": false,
                  "limit": 25, "total": 587, "totalPages": 24 }
}
```

### ⚠️ Quirks (all verified, all handled in the code)

1. **`page` is 1-INDEXED. `page=0` → HTTP 502. A MISSING `page` param ALSO → 502.** The first request MUST send `page=1`. (This masquerades as many other "bugs" — e.g. it's why empty queries *seemed* to 502.)
2. **`pageSize` hard-caps at 25.** Requesting 50/100/500 all return 25. A full mirror = many pages.
3. **`tenant` value is the full eLumen host** (`elac.elumenapp.com`), **NOT** the display code. `tenant=ELAC` → `total: 0`.
4. **`query` is full-text** over code/title/description. It matches the subject **code** for real subjects (`query=ACCTG` → 72 ACCTG courses) but does **not** match `"<subject> <number>"` (`query=ENGL 101` → 0). Watch out: LACCD English is subject **`ENGLISH`** (code `ENGLISH102`), not `ENGL`. To find a specific subject+number, fetch `query=<subject>` and filter `number` locally.
5. **`status=approved`** (lowercase; case-insensitive). The public catalog default. Optional — omitting it returns all statuses.
6. **`courseTransferStatus` is an OBJECT**, not a string: `{"valueName":"Transferable to both UC and CSU","valueCode":"A",...}`. Read `.valueName`.
7. **`fullCourseInfo` is a JSON-ENCODED STRING** inside each course record (both list and detail). Decode twice. It holds the full Outline of Record: `creditsAndHours`, `requisites`, `outcomes`, `objectives`, `courseDescription`, `modeOfDelivery`, `courseTransferStatus`, `courseGeneralEducationStatus`, etc.
8. **`requisites` is a nested AND/OR tree:** `{type:"OR", blockList:[{type:"AND", blockList:[{item:{code,title,itemType:"Prerequisite"|"Advisory"|"Co-Requisite", isCourse}}]}]}`. Leaves carry their own course code+title (self-contained — no store lookup needed).
9. **Backend is SLOW and occasionally flaky.** Cold requests ~15-20s; warm ~2s. The list endpoint sometimes returns transient 502s. Retry. A full district mirror (~9,664 courses ÷ 25/page × slow) is impractical — use targeted per-subject/per-college syncs.
10. **Empty `query` works with `page≥1`** — district-wide returns ~9,664 approved courses; one college (`tenant=...`) returns its full catalog (~700-1,700).

### College map (code → eLumen host) — several are non-obvious
```
ELAC  → elac.elumenapp.com         East LA College
LACC  → lacc.elumenapp.com         LA City College
LAHC  → lahc.elumenapp.com         LA Harbor College
LAMC  → lamission.elumenapp.com    LA Mission College      ← NOT "lamc"
LAPC  → pierce.elumenapp.com       LA Pierce College       ← NOT "lapc"
LASC  → lasc.elumenapp.com         LA Southwest College
LATTC → lattc.elumenapp.com        LA Trade-Technical College
LAVC  → lavc.elumenapp.com         LA Valley College
WLAC  → wlac.elumenapp.com         West LA College
```
(Hardcoded in `internal/cli/laccd_colleges.go` and fetched live via `colleges list`.)

---

## 3. Command surface

### Absorbed (generator-emitted typed endpoints)
- `courses search [--query --status --tenant --page-size --page --sort]`
- `courses get <id>`
- `programs search [--query --status --tenant --page-size --page]`
- `programs get <id>`
- `colleges list`
- `config list`

### Framework (generator-emitted)
`sync`, `search` (offline FTS), `analytics`, `doctor`, `workflow archive|status`, `agent-context`, `which`, `api`, `import`, `profile`, `feedback`, plus global flags `--json --agent --select --compact --csv --plain --quiet --data-source --dry-run --timeout --rate-limit`.

### Novel (hand-coded — `internal/cli/*.go`)
| Command | Data source | Notes |
|---------|-------------|-------|
| `courses compare <subject> <number> [--diff-only]` | local store | Cross-college matrix (units/transfer/prereqs). **Requires prior sync.** `--diff-only` collapses identical colleges. |
| `courses prereqs <id>` | auto (live, store fallback) | Walks the requisite tree. `id` validated numeric (path-traversal guard). |
| `courses subjects [subject]` | local store | No arg → distinct subjects + counts. With arg → courses in that subject. |
| `coverage` | local (default) / live | Default: synced counts per college (fast). `--data-source live` probes API totals (parallel; slow). |
| `stats` | local store | Per-college course/subject/transferable distribution. |
| friendly `--tenant` | — | `courses/programs search --tenant ELAC` maps code→host via `resolveTenant`. |

**Typical workflow for the local-store commands:**
```bash
# build a per-subject mirror (full district mirror is impractical):
laccd-courses-pp-cli sync --resources courses --resource-param courses:query=ACCTG
laccd-courses-pp-cli courses compare ACCTG 001 --diff-only
laccd-courses-pp-cli stats
```

---

## 4. Code structure

### Hand-authored, standalone files (regen-safe — no "Generated by" header, classified NOVEL on reprint)
- `internal/cli/laccd_colleges.go` — college map, `resolveTenant`, `collegeCodeForHost`
- `internal/cli/laccd_outline.go` — `fullCourseInfo` parsing: `decodeFullCourseInfo`, `parseRequisites`, `extractUnits`, `extractTransferStatus`, `normalizeCourseNumber`, `jsonNumber`
- `internal/cli/laccd_envelope.go` — `normalizeListPayload` (HAL `_embedded` → bare array)
- `internal/cli/coverage.go`, `stats.go`, `courses_compare.go`, `courses_prereqs.go`, `courses_subjects.go` — the 5 novel commands
- `internal/cli/laccd_outline_test.go`, `laccd_helpers_test.go` — table-driven tests for the pure logic

### Hand-EDITED generated files (⚠️ a Printing Press reprint will flag these for review)
These carry the "Generated by ... DO NOT EDIT" header but were edited to handle eLumen's contract. On `/printing-press-reprint`, `regen-merge` will classify them as `TEMPLATED-BODY-DRIFT` and require review:
- `internal/cli/data_source.go` — added `normalizeListPayload(data)` in the 3 live-read branches of `resolveReadWithStrategy` (so HAL list responses display + cache as bare arrays).
- `internal/cli/sync.go` — `determinePaginationDefaults` set `cursorType="page"`, `limit=25`; force `page=1` on first request (the `pageCursor` block); dogfood `maxPages=1`.
- `internal/cli/channel_workflow.go` — `workflow archive` page cap honors `IsDogfoodEnv` (was hardcoded 100); added `cliutil` import.
- `internal/cli/courses_search.go`, `programs_search.go` — `params["tenant"] = formatCLIParamValue(resolveTenant(flagTenant))`.
- `internal/cli/coverage_test.go`, `stats_test.go`, `courses_compare_test.go`, `courses_prereqs_test.go`, `courses_subjects_test.go` — generated `t.Skip` behavior placeholders replaced with real assertions.

### Spec
Hand-authored internal YAML: `~/printing-press/manuscripts/laccd-courses/20260622-152754-09360b55/research/laccd-courses-spec.yaml` (also `spec.yaml` in repo/library). `category: other` (the public-library enum has no education/reference category). The auto-sniffed spec (`laccd-courses-browser-sniff-spec.yaml`) was discarded — its HAL inference was wrong (picked nested sub-arrays as `response_path`).

---

## 5. Build / test / run

```bash
cd /Users/laccd/code/laccd-courses-pp-cli
go build -o laccd-courses-pp-cli ./cmd/laccd-courses-pp-cli
go test ./...
go vet ./...

./laccd-courses-pp-cli doctor
./laccd-courses-pp-cli courses search --query accounting --page-size 5 --json
./laccd-courses-pp-cli courses get 40830 --select code,name,subject,status,tenant
./laccd-courses-pp-cli courses prereqs 30636        # CS122 → CS119 advisory + MATH261 coreq
./laccd-courses-pp-cli sync --resources courses --resource-param courses:query=ACCTG
./laccd-courses-pp-cli courses compare ACCTG 001 --diff-only
```
Known-good fixture ids: course `40830` (ACCTG001, no prereqs), `30636` (CS122, has prereqs); program `255` (Nursing). These are live ids — they may change if eLumen renumbers.

---

## 6. Known issues / limitations / retro candidates

### Limitations (inherent to the slow/capped API, not bugs)
- A full district mirror is impractical (25/page × slow backend). The offline commands (`compare`, `subjects`, `stats`, `coverage`) operate over whatever you've synced; `coverage` exists to show how partial the mirror is.
- `compare`/`subjects`/`stats` require a prior `sync`; they emit a clear "sync first" hint when the store is empty.

### Scorecard below-max dims (structural for a small read-only no-auth API — not worth gaming)
- `mcp_token_efficiency` 4/10 (6-tool surface; the endpoint-collapse pattern is for ~70-endpoint APIs)
- `cache_freshness` 5/10 (no full-mirror sync path by design), `breadth` 7/10, `mcp_tool_design` 7/10

### gosec
25 findings (G104/G304/G201/G204) — **all in generator-emitted files, zero in hand-authored code.** The G201 SQL `Sprintf` is identifier-guarded with `?`-bound params; G204 is the fixed-binPath companion runner. These are Printing Press retro candidates.

### Printing Press generator gaps found this run (worth `/printing-press-retro`)
1. **`workflow archive` ignored `IsDogfoodEnv`** — used a hardcoded 100-page cap (unlike `newSyncCmd`), causing a live-dogfood timeout. The dogfood page cap should be centralized across all sync entry points.
2. **HAL `_embedded` live-read extraction** — `resolveReadWithStrategy` applies `response_path` only to sync + mutation responses, not live list *display*. Needed a hand-fix (`normalizeListPayload`).
3. **Sync pagination config** — declaring `pagination` in the internal spec did not produce `cursorType="page"`; had to hand-edit `determinePaginationDefaults`. The internal-YAML pagination block has no page-number type.

---

## 7. Printing Press operations (if continuing via the press)

The CLI lives in the managed library, so these slash-commands work:
- `/printing-press-polish laccd-courses` — re-run diagnostics + auto-fix.
- `/printing-press-reprint laccd-courses` — regenerate under a newer press, carrying prior research + novels + patches. **Heads-up:** regen-merge will flag the hand-edited generated files in §4 for review (the standalone hand-authored files are preserved automatically).
- `/printing-press-publish laccd-courses` — open a PR to the public `mvanhorn/printing-press-library` (this run instead pushed to a private repo).
- `/printing-press-retro` — file the generator gaps in §6.
- `/printing-press-score laccd-courses` — re-score.

To sync changes from this repo back into the library (or vice-versa), diff the two trees — they started identical at promote time.

---

## 8. Ideas for a next iteration (not built)
- A `seed`/multi-subject sync helper that paginates common subjects across all colleges to build a fuller offline mirror (mindful of the slow backend — background/long-running).
- `programs requirements <id>` — resolve a program's required courses (the program detail has `additionalParts`/`builderRules`; data shape needs verification).
- Surface `courseGeneralEducationStatus` (already parsed by `extractGeneralEducationStatus`) in `compare`/`get`.
- `articulation` view combining `compare` across a list of courses for a transfer plan.
- Cache the `courseTransferStatus`/units into typed store columns for faster `stats` (currently parsed from the `data` JSON per row).

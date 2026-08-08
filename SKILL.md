---
name: pp-laccd-courses
description: "Search every LACCD course and program across all 9 colleges from your terminal — with cross-college comparison, prerequisite chains, and offline search the per-college web catalog can't do. Trigger phrases: `search LACCD courses`, `find a course at Pierce college`, `compare ACCTG 001 across LACCD colleges`, `course outline for`, `LACCD degree programs`, `prerequisites for this course`, `use laccd-courses`, `run laccd-courses`."
author: "johnnyrobot"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - laccd-courses-pp-cli
    install:
      - kind: go
        bins: [laccd-courses-pp-cli]
        module: github.com/johnnyrobot/laccd-courses-pp-cli/cmd/laccd-courses-pp-cli
---

# LACCD Course Catalog — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `laccd-courses-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via Go (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`):
   ```bash
   go install github.com/johnnyrobot/laccd-courses-pp-cli/cmd/laccd-courses-pp-cli@latest
   ```
2. Verify: `laccd-courses-pp-cli --version`
3. Ensure the install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If Go is unavailable, download a pre-built binary from the [latest release](https://github.com/johnnyrobot/laccd-courses-pp-cli/releases/latest) and put it on `$PATH`.

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The Los Angeles Community College District publishes its public course and program catalog through eLumen, but the website only shows one college at a time and has no API, no export, and no CLI. This tool turns that catalog into a fast, scriptable, agent-native surface: search or browse ~9,664 approved courses and every degree/certificate program across all 9 colleges, pull a full Outline of Record as JSON, compare a course across colleges for articulation, and keep an offline mirror with full-text search. It also smooths the backend's quirks for you — the 1-indexed pagination (page 0 returns 502) and the eLumen tenant-host format are handled automatically.

## When to Use This CLI

Use this CLI when an agent or user needs LACCD course or program catalog data: searching or browsing courses across the 9 colleges, pulling a full Outline of Record (units, hours, requisites, SLOs, description), comparing a course across colleges for articulation or transfer, walking a course's prerequisite chain, or listing degree and certificate programs. It is ideal for academic-advising, transfer-planning, and curriculum-analysis tasks that need machine-readable catalog data.

## Anti-triggers

Do not use this CLI for:
- enrolling in, registering for, or paying for classes — this is read-only public catalog data
- class schedules, section times, instructors, or seat availability — the public catalog exposes course/program records, not term section offerings
- colleges outside LACCD — this CLI is scoped to the 9 Los Angeles Community College District colleges
- the authenticated eLumen staff / curriculum-approval portal (this CLI only reads the anonymous public catalog)

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-college intelligence
- **`courses compare`** — Compare the same subject+number across the LACCD colleges you've synced — units, transfer status, and prerequisite count side by side.

  _Reach for this for articulation/transfer questions — e.g. ACCTG 001 carries the same units and transfer status across LACCD but a different prerequisite count at some colleges. --diff-only surfaces only the mismatches._

  ```bash
  laccd-courses-pp-cli courses compare ACCTG 001 --diff-only --agent
  ```

### Local catalog that compounds
- **`courses subjects`** — List every distinct subject prefix you've synced with course counts, and pull all courses for one subject across colleges (e.g. 'courses subjects ACCTG').

  _Use this to map the catalog by discipline before drilling in — e.g. seed a RAG index with every MATH course across LACCD._

  ```bash
  laccd-courses-pp-cli courses subjects --agent
  ```
- **`coverage`** — Report how complete the local catalog mirror is per college — synced row counts vs the API's reported totals — before you trust offline search.

  _Check this before relying on offline results so an empty answer means 'not in the catalog', not 'not synced yet'._

  ```bash
  laccd-courses-pp-cli coverage --agent
  ```
- **`stats`** — Aggregate the synced course store into per-college counts, units distribution, and transfer-status share — a district-wide view no per-college page provides.

  _Reach for this for curriculum-analysis questions about how the catalog is distributed across the district._

  ```bash
  laccd-courses-pp-cli stats --agent
  ```

### Course structure
- **`courses prereqs`** — Walk a course's structured requisite blocks and print its prerequisite / advisory / corequisite chain with course codes and titles.

  _Use this when a student or planner needs to know what must be completed before a course — the chain is rebuilt by hand from the outline every term today._

  ```bash
  laccd-courses-pp-cli courses prereqs 30636 --agent
  ```

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 10 API entries from 10 total network entries
- Protocols: rest_json (75% confidence)
- Auth signals: none
- Candidate command ideas: get_courses — Derived from observed GET /public/courses/{course_id} traffic.; get_programs — Derived from observed GET /public/programs/{program_id} traffic.; list_configurations — Derived from observed GET /public/configurations traffic.; list_courses — Derived from observed GET /public/courses traffic.; list_programs — Derived from observed GET /public/programs traffic.; list_tenants — Derived from observed GET /public/tenants/ traffic.

## Command Reference

**colleges** — List the LACCD colleges (eLumen tenants) available in the catalog

- `laccd-courses-pp-cli colleges` — List the 9 LACCD colleges, their tenant codes, and catalog base URLs.

**config** — Read the catalog's public configuration values

- `laccd-courses-pp-cli config` — List the public eLumen catalog configuration key/value pairs (review windows, presentation settings).

**courses** — Search the LACCD course catalog and fetch full course outlines

- `laccd-courses-pp-cli courses get` — Fetch a single course by its numeric catalog id, including the full course outline (units, hours, requisites, outcomes
- `laccd-courses-pp-cli courses search` — Search or browse approved courses across LACCD. Leave --query empty to browse a whole college (or all 9).

**programs** — Search LACCD degree and certificate programs and fetch full program details

- `laccd-courses-pp-cli programs get` — Fetch a single program by its numeric id, including award type, credit ranges, requirements, and SLOs.
- `laccd-courses-pp-cli programs search` — Search or browse LACCD degree and certificate programs. Leave --query empty to browse a college's full program list.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
laccd-courses-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Narrow a big course record

```bash
laccd-courses-pp-cli courses get 40830 --agent --select code,name,subject,number,status,tenant
```

A course detail is ~40KB (the full Outline of Record lives in a nested fullCourseInfo blob); --select returns just the flat fields so agents don't burn context on the whole outline.

### Compare a course across the district

```bash
laccd-courses-pp-cli courses compare ACCTG 001 --diff-only
```

Aligns ACCTG 001 across every synced college and shows only what differs (e.g. prerequisite counts). Sync the subject first: 'sync --resources courses --resource-param courses:query=ACCTG'.

### Prerequisite chain for a course

```bash
laccd-courses-pp-cli courses prereqs 30636
```

Prints the course's prerequisite, advisory, and corequisite chain from its structured requisite blocks (fetched live; no sync needed).

### Browse a college's catalog by subject

```bash
laccd-courses-pp-cli courses subjects --agent
```

After syncing, lists every subject you've mirrored with district-wide counts; add a subject (e.g. 'subjects ACCTG') to list its courses.

### Find degree and certificate programs

```bash
laccd-courses-pp-cli programs search --query nursing --agent
```

Search programs across LACCD; --agent emits structured JSON for downstream advising tools.

## Auth Setup

No authentication required.

Run `laccd-courses-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  laccd-courses-pp-cli colleges --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `LACCD_COURSES_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `LACCD_COURSES_CONFIG_DIR`, `LACCD_COURSES_DATA_DIR`, `LACCD_COURSES_STATE_DIR`, `LACCD_COURSES_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `LACCD_COURSES_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `laccd-courses-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "laccd-courses": {
        "command": "laccd-courses-pp-mcp",
        "env": {
          "LACCD_COURSES_HOME": "/srv/laccd-courses"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `LACCD_COURSES_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `LACCD_COURSES_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
laccd-courses-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
laccd-courses-pp-cli feedback --stdin < notes.txt
laccd-courses-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `LACCD_COURSES_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `LACCD_COURSES_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
laccd-courses-pp-cli profile save briefing --json
laccd-courses-pp-cli --profile briefing colleges
laccd-courses-pp-cli profile list --json
laccd-courses-pp-cli profile show briefing
laccd-courses-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `laccd-courses-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/johnnyrobot/laccd-courses-pp-cli/cmd/laccd-courses-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add laccd-courses-pp-mcp -- laccd-courses-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which laccd-courses-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   laccd-courses-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `laccd-courses-pp-cli <command> --help`.

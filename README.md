# LACCD Course Catalog CLI

**Search every LACCD course and program across all 9 colleges from your terminal — with cross-college comparison, prerequisite chains, and offline search the per-college web catalog can't do.**

The Los Angeles Community College District publishes its public course and program catalog through eLumen, but the website only shows one college at a time and has no API, no export, and no CLI. This tool turns that catalog into a fast, scriptable, agent-native surface: search or browse ~9,664 approved courses and every degree/certificate program across all 9 colleges, pull a full Outline of Record as JSON, compare a course across colleges for articulation, and keep an offline mirror with full-text search. It also smooths the backend's quirks for you — the 1-indexed pagination (page 0 returns 502) and the eLumen tenant-host format are handled automatically.

Learn more at [LACCD Course Catalog](https://portalapi-laccd.elumenapp.com).

Created by [@johnnyrobot](https://github.com/johnnyrobot) (johnnyrobot).

## Install

`laccd-courses-pp-cli` is all you need. The separate `laccd-courses-pp-mcp` server is only for chat apps that can't run shell commands — see [Use with Claude Desktop](#use-with-claude-desktop).

### Go install

Requires Go 1.26.4 or newer:

```bash
go install github.com/johnnyrobot/laccd-courses-pp-cli/cmd/laccd-courses-pp-cli@latest
```

To also install the MCP server binary:

```bash
go install github.com/johnnyrobot/laccd-courses-pp-cli/cmd/laccd-courses-pp-mcp@latest
```

### Pre-built binary

Download a binary for your platform from the [latest release](https://github.com/johnnyrobot/laccd-courses-pp-cli/releases/latest).

On macOS, clear the Gatekeeper quarantine after downloading:

```bash
xattr -d com.apple.quarantine laccd-courses-pp-cli
```

On Unix, mark it executable:

```bash
chmod +x laccd-courses-pp-cli
```

### From source

```bash
git clone https://github.com/johnnyrobot/laccd-courses-pp-cli.git
cd laccd-courses-pp-cli
make build          # or: go build ./cmd/laccd-courses-pp-cli
make build-all      # CLI + MCP server
```

### Agent skill

`SKILL.md` in this repo is the agent skill. Point your agent at it directly, or copy it into your agent's skills directory (for Claude Code: `~/.claude/skills/pp-laccd-courses/SKILL.md`).

## Use with Claude Desktop

Claude Desktop has no shell, so it can't invoke the CLI. For that, `laccd-courses-pp-mcp` bridges the gap: a thin MCP server that shells out to the same CLI binary on your behalf.

Install the MCP binary:

```bash
go install github.com/johnnyrobot/laccd-courses-pp-cli/cmd/laccd-courses-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "laccd-courses": {
      "command": "laccd-courses-pp-mcp"
    }
  }
}
```

## Quick Start

```bash
# Confirm the CLI is wired up and the catalog API is reachable (no auth needed).
laccd-courses-pp-cli doctor --dry-run

# See the 9 LACCD colleges and their eLumen tenant hosts.
laccd-courses-pp-cli colleges list

# Search the whole district for courses matching a term.
laccd-courses-pp-cli courses search --query accounting --page-size 5

# Fetch one course's full Outline of Record (units, hours, requisites, SLOs).
laccd-courses-pp-cli courses get 40830

# Scope to one college — friendly codes (ELAC) map to the eLumen host for you.
laccd-courses-pp-cli courses search --tenant ELAC --query nursing --page-size 5

```

## Unique Features

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

## Recipes


### Full course outline as agent-friendly JSON

```bash
laccd-courses-pp-cli courses get 40830 --agent --select code,name,subject,number,status,tenant
```

The course detail is ~40KB; narrow the deeply nested Outline of Record to just the fields you need with dotted --select paths.

### Compare a course across the district

```bash
laccd-courses-pp-cli courses compare ACCTG 001 --diff-only
```

Fans out to all 9 colleges and shows only the units/requisites/transfer fields that differ — the rows that break a transfer.

### Browse a whole college's catalog

```bash
laccd-courses-pp-cli courses search --tenant pierce.elumenapp.com --page-size 50
```

Leave --query empty to browse every approved course at one college (here, LA Pierce), 50 per page.

### Prerequisite chain for a course

```bash
laccd-courses-pp-cli courses prereqs 30636
```

Prints the course's prerequisite, advisory, and corequisite chain from its structured requisite blocks.

### Find degree and certificate programs

```bash
laccd-courses-pp-cli programs search --query nursing --agent
```

Search programs across LACCD; --agent emits structured JSON for downstream advising tools.

## Usage

Run `laccd-courses-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `LACCD_COURSES_CONFIG_DIR`, `LACCD_COURSES_DATA_DIR`, `LACCD_COURSES_STATE_DIR`, or `LACCD_COURSES_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `LACCD_COURSES_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export LACCD_COURSES_HOME=/srv/laccd-courses
laccd-courses-pp-cli doctor
```

Under `LACCD_COURSES_HOME=/srv/laccd-courses`, the four dirs resolve to `/srv/laccd-courses/config`, `/srv/laccd-courses/data`, `/srv/laccd-courses/state`, and `/srv/laccd-courses/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `LACCD_COURSES_DATA_DIR` overrides an explicit `--home` for that kind. Use `LACCD_COURSES_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `LACCD_COURSES_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `laccd-courses-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### colleges

List the LACCD colleges (eLumen tenants) available in the catalog

- **`laccd-courses-pp-cli colleges`** - List the 9 LACCD colleges, their tenant codes, and catalog base URLs.

### config

Read the catalog's public configuration values

- **`laccd-courses-pp-cli config`** - List the public eLumen catalog configuration key/value pairs (review windows, presentation settings).

### courses

Search the LACCD course catalog and fetch full course outlines

- **`laccd-courses-pp-cli courses get`** - Fetch a single course by its numeric catalog id, including the full course outline (units, hours, requisites, outcomes, objectives, description).
- **`laccd-courses-pp-cli courses search`** - Search or browse approved courses across LACCD. Leave --query empty to browse a whole college (or all 9). Full-text matches course code, title, and description.

### programs

Search LACCD degree and certificate programs and fetch full program details

- **`laccd-courses-pp-cli programs get`** - Fetch a single program by its numeric id, including award type, credit ranges, requirements, and SLOs.
- **`laccd-courses-pp-cli programs search`** - Search or browse LACCD degree and certificate programs. Leave --query empty to browse a college's full program list. Full-text matches program name and description.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
laccd-courses-pp-cli colleges

# JSON for scripting and agents
laccd-courses-pp-cli colleges --json

# Filter to specific fields
laccd-courses-pp-cli colleges --json --select id,name,status

# Dry run — show the request without sending
laccd-courses-pp-cli colleges --dry-run

# Agent mode — JSON + compact + no prompts in one flag
laccd-courses-pp-cli colleges --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
laccd-courses-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `laccd-courses-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/laccd-courses-pp-cli/config.toml`; `--home`, `LACCD_COURSES_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **courses search returns HTTP 502** — The eLumen list endpoint is 1-indexed and flaky on cold starts; the CLI defaults --page to 1 (page 0 always 502s) and retries. If it persists, retry or narrow with --tenant <college-host>.
- **--tenant ELAC returns no results** — The wire value is the eLumen host. Use --tenant elac.elumenapp.com (run 'colleges list' for all 9), or the friendly code which the CLI maps for you.
- **courses compare / coverage / stats return empty** — These read the local mirror. Run 'laccd-courses-pp-cli sync' or a 'courses search' first to populate it, then re-run.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://laccd.elumenapp.com/public/courses
- Capture coverage: 10 API entries from 10 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Auth signals: none
- Candidate command ideas: get_courses — Derived from observed GET /public/courses/{course_id} traffic.; get_programs — Derived from observed GET /public/programs/{program_id} traffic.; list_configurations — Derived from observed GET /public/configurations traffic.; list_courses — Derived from observed GET /public/courses traffic.; list_programs — Derived from observed GET /public/programs traffic.; list_tenants — Derived from observed GET /public/tenants/ traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

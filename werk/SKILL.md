---
name: werk
description: >
  Local-first task, decision, and session tracking via the werk CLI.
  Use for all project planning, work decomposition, dependency management,
  and architectural decision recording. Activate when starting work sessions,
  filing tasks, or tracking progress on multi-session features.
compatibility: Requires the werk binary in PATH. Run werk init to set up.
allowed-tools: Bash(werk:*)
metadata:
  author: werk
  version: "0.2.2"
---

# Task Tracking

Use the `werk` CLI for all task and decision tracking. DB lives at `.werk/tasks.db`. Always pass `--agent` on all write commands.

Task-like items now expose both:
- `id` (internal hash primary key like `tk-a1b2c3`)
- `ref` (stable dotted ref like `1.2` or `1.2.1`)

Use refs by default in prompts/commands. Any command that accepts a task-like ID also accepts `<id-or-ref>`.

See [CLI Reference](references/CLI.md) for the full command list and output format.

## Setup

Install: `brew install zackbart/tap/werk` or `curl -fsSL https://raw.githubusercontent.com/zackbart/werk/main/install.sh | sh`

Initialize at the repo root (or monorepo root):

```
werk init
```

This creates `.werk/tasks.db`. All commands automatically walk up the directory tree to find it, so `werk` works from any subdirectory — just like `git`. In git worktrees, `werk` also checks the main worktree's `.werk/` as a fallback.

If `.werk/snapshot.json` exists at init time, it is automatically imported — restoring task history from a committed snapshot.

Running `werk init` on an existing project is safe — it upgrades the database schema, fixes `.gitignore` patterns, and registers the workspace. Returns `{"status": "upgraded"}`.

**Post-init checks:**

1. Check if any ancestor `.gitignore` has a `*.db` rule (common in Dolt/beads, Rails, and other projects). If so, add `!.werk/tasks.db` to the root `.gitignore` so the database is tracked by git.

---

## Session lifecycle

**Start every session:**

```
werk session start --with-context --agent
```

The `--with-context` flag returns the session object bundled with recent log, ready tasks, and in-progress tasks — everything you need to orient in one call.

Alternatively, without `--with-context`:

```
werk session start --agent
werk log --pretty --agent              # catch up on recent activity
werk task ready --agent
werk task list --status in_progress --agent
```

Use `werk log --verbose --pretty -n 10` if you need more context (notes, rationale, session details).

**End every session:**

```
werk session end --summary "<plain-English description of what was accomplished>" --agent
```

The summary should orient the next agent quickly — what was done, what was filed, what's left. Example: "Implemented hash ID generation, wired audit trail to all write commands, filed 2 new tasks discovered during work."

If `--summary` is omitted, an auto-summary is generated from audit entries during the session.

---

## Core workflow

1. `werk next --agent` — pick the highest-priority ready task and start it automatically
   - Or manually: `werk task ready --agent` then `werk task start <ref> --agent`
2. `werk subtask list --task <ref> --agent` — check for subtasks
3. Do the work. File subtasks for steps discovered along the way.
4. `werk task note <ref> "<text>" --agent` — append progress notes as you go
5. `werk subtask close <ref> --agent` — close each subtask as you finish it
6. `werk task close <ref> --agent` — close the task (all subtasks must be done first)
7. `werk task ready --agent` — check for newly unblocked work and repeat

### Bulk operations

Start, close, and reopen accept multiple refs:

```
werk task close 1.1 1.2 1.3 --agent
werk subtask start 1.1.1 1.1.2 --agent
```

---

## Issue classification

### Epic

An epic is a **complete, shippable feature or goal**. It answers "what are we building?" not "how."

**When to file:** The work is a distinct capability, will span multiple sessions, and decomposes into 3-10 tasks.

**Do not file an epic for:** a single bug fix, a single refactor, or anything completable in one session.

**Title** — noun phrase or goal statement:
- "User Authentication System"
- "CLI Dependency Commands"
- "Web UI Board View"
- "Database Migration Tooling"
- "Support OAuth Login"

**Notes** — the *why*. This is what an agent reads to understand scope before decomposing into tasks. Include:
- Motivation: why this feature exists, what problem it solves
- Acceptance criteria: what "done" looks like
- Links to design docs, specs, or relevant context
- Scope boundaries: what's explicitly out of scope

**Priority** — reflects business importance, not technical complexity. Use shorthand flags: `--critical` (P0), `--high` (P1), `--low` (P3).

```
werk epic create "User Authentication System" \
  --high \
  --notes "Users need to log in to access protected resources. Acceptance criteria: login/logout flow, password reset, session management. Out of scope: OAuth (separate epic)." \
  --agent
```

---

### Task

A task is a **concrete, completable unit of work within an epic**. It answers "what needs to happen for this epic to ship?"

**When to file:** It's a distinct concern within an epic, takes more than ~2 minutes, and has a clear done state. It could block or be blocked by other tasks.

**Do not file a task for:** something under 2 minutes, or purely exploratory work with no clear done state (use a decision instead).

**Title** — imperative verb phrase. Follow these conventions by work type:
- Feature work: "Implement X", "Add Y to Z"
- Bug fixes: "Fix X when Y"
- Tests: "Write tests for X"
- Chores: "Refactor X", "Update dependency Y"
- Docs: "Document X command"

**Notes** — operational context for whoever picks this up:
- Approach hints: "Use sha256 truncated to 6 chars, retry on collision"
- Code pointers: "See internal/db/id.go for the existing pattern"
- Known gotchas: "SQLite doesn't support concurrent writes, use a mutex"
- Edge cases to handle: "Empty title, duplicate names, max length"

**Priority** — can differ from parent epic. A critical bug fix task can live under a normal-priority epic. Use shorthand flags: `--critical` (P0), `--high` (P1), `--low` (P3).

```
werk task create "Implement hash ID generation" \
  --epic 1 \
  --high \
  --notes "Use sha256(title+timestamp)[:6] with prefix. Retry up to 5 times on collision. See spec.md ID Scheme section." \
  --agent
```

### Task notes and links

Append notes incrementally as you work — no need to rewrite the full notes field:

```
werk task note 1.1 "Discovered edge case: empty title produces collision. Added retry logic." --agent
```

Associate files and URLs with tasks for traceability:

```
werk task link 1.1 internal/db/id.go --agent
werk task link 1.1 https://github.com/org/repo/pull/42 --agent
werk task link 1.1 internal/db/id.go --remove --agent
```

---

### Subtask

A subtask is a **discrete step within a task**. It tracks individual progress within a larger unit of work.

**When to file:**
- A task has clearly separable steps that benefit from individual tracking
- A step could be done independently or in parallel
- You discover additional required steps during work that weren't originally scoped

**Do not file a subtask for:** steps under a minute, or work too granular to be worth auditing individually.

**Title** — specific and concrete. Say exactly what will be done:
- "Write schema migration SQL for users table"
- "Add rollback path for failed migrations"
- "Test ID generation against existing DB fixture"
- "Handle edge case: empty input on title field"
- "Wire audit trail to the UpdateTask function"

**Notes** — only when something is non-obvious about this specific step. Most subtasks don't need notes.

```
werk subtask create "Write schema migration SQL for users table" \
  --task 1.1 \
  --agent
```

---

### Decision

A decision is a **permanent record of a non-obvious technical or architectural choice**. Decisions are append-only — they cannot be closed or deleted.

**When to file:**
- You chose between two or more viable options
- The rationale wouldn't be obvious to someone reading the code later
- You want to prevent future agents or engineers from re-litigating the same choice

**Do not file a decision for:** obvious implementation details, or choices you may want to revisit (file a task with notes instead).

**Summary** — the choice, stated plainly: "Use X instead of Y", "Store Z as JSON array"

**Rationale** — the why: constraints considered, alternatives rejected, tradeoffs accepted.

```
werk decision create "Use pure-Go SQLite driver instead of CGO" \
  --rationale "Avoids CGO compilation complexity and cross-compilation issues. modernc.org/sqlite has full feature parity with the C version. Tradeoff: ~10% slower on write-heavy workloads, acceptable for this use case." \
  --agent
```

---

## Priority levels

| Value | Label    | Flag         | When to use                                                |
|-------|----------|--------------|------------------------------------------------------------|
| 0     | Critical | `--critical` | Security issues, data loss, broken builds, release blockers |
| 1     | High     | `--high`     | Major features, important bugs affecting core functionality |
| 2     | Normal   | (default)    | Standard work items (default if omitted)                   |
| 3     | Low      | `--low`      | Polish, optimisation, nice-to-haves                        |
| 4     | Backlog  |              | Future ideas, not scheduled                                |

---

## Dependencies

Dependencies model "blocks" relationships. The upstream task must be done before the downstream task can start.

```
werk dep add 1.1 1.2 --agent
werk dep remove 1.1 1.2 --agent
werk dep list 1.2 --agent                     # shows what blocks it and what it blocks
```

Before starting a task, check its blockers: `werk dep list <id-or-ref> --agent`. Never work on a task with open blockers.

Dependencies can cross epic boundaries. Cycles are rejected automatically.

---

## Reparenting (move)

Move tasks between epics or subtasks between tasks:

```
werk task move 1.1 --epic 2 --agent
werk subtask move 1.1.1 --task 2.1 --agent
```

Refs are reassigned automatically. Children refs update recursively.

---

## Archive

Archive completed epics/tasks to hide them from default list views:

```
werk epic archive 1 --agent
werk task archive 1.1 --agent
werk epic list --archived                     # show archived items
werk epic unarchive 1 --agent
```

---

## Closing, reopening, and deleting

- All **subtasks** must be closed before their parent **task** can be closed
- All **tasks** must be closed before their parent **epic** can be closed
- Closing sets `status=done` and records `closed_at`
- **Reopening** sets `status=open` and clears `closed_at`:
  ```
  werk task reopen <id-or-ref> --agent
  ```
- **Deleting** permanently removes a row and its audit history. Use for duplicates and mistakes only — not for completed work:
  ```
  werk task delete <id-or-ref> --agent             # only works on open items
  werk task delete <id-or-ref> --force --agent     # works on any status
  ```
- Children must be deleted before parents (subtasks -> task -> epic)

---

## Workspaces

Register named workspaces to target `.werk/` databases beyond the nearest one:

```
werk workspace add myproject /path/to/project
werk workspace list
werk workspace remove myproject
```

Then use `--ws <name>` on any command:

```
werk --ws myproject status
werk --ws myproject task list
```

---

## Export & import

```
werk export > backup.json
werk import backup.json
```

Export produces a complete JSON snapshot. Import uses `INSERT OR IGNORE` so it's safe for merging.

---

## Snapshot workflow

Commit a snapshot so fresh clones inherit task history:

```
werk export > .werk/snapshot.json
git add .werk/snapshot.json && git commit -m "Update werk snapshot"
```

On `werk init` in a fresh clone, the snapshot is automatically imported. Returns `{"status": "initialized", "snapshot": "restored"}`.

The binary `tasks.db` stays gitignored. The JSON snapshot is the portable, committable representation. Update it periodically (e.g. before merging feature branches).

---

## Diff

See what changed since the last session:

```
werk diff                              # changes since last ended session
werk diff --since <session-id>         # changes since a specific session
```

---

## Handoff

Use compact handoff packets to transfer context between agents:

```
werk handoff <id-or-ref> --compact
```

The packet includes item identity/core metadata, dependencies + blockers, child items, recent decisions, and recent audit context.

---

## Error handling

On failures, parse JSON with stable machine codes:

```
{"code":"ERR_*","message":"<message>"}
```

Do not parse free-form message strings for control flow; branch on `code`.

---

## Invariants — never break these

- Never work on a task with open blockers
- Prefer closing over deleting — delete only duplicates and mistakes
- Close/delete children before parents (subtasks -> task -> epic)
- Always record decisions when making non-obvious technical choices
- Always start and end sessions with the session commands
- Always pass `--agent` on all write commands
- File discovered work immediately — don't leave it untracked

---

## Quick reference

```
werk next --agent                       Pick highest-priority ready task and start it
werk log --pretty                       What happened recently?
werk log --pretty --task 1.1            Activity for a specific task
werk log --pretty --verbose -n 10       Detailed recent activity
werk task ready                         What can I work on right now?
werk task list --status in_progress     What is currently in flight?
werk task find "login"                  Search tasks by title/notes
werk epic list                          What features exist?
werk dep list <id-or-ref>              What blocks this / what does this block?
werk audit <id-or-ref>                 Full history of a single task
werk handoff <id-or-ref> --compact     Context packet for agent handoff
werk status                             Project summary (with active session)
werk decision list                      Architectural decision log
werk diff                               Changes since last session
werk export > backup.json               Full data export
```

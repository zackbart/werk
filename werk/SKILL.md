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
  version: "0.1.1"
---

# Task Tracking

Use the `werk` CLI for all task and decision tracking. DB lives at `.werk/tasks.db`. Always pass `--agent` on all write commands.

See [CLI Reference](references/CLI.md) for the full command list and output format.

## Setup

Install: `brew install zackbart/tap/werk` or `curl -fsSL https://raw.githubusercontent.com/zackbart/werk/main/install.sh | sh`

Initialize at the repo root (or monorepo root):

```
werk init
```

This creates `.werk/tasks.db`. All commands automatically walk up the directory tree to find it, so `werk` works from any subdirectory — just like `git`.

**Post-init checks:**

1. Verify `.werk/.gitignore` uses correct relative patterns (`*.db-wal`, `*.db-shm`, `session.lock`) — not `.werk/`-prefixed paths. Versions ≤ 0.1.1 generated broken patterns; if you see `.werk/*.db-wal`, fix them.
2. Check if any ancestor `.gitignore` has a `*.db` rule (common in Dolt/beads, Rails, and other projects). If so, add `!.werk/tasks.db` to the root `.gitignore` so the database is tracked by git.

---

## Session lifecycle

**Start every session:**

```
werk session start --agent
werk task ready --agent
werk task list --status in_progress --agent
```

**End every session:**

```
werk session end --summary "<plain-English description of what was accomplished>" --agent
```

The summary should orient the next agent quickly — what was done, what was filed, what's left. Example: "Implemented hash ID generation, wired audit trail to all write commands, filed 2 new tasks discovered during work."

---

## Core workflow

1. `werk task ready --agent` — find work with no open blockers
2. `werk task start <id> --agent` — claim it
3. `werk subtask list --task <id> --agent` — check for subtasks
4. Do the work. File subtasks for steps discovered along the way.
5. `werk subtask close <id> --agent` — close each subtask as you finish it
6. `werk task close <id> --agent` — close the task (all subtasks must be done first)
7. `werk task ready --agent` — check for newly unblocked work and repeat

---

## Issue classification

### Epic

An epic is a **complete, shippable feature or goal**. It answers "what are we building?" not "how."

**When to file:** The work is a distinct capability, will span multiple sessions, and decomposes into 3–10 tasks.

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

**Priority** — reflects business importance, not technical complexity.

```
werk epic create "User Authentication System" \
  --priority 1 \
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

**Priority** — can differ from parent epic. A critical bug fix task can live under a normal-priority epic.

```
werk task create "Implement hash ID generation" \
  --epic ep-a1b2c3 \
  --priority 1 \
  --notes "Use sha256(title+timestamp)[:6] with prefix. Retry up to 5 times on collision. See spec.md ID Scheme section." \
  --agent
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
  --task tk-d4e5f6 \
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

| Value | Label    | When to use                                                |
|-------|----------|------------------------------------------------------------|
| 0     | Critical | Security issues, data loss, broken builds, release blockers |
| 1     | High     | Major features, important bugs affecting core functionality |
| 2     | Normal   | Standard work items (default if omitted)                   |
| 3     | Low      | Polish, optimisation, nice-to-haves                        |
| 4     | Backlog  | Future ideas, not scheduled                                |

---

## Dependencies

Dependencies model "blocks" relationships. The upstream task must be done before the downstream task can start.

```
werk dep add tk-implement tk-tests --agent    # tk-implement blocks tk-tests
werk dep remove tk-implement tk-tests --agent
werk dep list tk-tests --agent                # shows what blocks it and what it blocks
```

Before starting a task, check its blockers: `werk dep list <id> --agent`. Never work on a task with open blockers.

Dependencies can cross epic boundaries. Cycles are rejected automatically.

---

## Closing and deleting

- All **subtasks** must be closed before their parent **task** can be closed
- All **tasks** must be closed before their parent **epic** can be closed
- Closing sets `status=done` and records `closed_at` — this is permanent
- **Deleting** permanently removes a row and its audit history. Use for duplicates and mistakes only — not for completed work:
  ```
  werk task delete <id> --agent             # only works on open items
  werk task delete <id> --force --agent     # works on any status
  ```
- Children must be deleted before parents (subtasks → task → epic)

---

## Invariants — never break these

- Never work on a task with open blockers
- Prefer closing over deleting — delete only duplicates and mistakes
- Close/delete children before parents (subtasks → task → epic)
- Always record decisions when making non-obvious technical choices
- Always start and end sessions with the session commands
- Always pass `--agent` on all write commands
- File discovered work immediately — don't leave it untracked

---

## Migrating from Beads

If this project uses [Beads](https://github.com/steveyegge/beads) (`.beads/` directory exists), see [Migration Guide](references/MIGRATION.md) for a full walkthrough. The short version:

1. Export from beads: `bd export` or `bd list --format json`
2. Create matching epics, tasks, and subtasks in werk
3. Re-create `blocks` dependencies (other dep types are dropped)
4. Migrate compacted summaries as decisions
5. Record the migration as a decision

Migration is lossy by design — messages, labels, molecules, wisps, and named dependency types beyond "blocks" are not carried over.

---

## Quick reference

```
werk task ready                       What can I work on right now?
werk task list --status in_progress   What is currently in flight?
werk epic list                        What features exist?
werk dep list <id>                    What blocks this / what does this block?
werk audit <id>                       Full history of a task
werk status                           Project summary
werk decision list                    Architectural decision log
```

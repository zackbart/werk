# Issue Classification Guide

Read this when creating new epics, tasks, subtasks, or decisions. It covers what each type is for, when to file one, and how to write good titles and notes.

## Epic

An epic is a **complete, shippable feature or goal**. It answers "what are we building?" not "how."

**When to file:** The work is a distinct capability, will span multiple sessions, and decomposes into 3-10 tasks.

**Do not file an epic for:** a single bug fix, a single refactor, or anything completable in one session.

**Title** — noun phrase or goal statement:
- "User Authentication System"
- "CLI Dependency Commands"
- "Web UI Board View"
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

## Task

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
  --epic 1 \
  --high \
  --notes "Use sha256(title+timestamp)[:6] with prefix. Retry up to 5 times on collision. See spec.md ID Scheme section." \
  --agent
```

### Task notes and links

Append notes incrementally as you work — no need to rewrite the full notes field:

```
werk task note tk-a1b2c3 "Discovered edge case: empty title produces collision. Added retry logic." --agent
```

Associate files and URLs with tasks for traceability:

```
werk task link tk-a1b2c3 internal/db/id.go --agent
werk task link tk-a1b2c3 https://github.com/org/repo/pull/42 --agent
werk task link tk-a1b2c3 internal/db/id.go --remove --agent
```

---

## Subtask

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

**Notes** — only when something is non-obvious about this specific step. Most subtasks don't need notes.

```
werk subtask create "Write schema migration SQL for users table" \
  --task tk-a1b2c3 \
  --agent
```

---

## Decision

A decision is a **permanent record of a non-obvious technical or architectural choice**. Decisions are append-only — they cannot be closed or deleted.

**When to file:**
- You chose between two or more viable options
- The rationale wouldn't be obvious to someone reading the code later
- You want to prevent future agents or engineers from re-litigating the same choice

**Do not file a decision for:** obvious implementation details, forced choices where no real alternative existed, or choices you may want to revisit (file a task with notes instead). When in doubt, ask: "would someone later wonder *why* we did this?" If yes, file it. If the answer is self-evident from the code, skip it.

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
werk dep add tk-a1b2c3 tk-d4e5f6 --agent
werk dep remove tk-a1b2c3 tk-d4e5f6 --agent
werk dep list tk-d4e5f6 --agent               # shows what blocks it and what it blocks
```

Before starting a task, check its blockers: `werk dep list <id> --agent`. Never work on a task with open blockers.

Dependencies can cross epic boundaries. Cycles are rejected automatically.

---

## Reparenting (move)

Move tasks between epics or subtasks between tasks:

```
werk task move tk-a1b2c3 --epic ep-d4e5f6 --agent
werk subtask move st-a1b2c3 --task tk-d4e5f6 --agent

---

## Archive

Archive completed epics/tasks to hide them from default list views:

```
werk epic archive ep-a1b2c3 --agent
werk task archive tk-a1b2c3 --agent
werk epic list --archived                     # show archived items
werk epic unarchive ep-a1b2c3 --agent
```

---

## Closing, reopening, and deleting

- All **subtasks** must be closed before their parent **task** can be closed
- All **tasks** must be closed before their parent **epic** can be closed
- Closing sets `status=done` and records `closed_at`
- **Reopening** sets `status=open` and clears `closed_at`:
  ```
  werk task reopen <id> --agent
  ```
- **Deleting** permanently removes a row and its audit history. Use for duplicates and mistakes only — not for completed work:
  ```
  werk task delete <id> --agent             # only works on open items
  werk task delete <id> --force --agent     # works on any status
  ```
- Children must be deleted before parents (subtasks -> task -> epic)

---

## Setup

Install: `brew install zackbart/tap/werk` or `curl -fsSL https://raw.githubusercontent.com/zackbart/werk/main/install.sh | sh`

Initialize at the repo root (or monorepo root):

```
werk init
```

This creates `.werk/tasks.db`. All commands automatically walk up the directory tree to find it, so `werk` works from any subdirectory — just like `git`. In git worktrees, `werk` also checks the main worktree's `.werk/` as a fallback.

If `.werk/snapshot.json` exists at init time, it is automatically imported — restoring task history from a committed snapshot.

Running `werk init` on an existing project is safe — it upgrades the database schema, fixes `.gitignore` patterns, and registers the werkspace. Returns `{"status": "upgraded"}`.

Use `--name <name>` to override the werkspace name (default: directory basename).

**Gitignore setup:**

The `.werk/` directory should be ignored in your root `.gitignore`. The binary `tasks.db` is not committed — the JSON snapshot is the portable, committable representation.

`werk init` adds `.werk/` to your root `.gitignore` automatically. If you need to set it up manually:

```
# .gitignore (root)
.werk/
```

---

## Werkspaces

Register named werkspaces to target `.werk/` databases beyond the nearest one:

```
werk werkspace add myproject /path/to/project
werk werkspace list
werk werkspace remove myproject
```

Then use `--ws <name>` on any command:

```
werk --ws myproject status
werk --ws myproject task list
```

Werkspaces are stored at `~/.config/werk/werkspaces.json`. `werk init` auto-registers the current directory using its basename as the werkspace name.

---

## Export, import, and snapshots

```
werk export > backup.json
werk import backup.json
```

Export produces a complete JSON snapshot. Import uses `INSERT OR IGNORE` so it's safe for merging.

### Snapshot workflow

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
werk handoff <id> --compact
```

The packet includes item identity/core metadata, dependencies + blockers, child items, recent decisions, and recent audit context.

---

## Error handling

On failures, parse JSON with stable machine codes:

```
{"code":"ERR_*","message":"<message>"}
```

Do not parse free-form message strings for control flow; branch on `code`.

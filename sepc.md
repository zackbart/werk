# Werk Spec
> Status: Ready for implementation
> Last updated: 2026-03-04

---

## Overview

A local-first task and decision tracker for AI-assisted development. SQLite as the source of truth, a CLI as the **sole interface** for both engineers and agents, and a lightweight local web UI for visualization.

Designed to be dead simple but with the quality-of-life things modern developers expect. No daemon. No server required to use the CLI. Single binary.

---

## Problem

Coding agents have a fixed context window. On long-horizon tasks they lose track of what was planned, what's done, what's blocked, and what's next. Existing solutions (e.g. Beads) solve this well but bring infrastructure complexity and lock-in. Werk solves the same problem with a minimal, ownable footprint.

---

## Design Principles

- **Local-first.** Everything lives in a SQLite file in your repo. No dependencies to run.
- **CLI is the only interface.** Agents and engineers use identical commands. No raw SQL ever touches the DB directly — the CLI enforces all invariants (audit trail, hash generation, hierarchy rules).
- **Audit everything.** Every state change is logged with who made it and when. History is never destroyed — only closed.
- **JSON by default.** All commands output JSON (agent-friendly). `--pretty` flag for human-readable output.
- **Single binary.** CLI and web server in one executable.
- **Fail loudly.** Non-zero exit codes and `{"error": "message"}` on all failures. No silent failures.

---

## Language & Implementation

**Go.** Single binary, excellent SQLite bindings, fast CLI startup, easy cross-platform distribution. Use `modernc.org/sqlite` (pure Go, no CGO required).

**Project structure:**
```
werk/
├── cmd/werk/         # CLI entrypoint
├── internal/
│   ├── db/           # SQLite layer, migrations
│   ├── models/       # types: Task, Decision, Session, AuditEntry
│   ├── commands/     # one file per noun: epic.go, task.go, dep.go...
│   └── server/       # web UI server
├── web/              # single-file HTML/JS/CSS web UI (embedded via go:embed)
├── .werk/            # created at init time, contains tasks.db
└── SKILL.md          # agent skill file (see below)
```

---

## Data Model

### tasks

```sql
CREATE TABLE tasks (
  id          TEXT PRIMARY KEY,
  parent_id   TEXT REFERENCES tasks(id),
  type        TEXT NOT NULL CHECK(type IN ('epic','task','subtask')),
  title       TEXT NOT NULL,
  status      TEXT NOT NULL DEFAULT 'open'
                CHECK(status IN ('open','in_progress','blocked','done')),
  priority    INTEGER NOT NULL DEFAULT 2
                CHECK(priority BETWEEN 0 AND 4),
  notes       TEXT,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME,
  closed_at   DATETIME
);
```

### dependencies

```sql
CREATE TABLE dependencies (
  upstream_id   TEXT NOT NULL REFERENCES tasks(id),
  downstream_id TEXT NOT NULL REFERENCES tasks(id),
  PRIMARY KEY (upstream_id, downstream_id),
  CHECK (upstream_id != downstream_id)
);
```

`upstream blocks downstream`. Single relationship type only.

### audit

```sql
CREATE TABLE audit (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id     TEXT NOT NULL REFERENCES tasks(id),
  field       TEXT NOT NULL,
  old_value   TEXT,
  new_value   TEXT,
  changed_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  changed_by  TEXT NOT NULL CHECK(changed_by IN ('agent','human'))
);
```

Every write through the CLI writes a corresponding audit row. No write path should skip this.

### decisions

```sql
CREATE TABLE decisions (
  id          TEXT PRIMARY KEY,
  summary     TEXT NOT NULL,
  rationale   TEXT,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_by  TEXT NOT NULL CHECK(created_by IN ('agent','human'))
);
```

Decisions are not tasks. They are not completable or closeable. Permanent append-only log.

### sessions

```sql
CREATE TABLE sessions (
  id            TEXT PRIMARY KEY,
  started_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  ended_at      DATETIME,
  summary       TEXT,
  tasks_touched TEXT  -- JSON array of task IDs e.g. ["tk-a1b2c3","tk-d4e5f6"]
);
```

A session represents one agent or human work period — the span between `werk session start` and `werk session end`. It is a coarse-grained complement to the per-field audit trail: where the audit trail answers "what changed and when", sessions answer "what did this work period accomplish".

**Lifecycle:**

1. `werk session start` creates a session row and writes the session ID to `.werk/session.lock`
2. Every subsequent CLI write command detects the lockfile and appends its task ID to the active session's `tasks_touched` list
3. `werk session end --summary "..."` finalises the row (sets `ended_at`, writes the summary, removes the lockfile)

**Lockfile:** `.werk/session.lock` contains the active session ID as plain text. It is created on `start` and deleted on `end`. If a lockfile already exists when `start` is called, the CLI errors — only one active session at a time. If no lockfile exists, write commands still succeed (sessions are optional, not enforced).

**`tasks_touched`** is updated in-place on every write by reading the current JSON array, appending the task ID if not already present, and writing it back. Deduped — a task touched 10 times appears once.

**The summary** is written by the agent at session end and should be a plain-English description of what was accomplished: "Implemented hash ID generation, wired audit trail to all write commands, filed 2 new tasks discovered during work." This is what a future agent reads at session start to get oriented quickly without trawling individual audit rows.

**Add to `.gitignore`:** `.werk/session.lock`

---

## ID Scheme

Short 6-character alphanumeric hashes derived from `sha256(title + timestamp)[:6]`, collision-checked against existing IDs before insert. Retry up to 5 times with different salts on collision.

| Type     | Prefix | Example      |
|----------|--------|--------------|
| Epic     | `ep-`  | `ep-a1b2c3`  |
| Task     | `tk-`  | `tk-d4e5f6`  |
| Subtask  | `st-`  | `st-g7h8i9`  |
| Decision | `dc-`  | `dc-j0k1l2`  |
| Session  | `ss-`  | `ss-m3n4o5`  |

---

## Hierarchy Rules

```
Epic        → no parent, represents a full feature or goal
└── Task    → parent is an epic, represents a concrete unit of work
    └── Subtask → parent is a task, represents a step within that work
```

**Enforcement:**
- Epics may not have a parent
- Tasks must have a parent of type `epic`
- Subtasks must have a parent of type `task`
- Max depth is 3 levels — no sub-subtasks
- An epic is implicitly considered blocked if any child task is not `done` (derived at query time, never stored as a dependency)
- All subtasks must be closed before their parent task can be closed
- All tasks must be closed before their parent epic can be closed

**Dependencies** are separate from hierarchy and can cross hierarchy boundaries freely.

---

## Priority Levels

| Value | Label    | When to use |
|-------|----------|-------------|
| 0     | Critical | Security issues, data loss, broken builds, release blockers |
| 1     | High     | Major features, important bugs affecting core functionality |
| 2     | Normal   | Standard work items (default) |
| 3     | Low      | Polish, optimisation, nice-to-haves |
| 4     | Backlog  | Future ideas, not scheduled |

---

## Issue Classification Guide

This section defines what belongs at each level. Agents must follow this when decomposing work.

### Epic

An epic represents a **complete, shippable feature or goal**. It is the answer to "what are we building?" not "how are we building it."

**File an epic when:**
- The work represents a distinct capability recognisable as a feature
- It will require multiple sessions to complete
- It can be decomposed into 3–10 tasks
- Examples: "User Authentication System", "CLI dep commands", "Web UI board view", "Database migration tooling"

**Epic fields:**
- `title` — noun phrase or goal statement: "User Authentication" or "Support OAuth login"
- `notes` — the *why*: motivation, acceptance criteria, links to design docs. This is what an agent reads to understand scope before decomposing.
- `priority` — reflects business importance, not technical complexity

**Do not file an epic for:** a single bug fix, a single refactor, or work completable in one session.

---

### Task

A task represents a **concrete, completable unit of work within an epic**. It answers "what needs to happen for this epic to ship?"

**File a task when:**
- It represents a distinct concern within an epic
- It will take more than ~2 minutes to complete
- It could block or be blocked by other tasks
- Examples: "Implement hash ID generation", "Write migration script", "Add audit trail to all write commands"

**Task title conventions** — use an imperative verb phrase:
- Feature work: "Implement X", "Add Y to Z"
- Bug fixes: "Fix X when Y"
- Tests: "Write tests for X"
- Chores: "Refactor X", "Update dependency Y"
- Docs: "Document X command"

**Task fields:**
- `title` — imperative verb phrase (see above)
- `notes` — approach hints, relevant code pointers, known gotchas
- `priority` — can differ from parent epic priority
- `epic` — required

**Do not file a task for:** something under 2 minutes, or purely exploratory work with no clear done state (use a decision instead).

---

### Subtask

A subtask represents **a discrete step within a task**.

**File a subtask when:**
- A task has clearly separable steps that benefit from individual tracking
- A step could be done independently or in parallel with other steps
- Discovery during work reveals additional required steps not originally scoped
- Examples: "Write schema migration SQL", "Add rollback path", "Test against existing DB fixture"

**Subtask fields:**
- `title` — specific and concrete: "Write X", "Test Y against Z", "Handle edge case: empty input"
- `notes` — only if there is something non-obvious about this specific step
- `task` — required

**Do not file a subtask for:** steps under a minute, or work too granular to be worth auditing individually.

---

### Decision

A decision is a **permanent record of a non-obvious technical or architectural choice**.

**File a decision when:**
- You chose between two or more viable options
- The rationale would not be obvious to someone reading the code later
- You want to prevent future agents or engineers from re-litigating the choice
- Examples: "Use pure-Go SQLite driver instead of CGO", "Hash IDs over sequential integers", "Blocks-only dependency type"

**Decision fields:**
- `summary` — the choice, stated plainly: "Use X instead of Y", "Store Z as JSON array"
- `rationale` — why: constraints considered, alternatives rejected, tradeoffs accepted

**Do not file a decision for:** obvious implementation details, or choices you may want to revisit (file a task with notes instead).

---

## CLI Specification

### Global flags

```
--db <path>     override DB path (default: .werk/tasks.db)
--pretty        human-readable output instead of JSON
--agent         sets changed_by="agent" on all writes (default: human)
--help          show help
--version       show version
```

`WERK_DB` environment variable also overrides DB path.

### Commands

**Init & Status**
```
werk init                             initialise .werk/tasks.db in current directory
werk status                           summary: X open, Y in_progress, Z blocked, W done
werk serve [--port 8080]              start the local web UI (default port 8080)
werk migrate --from beads [--path ./] import from Beads export
```

**Epics**
```
werk epic create "<title>" [--priority 0-4] [--notes "<text>"]
werk epic list [--status open|in_progress|done|all]
werk epic show <id>
werk epic update <id> [--title "<text>"] [--priority 0-4] [--notes "<text>"]
werk epic close <id>
```

**Tasks**
```
werk task create "<title>" --epic <id> [--priority 0-4] [--notes "<text>"]
werk task list [--epic <id>] [--status open|in_progress|done|blocked|all]
werk task show <id>
werk task update <id> [--title "<text>"] [--priority 0-4] [--notes "<text>"]
werk task start <id>
werk task block <id>
werk task close <id>
werk task ready                       list all tasks with no open blockers
```

**Subtasks**
```
werk subtask create "<title>" --task <id> [--priority 0-4] [--notes "<text>"]
werk subtask list --task <id>
werk subtask show <id>
werk subtask update <id> [--title "<text>"] [--notes "<text>"]
werk subtask start <id>
werk subtask close <id>
```

**Dependencies**
```
werk dep add <upstream-id> <downstream-id>     upstream blocks downstream
werk dep remove <upstream-id> <downstream-id>
werk dep list <id>                             what blocks this + what this blocks
```

**Decisions**
```
werk decision create "<summary>" [--rationale "<text>"]
werk decision list
werk decision show <id>
```

**Sessions**
```
werk session start [--summary "<text>"]
werk session end [--summary "<text>"]
werk session list
werk session show <id>
```

**Audit**
```
werk audit <task-id>                  full change history for a task
```

### Output format

Every command returns JSON. Arrays for list commands, objects for show/create/update.

Example task object:
```json
{
  "id": "tk-a1b2c3",
  "type": "task",
  "title": "Implement hash ID generation",
  "status": "open",
  "priority": 1,
  "epic_id": "ep-d4e5f6",
  "blockers": [],
  "notes": "Use sha256(title+timestamp)[:6], retry on collision",
  "created_at": "2026-03-04T10:00:00Z",
  "updated_at": null,
  "closed_at": null
}
```

`werk status` output:
```json
{
  "open": 12,
  "in_progress": 3,
  "blocked": 2,
  "done": 47,
  "decisions": 8,
  "sessions": 14
}
```

Errors always return `{"error": "<message>"}` with a non-zero exit code.

---

## Web UI

Started with `werk serve`. Embedded as a single HTML file compiled into the binary via `go:embed`.

**Board view** — kanban columns: open / in_progress / blocked / done. Cards grouped by epic. Each card shows title, ID, priority badge, blocker count. Click to open audit drawer.

**Graph view** — D3 force-directed dependency graph. Nodes colored by status. Epics as clusters. Dependency arrows cross cluster boundaries where needed. Click a node to open audit drawer.

**Audit drawer** — slides in on task click. Full audit trail: timestamp, field, old value, new value, changed_by.

**Decisions panel** — separate tab, decisions listed chronologically with summary and rationale.

Read-only in v1. No writes from the UI.

---

## DB Location & Portability

- Default: `.werk/tasks.db` relative to current working directory
- Override: `--db <path>` flag or `WERK_DB` env var
- The `.werk/` directory should be gitignored — the binary `tasks.db` is not committed
- Use `werk export > .werk/snapshot.json` to commit a portable JSON snapshot instead
- On `werk init` in a fresh clone, the snapshot is automatically imported

---

## Migration

`werk migrate --from beads [--path <dir>]` imports from a Beads export.

Supported inputs: Dolt SQL dump (`bd dolt dump`) or `.beads/` directory.

| Beads concept | Maps to |
|---|---|
| epic | epic |
| task / feature / bug / chore | task |
| subtask | subtask |
| blocks dependency | dep |
| relates_to / duplicates / supersedes | dropped |
| messages / threads | dropped |
| compacted summaries | decisions |

Migration is lossy by design.

---

## SKILL.md

Place this file at `.werk/SKILL.md` in the repo. Reference it from `CLAUDE.md`.

```markdown
# Task Tracking Skill

Use the `werk` CLI for all task and decision tracking.
DB lives at `.werk/tasks.db`.

---

## Session lifecycle

**Start every session:**
werk session start --agent
werk task ready --agent
werk task list --status in_progress --agent

**End every session:**
werk session end --summary "<what was done>" --agent

---

## Core workflow

1. Find ready work:   werk task ready --agent
2. Claim a task:      werk task start <id> --agent
3. Check subtasks:    werk subtask list --task <id> --agent
4. Close subtasks:    werk subtask close <id> --agent  (each one as done)
5. Close task:        werk task close <id> --agent
6. Repeat:            werk task ready --agent  (check for newly unblocked work)

---

## Creating work

Decompose new features into epics and tasks before starting.

  werk epic create "Feature name" --priority 1 --notes "Why + acceptance criteria" --agent
  werk task create "Implement X" --epic ep-xxxx --priority 1 --agent
  werk task create "Write tests for X" --epic ep-xxxx --priority 2 --agent
  werk dep add tk-implement tk-tests --agent   # implement before tests

When you discover additional work during a session, file it immediately:
  werk task create "Handle edge case: X" --epic ep-xxxx --priority 2 --agent

---

## What to file at each level

Epic   → distinct shippable feature, multiple sessions, 3–10 tasks
         title: noun phrase or goal — "User Authentication", "Support OAuth login"
         notes: the why, acceptance criteria, design doc links

Task   → concrete unit of work, takes >2 mins, clear done state
         title: imperative verb — "Implement X", "Fix Y", "Write tests for Z"
         notes: approach hints, code pointers, gotchas

Subtask → discrete step within a task, benefits from individual tracking
          title: specific and concrete — "Write migration SQL", "Handle edge case: empty input"

Decision → non-obvious technical choice between real alternatives
           summary: the choice — "Use X instead of Y"
           rationale: why, what was rejected, tradeoffs

---

## Recording decisions

File a decision for any non-obvious technical choice:
  werk decision create "Use pure-Go SQLite driver instead of CGO" \
    --rationale "Avoids CGO compilation complexity; modernc.org/sqlite has full feature parity" \
    --agent

---

## Invariants — never break these

- Never work on a task with open blockers  →  werk dep list <id> to check
- Never delete rows — only close them
- Close all subtasks before closing their parent task
- Close all tasks before closing their parent epic
- Always record decisions when making non-obvious technical choices
- Always start and end sessions with the session commands
- Always pass --agent on all writes

---

## Quick reference

werk task ready                       what can I work on right now?
werk task list --status in_progress   what is currently in flight?
werk epic list                        what features exist?
werk dep list <id>                    what blocks this / what does this block?
werk audit <id>                       full history of a task
werk status                           project summary
werk decision list                    architectural decision log
```

---

## CLAUDE.md Block

```markdown
## Task Tracking

Read `.werk/SKILL.md` before starting any work. It contains the complete interface.

Quick start:
1. `werk session start --agent`
2. `werk task ready --agent`
3. Follow the skill workflow
```

---

## Out of Scope

- Stealth mode
- Multi-agent atomic task claiming
- Remote sync / federation
- Named dependency types beyond "blocks"
- Compaction / summarization
- Messaging / threading
- Contributor / maintainer roles
- Writes from the web UI (permanently read-only)

The schema supports most of these as additive changes if needed later.

---

## Decisions Log

| Decision | Choice | Rationale |
|---|---|---|
| Name | `werk` | Short, memorable, work-adjacent, no active conflicts |
| Language | Go | Single binary, pure-Go SQLite, fast CLI startup |
| SQLite driver | `modernc.org/sqlite` | No CGO, full feature parity |
| Web UI writes | Read-only permanently | CLI is the single source of truth — no split-brain risk |
| `werk task ready` scope | Tasks only, not subtasks | Agents claim tasks and manage their own subtasks; subtasks are not independently schedulable units |
| Session concurrency | One active session at a time | Solo-use tool; lockfile enforces this naturally |

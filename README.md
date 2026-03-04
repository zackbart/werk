# werk

A local-first task and decision tracker for AI-assisted development. SQLite-backed, CLI-driven, single binary.

Designed to give coding agents persistent memory across sessions without infrastructure complexity. Agents and humans use the same CLI — no special APIs, no daemons, no dependencies.

## Why

Coding agents have a fixed context window. On long-horizon tasks they lose track of what was planned, what's done, what's blocked, and what's next. Werk solves this with a minimal, ownable footprint: a SQLite file in your repo and a CLI that enforces structure.

## Install

**Homebrew (macOS/Linux):**

```bash
brew install zackbart/tap/werk
```

**Shell script:**

```bash
curl -fsSL https://raw.githubusercontent.com/zackbart/werk/main/install.sh | sh
```

**Go:**

```bash
go install github.com/zackbart/werk/cmd/werk@latest
```

**From source:**

```bash
git clone https://github.com/zackbart/werk.git
cd werk
go build -o werk ./cmd/werk/
```

Or download a binary directly from [GitHub Releases](https://github.com/zackbart/werk/releases).

## Quick start

```bash
werk init                                          # creates .werk/tasks.db
werk epic create "User Auth" --priority 1 --notes "Login, logout, sessions"
werk task create "Implement login endpoint" --epic ep-a1b2c3
werk task create "Write auth tests" --epic ep-a1b2c3
werk dep add tk-abc123 tk-def456                   # login blocks tests
werk task start tk-abc123
werk task close tk-abc123
werk task ready                                    # shows unblocked work
werk status                                        # project summary
```

## Hierarchy

```
Epic        → a complete shippable feature or goal (3–10 tasks)
└── Task    → a concrete unit of work within an epic
    └── Subtask → a discrete step within a task
```

Epics have no parent. Tasks must belong to an epic. Subtasks must belong to a task. Max depth is 3.

## Commands

```
werk init                              Initialize .werk/tasks.db
werk status                            Project summary

werk epic create|list|show|update|close|delete
werk task create|list|show|update|start|block|close|delete|ready
werk subtask create|list|show|update|start|close|delete
werk dep add|remove|list
werk decision create|list|show
werk session start|end|list|show
werk audit <task-id>

werk serve up [--port 8080]            Start web UI
werk serve down                        Stop web UI
```

All commands output JSON by default. Add `--pretty` for human-readable output.

## Agent usage

Pass `--agent` on all write commands to mark changes as agent-authored in the audit trail:

```bash
werk session start --agent
werk task ready --agent
werk task start tk-abc123 --agent
# ... do work ...
werk task close tk-abc123 --agent
werk session end --summary "Implemented login endpoint, filed 2 new tasks" --agent
```

### Agent Skill

Werk ships as an [Agent Skill](https://agentskills.io) compatible with Claude Code, Cursor, Copilot, and other agents that support the open skills standard. Browse skills at [skills.sh](https://skills.sh).

**Install the skill globally (available in all projects):**

```bash
npx skills add zackbart/werk -g
```

**Install for the current project only:**

```bash
npx skills add zackbart/werk
```

**Or reference it manually** — add to your `CLAUDE.md` (or equivalent agent config):

```markdown
## Task Tracking
Read `werk/SKILL.md` before starting any work.
```

The skill teaches agents the full werk workflow: session lifecycle, issue classification, titling conventions, priority levels, dependency management, and the complete CLI reference.

## Web UI

```bash
werk serve up --port 8080
```

Opens a read-only web UI with:

- **Board view** — kanban columns (open / in progress / blocked / done), cards grouped by epic
- **Graph view** — force-directed dependency graph, nodes colored by status
- **Decisions panel** — chronological log of architectural decisions
- **Audit drawer** — click any card to see its full change history

Stop it with `werk serve down`.

## Decisions

Werk has a first-class concept for recording non-obvious technical choices:

```bash
werk decision create "Use pure-Go SQLite driver instead of CGO" \
  --rationale "No CGO compilation complexity. modernc.org/sqlite has full feature parity."
```

Decisions are append-only. They can't be closed or deleted. They exist so future agents and engineers don't re-litigate settled choices.

## Audit trail

Every write through the CLI creates an audit entry recording what changed, when, and whether it was an agent or human:

```bash
werk audit tk-abc123
```

```json
[
  {"field": "status", "old_value": "open", "new_value": "in_progress", "changed_by": "agent", "changed_at": "2026-03-04T10:00:00Z"},
  {"field": "status", "old_value": "in_progress", "new_value": "done", "changed_by": "agent", "changed_at": "2026-03-04T11:30:00Z"}
]
```

## Sessions

Sessions track what a work period accomplished:

```bash
werk session start --agent
# ... work ...
werk session end --summary "Built auth middleware, filed 3 subtasks" --agent
```

Every CLI write during an active session automatically records which tasks were touched. The summary gives the next agent a fast orientation without reading individual audit entries.

## Design

- **Local-first** — everything lives in `.werk/tasks.db`. No network, no accounts, no sync.
- **CLI is the only write interface** — the web UI is permanently read-only. No split-brain risk.
- **Audit everything** — every state change is logged. History is never destroyed.
- **JSON by default** — agent-friendly. `--pretty` for humans.
- **Single binary** — CLI and web server in one executable. Pure Go, no CGO.
- **Fail loudly** — non-zero exit codes and `{"error": "message"}` on all failures.

## Project structure

```
cmd/werk/            CLI entrypoint
internal/
├── db/              SQLite layer, migrations, ID generation, audit
├── models/          Types: Task, Decision, Session, AuditEntry
├── commands/        One file per noun: epic.go, task.go, dep.go...
└── server/          Web UI HTTP server
web/                 Embedded single-file HTML/JS/CSS web UI
werk/                Agent Skill (SKILL.md + references/)
```

## License

MIT

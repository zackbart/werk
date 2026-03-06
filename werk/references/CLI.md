# CLI Reference

All commands output JSON by default. Add `--pretty` for human-readable output.

## DB discovery

By default, `werk` walks up the directory tree from the current directory to find `.werk/tasks.db` — just like `git` finds `.git/`. This means you can `werk init` at a monorepo root and use `werk` from any subdirectory.

Override with `--db <path>`, `--root <path>`, `--ws <name>`, or the `WERK_DB` / `WERK_ROOT` environment variables.

Resolution order:
1. `--db <path>` flag
2. `--ws <name>` — resolve from workspace registry
3. `--root <path>` — `<path>/.werk/tasks.db`
4. `WERK_DB` env
5. `WERK_ROOT` env
6. Walk-up discovery (default)

## Global flags

```
--db <path>     Override DB path (default: auto-discovered .werk/tasks.db)
--root <path>   Project root directory (uses <path>/.werk/tasks.db)
--ws <name>     Named workspace (from workspace registry)
--pretty        Human-readable output instead of JSON (or set WERK_PRETTY=1)
--agent         Set changed_by to "agent" on all writes (default: human)
--help          Show help
--version       Show version
```

## Init & status

```
werk init                                          Create .werk/tasks.db
werk status                                        Project summary (includes active session)
werk serve up [--port 8080]                        Start web UI in background
werk serve down                                    Stop web UI
```

## Epics

```
werk epic create "<title>" [--priority 0-4] [--critical|--high|--low] [--notes "<text>"] --agent
werk epic list [--status open|in_progress|done|all] [--filter "<query>"] [--archived]
werk epic show <id-or-ref>
werk epic update <id-or-ref> [--title ""] [--priority 0-4] [--notes ""] --agent
werk epic close <id-or-ref> [id-or-ref...] --agent
werk epic reopen <id-or-ref> [id-or-ref...] --agent
werk epic archive <id-or-ref> --agent
werk epic unarchive <id-or-ref> --agent
werk epic delete <id-or-ref> [--force] --agent            Permanently remove
```

## Tasks

```
werk task create "<title>" --epic <id-or-ref> [--priority 0-4] [--critical|--high|--low] [--notes "<text>"] --agent
werk task list [--epic <id-or-ref>] [--status open|in_progress|done|blocked|all] [--filter "<query>"] [--archived]
werk task show <id-or-ref>                                 Includes subtask progress
werk task update <id-or-ref> [--title ""] [--priority 0-4] [--notes ""] --agent
werk task start <id-or-ref> [id-or-ref...] --agent         Set to in_progress (records started_at)
werk task block <id-or-ref> --agent                        Set to blocked
werk task close <id-or-ref> [id-or-ref...] --agent         Set to done
werk task reopen <id-or-ref> [id-or-ref...] --agent        Reopen a closed task
werk task ready                                            List unblocked tasks
werk task find <query>                                     Search tasks by title or notes
werk task move <id-or-ref> --epic <id-or-ref> --agent      Reparent to a different epic
werk task note <id-or-ref> <text> --agent                  Append text to notes
werk task link <id-or-ref> <path-or-url> [--remove] --agent  Add/remove file or URL association
werk task archive <id-or-ref> --agent
werk task unarchive <id-or-ref> --agent
werk task delete <id-or-ref> [--force] --agent             Permanently remove
```

## Subtasks

```
werk subtask create "<title>" --task <id-or-ref> [--priority 0-4] [--critical|--high|--low] [--notes "<text>"] --agent
werk subtask list --task <id-or-ref>
werk subtask show <id-or-ref>
werk subtask update <id-or-ref> [--title ""] [--notes ""] --agent
werk subtask start <id-or-ref> [id-or-ref...] --agent
werk subtask close <id-or-ref> [id-or-ref...] --agent
werk subtask reopen <id-or-ref> [id-or-ref...] --agent
werk subtask move <id-or-ref> --task <id-or-ref> --agent   Reparent to a different task
werk subtask delete <id-or-ref> [--force] --agent          Permanently remove
```

## Dependencies

```
werk dep add <upstream-id-or-ref> <downstream-id-or-ref> --agent
werk dep remove <upstream-id-or-ref> <downstream-id-or-ref> --agent
werk dep list <id-or-ref>                          Blockers + what this blocks
```

## Decisions

```
werk decision create "<summary>" [--rationale "<text>"] --agent
werk decision list
werk decision show <id>
```

## Sessions

```
werk session start [--with-context] --agent        Start a new session
werk session end [--summary "<text>"] --agent      End session (auto-summary if omitted)
werk session list
werk session show <id>
werk session recover
```

`--with-context` bundles the session object with log, ready tasks, and in-progress tasks in one response.

Auto-summary: if `--summary` is not provided on `session end`, a summary is generated from audit entries during the session.

## Audit & handoff

```
werk audit <task-id-or-ref>                        Full change history
werk handoff <id-or-ref> --compact                 Compact handoff packet
```

## Log

```
werk log [-n <limit>] [--verbose] [--task <id-or-ref>]   Recent project activity
```

Shows a reverse-chronological feed of high-signal events: task status changes (created, started, closed, blocked), decisions, and session start/end with summaries.

- Default limit is 20 entries
- `--verbose` / `-v` includes notes, decision rationale, and session touched-tasks
- `--task <id-or-ref>` filters log to a specific task
- Use `--pretty` for human-readable one-line-per-event format

## Next

```
werk next --agent                                  Pick highest-priority ready task and start it
```

Selects the highest-priority unblocked task (by priority ASC, then created_at ASC) and sets it to `in_progress`.

## Batch

```
werk batch                                         Execute commands from stdin, one per line
```

Reads werk subcommands from stdin (one per line, `#` comments and blank lines ignored) and executes them sequentially.

## Diff

```
werk diff [--since <session-id>]                   Show changes since last session
```

Without `--since`: uses the most recent ended session. Shows audit entries grouped by task.

## Export & import

```
werk export                                        Export all data as JSON
werk import <file>                                 Import data from JSON file
```

## Workspaces

```
werk workspace add <name> [path]                   Register a workspace (default: cwd)
werk workspace list                                List registered workspaces
werk workspace remove <name>                       Unregister a workspace
```

Workspaces let you target `.werk/` databases beyond the nearest one. Use `--ws <name>` on any command to target a named workspace.

## Output format

Every command returns JSON. Arrays for list commands, objects for show/create/update.

Example task object:
```json
{
  "id": "tk-a1b2c3",
  "ref": "1.1",
  "parent_id": "ep-d4e5f6",
  "parent_ref": "1",
  "type": "task",
  "title": "Implement hash ID generation",
  "status": "open",
  "priority": 1,
  "blockers": [],
  "notes": "Use sha256(title+timestamp)[:6], retry on collision",
  "links": [],
  "started_at": null,
  "subtask_progress": {"open": 1, "done": 2, "total": 3},
  "created_at": "2026-03-04T10:00:00Z",
  "updated_at": null,
  "closed_at": null
}
```

Errors always return `{"code":"ERR_*","message":"<message>"}` with a non-zero exit code.

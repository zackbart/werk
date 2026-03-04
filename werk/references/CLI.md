# CLI Reference

All commands output JSON by default. Add `--pretty` for human-readable output.

## DB discovery

By default, `werk` walks up the directory tree from the current directory to find `.werk/tasks.db` — just like `git` finds `.git/`. This means you can `werk init` at a monorepo root and use `werk` from any subdirectory.

Override with `--db <path>` or the `WERK_DB` environment variable.

## Global flags

```
--db <path>     Override DB path (default: auto-discovered .werk/tasks.db)
--pretty        Human-readable output instead of JSON
--agent         Set changed_by to "agent" on all writes (default: human)
--help          Show help
--version       Show version
```

## Init & status

```
werk init                                          Create .werk/tasks.db
werk status                                        Project summary counts
werk serve up [--port 8080]                        Start web UI in background
werk serve down                                    Stop web UI
```

## Epics

```
werk epic create "<title>" [--priority 0-4] [--notes "<text>"] --agent
werk epic list [--status open|in_progress|done|all]
werk epic show <id>
werk epic update <id> [--title ""] [--priority 0-4] [--notes ""] --agent
werk epic close <id> --agent
werk epic delete <id> [--force] --agent            Permanently remove
```

## Tasks

```
werk task create "<title>" --epic <id> [--priority 0-4] [--notes "<text>"] --agent
werk task list [--epic <id>] [--status open|in_progress|done|blocked|all]
werk task show <id>
werk task update <id> [--title ""] [--priority 0-4] [--notes ""] --agent
werk task start <id> --agent                       Set to in_progress
werk task block <id> --agent                       Set to blocked
werk task close <id> --agent                       Set to done
werk task delete <id> [--force] --agent            Permanently remove
werk task ready                                    List unblocked tasks
```

## Subtasks

```
werk subtask create "<title>" --task <id> [--notes "<text>"] --agent
werk subtask list --task <id>
werk subtask show <id>
werk subtask update <id> [--title ""] [--notes ""] --agent
werk subtask start <id> --agent
werk subtask close <id> --agent
werk subtask delete <id> [--force] --agent         Permanently remove
```

## Dependencies

```
werk dep add <upstream> <downstream> --agent       Upstream blocks downstream
werk dep remove <upstream> <downstream> --agent
werk dep list <id>                                 Blockers + what this blocks
```

## Decisions

```
werk decision create "<summary>" [--rationale "<text>"] --agent
werk decision list
werk decision show <id>
```

## Sessions

```
werk session start --agent
werk session end [--summary "<text>"] --agent
werk session list
werk session show <id>
```

## Audit

```
werk audit <task-id>                               Full change history
```

## Log

```
werk log [-n <limit>] [--verbose]                  Recent project activity
```

Shows a reverse-chronological feed of high-signal events: task status changes (created, started, closed, blocked), decisions, and session start/end with summaries.

- Default limit is 20 entries
- `--verbose` / `-v` includes notes, decision rationale, and session touched-tasks
- Use `--pretty` for human-readable one-line-per-event format

## Output format

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

Errors always return `{"error": "<message>"}` with a non-zero exit code.

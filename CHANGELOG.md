# Changelog

## v0.2.3 — 2026-03-06

### Breaking changes

- **Renamed `workspace` → `werkspace`** — CLI subcommand is now `werk werkspace add|list|remove`. The `--ws` flag is unchanged.
- **Werkspace config moved** — Now always stored at `~/.config/werk/werkspaces.json` (was `~/Library/Application Support/werk/workspaces.json` on macOS). Existing `workspaces.json` files are not migrated automatically.

### Improvements

- **`werk init --name <name>`** — Override the auto-derived werkspace name. Without `--name`, the directory basename is used.
- **`werk init` always updates werkspace registration** — Re-running `init` now overwrites the werkspace entry for the current name, keeping the registry in sync.

---

## v0.2.2 — 2026-03-06

### Improvements

- **`werk init` now upgrades existing databases** — Running `init` on an already-initialized project runs migrations, fixes `.gitignore` patterns (broken in <= 0.1.1), and registers the werkspace. Returns `{"status": "upgraded"}` instead of erroring out.
- **Worktree-aware DB discovery** — When `.werk/tasks.db` isn't found by walking up, `werk` uses `git rev-parse --git-common-dir` to locate the main worktree's database. Works transparently in git worktrees that live outside the repo tree.
- **Snapshot restore on init** — If `.werk/snapshot.json` exists when running `werk init` on a fresh project, the snapshot is automatically imported. Commit snapshots via `werk export > .werk/snapshot.json` to preserve task history across fresh clones.

---

## v0.2.1 — 2026-03-06

### New commands

- **`werk next`** — Pick the highest-priority ready task and start it in one command
- **`werk batch`** — Execute commands from stdin, one per line (for scripting)
- **`werk diff`** — Show changes since the last session (or `--since <session-id>`)
- **`werk export` / `werk import`** — Full JSON data export and import
- **`werk werkspace add|list|remove`** — Named werkspace registry for multi-project setups

### New subcommands

- **`werk task note`** — Append text to task notes incrementally
- **`werk task link`** — Associate files and URLs with tasks (`--remove` to unlink)
- **`werk task find`** — Search tasks by title or notes substring
- **`werk task move`** / **`werk subtask move`** — Reparent items between epics/tasks
- **`werk task reopen`** / **`werk epic reopen`** / **`werk subtask reopen`** — Reopen closed items
- **`werk task archive`** / **`werk epic archive`** — Hide completed items from default views
- **`werk task unarchive`** / **`werk epic unarchive`** — Restore archived items

### Improvements

- **Priority shorthands** — `--critical` (P0), `--high` (P1), `--low` (P3) on all create commands
- **Bulk operations** — `start`, `close`, and `reopen` accept multiple refs: `werk task close 1.1 1.2 1.3`
- **`--with-context` on session start** — Returns session + log + ready + in-progress tasks in one call
- **Auto-summary on session end** — If `--summary` is omitted, generates one from audit entries
- **`--task` filter on log** — `werk log --task 1.1` shows activity for a specific task only
- **`started_at` tracking** — Records when a task first moves to `in_progress`
- **Subtask progress** — `werk task show` includes `subtask_progress: {open, done, total}`
- **Active session in status** — `werk status` shows the active session ID when one exists
- **`WERK_PRETTY=1` env var** — Set pretty output as default without passing `--pretty` every time
- **`--root` flag and `WERK_ROOT` env** — Target a project root directory directly
- **`--ws` flag** — Target a named werkspace from the registry
- **`--filter` on list commands** — Filter epics/tasks by title/notes substring
- **`--archived` on list commands** — Include archived items in list output

### Web UI

- **Replaced SVG graph with tree view** — Collapsible epic/task/subtask hierarchy with status badges, priority indicators, dependency badges, and progress counters
- **Show/hide done items** and **show/hide dependencies** toggles

### Internal

- Generic `ensureColumn()` migration helper for safe schema evolution
- Fixed SQLite timestamp format mismatch (`CURRENT_TIMESTAMP` vs RFC3339)

---

## v0.2.0 — 2026-03-05

- Dotted refs (`1`, `1.1`, `1.1.2`) for stable human-readable identifiers
- Hash-based IDs (`ep-`, `tk-`, `st-`, `dc-`, `sn-`) as primary keys
- Handoff packets with `--compact` flag
- Session recovery (`werk session recover`)
- Audit trail on all write operations

## v0.1.1 — 2026-03-04

- Walk-up DB discovery (like git)
- Web UI with board view and audit drawer
- `werk serve up/down` background server management
- Log command with `--verbose` and `--pretty`

## v0.1.0 — 2026-03-03

- Initial release
- Epics, tasks, subtasks, decisions, dependencies
- Sessions with lock file
- SQLite storage with pure-Go driver

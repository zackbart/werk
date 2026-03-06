# Migrating from Beads to Werk

This guide helps agents migrate existing project data from Beads to Werk.

## Prerequisites

- Beads CLI (`bd`) installed and project initialized (`.beads/` exists)
- Werk CLI (`werk`) installed and project initialized (`werk init`)

## Step 1: Export from Beads

Export all current issues from beads as JSONL:

```bash
bd export
```

This writes to `.beads/beads.jsonl`. Each line is a JSON object representing one issue.

Alternatively, query specific data directly:

```bash
bd list --status open --format json
bd list --status in_progress --format json
bd list --status blocked --format json
bd list --type epic --format json
```

## Step 2: Understand the mapping

| Beads concept | Werk equivalent | Notes |
|---|---|---|
| Issue type `epic` | `werk epic create` | Direct mapping |
| Issue type `task`, `feature`, `bug`, `chore` | `werk task create` | All become tasks in werk |
| Subtask (hierarchical ID like `bd-a1b2.1.1`) | `werk subtask create` | Flattened to parent task |
| `blocks` dependency | `werk dep add` | Direct mapping |
| `parent-child` dependency | Hierarchy (epic → task → subtask) | Implicit in werk |
| `related`, `duplicates`, `supersedes` | Dropped | Werk only supports "blocks" |
| Messages / threads | Dropped | Use decision or task notes instead |
| Comments | Task/epic notes | Merge into `--notes` field |
| Compacted summaries | `werk decision create` | Architectural context preserved |
| Labels | Dropped | Use priority and notes instead |
| Molecules / Wisps | Dropped | Not applicable |
| Sessions | `werk session` | Start fresh in werk |

### Status mapping

| Beads status | Werk status |
|---|---|
| `open` | `open` |
| `in_progress` | `in_progress` |
| `blocked` | `blocked` |
| `closed` | `done` |

### Priority mapping

Direct 1:1 — both use 0-4 scale (0=critical, 4=backlog).

## Step 3: Dry run — inventory before migrating

Before creating anything in werk, export a full inventory from beads so you know exactly what to expect:

```bash
# Count what you're migrating
echo "Epics:    $(bd list --type epic --format json | wc -l)"
echo "Tasks:    $(bd list --type task --format json | wc -l)"
echo "Subtasks: $(bd list --type subtask --format json 2>/dev/null | wc -l)"

# Save full export for reference
bd export  # writes .beads/beads.jsonl
```

Keep these counts — you'll compare them against werk after migration.

## Step 4: Migration script

Run this workflow to migrate. Adjust the `bd list` commands based on your beads setup.

**Important:** Werk has no delete command (by design). If you create duplicates during migration, cleanup requires closing the extras or manual SQL. Run each step once and verify before proceeding to the next.

### 4a. Migrate epics

```bash
# List beads epics and create in werk
# Check for existing epics first to avoid duplicates on re-runs
bd list --type epic --format json | while read -r line; do
  TITLE=$(echo "$line" | jq -r '.Title')
  PRIORITY=$(echo "$line" | jq -r '.Priority // 2')
  DESC=$(echo "$line" | jq -r '.Description // empty')
  NOTES=$(echo "$line" | jq -r '.Notes // empty')
  COMBINED="${DESC:+$DESC}${DESC:+$'\n'}${NOTES:+$NOTES}"

  # Skip if an epic with this exact title already exists in werk
  EXISTING=$(werk epic list | jq -r --arg t "$TITLE" '.[] | select(.title == $t) | .id')
  if [ -n "$EXISTING" ]; then
    echo "SKIP epic (already exists as $EXISTING): $TITLE"
    continue
  fi

  if [ -n "$COMBINED" ]; then
    werk epic create "$TITLE" --priority "$PRIORITY" --notes "$COMBINED" --agent
  else
    werk epic create "$TITLE" --priority "$PRIORITY" --agent
  fi
done
```

### 4b. Migrate tasks

For each epic, migrate its child tasks. You'll need to map beads epic IDs to werk epic IDs.

```bash
# Get the werk epic ID for a given title
WERK_EPIC=$(werk epic list | jq -r '.[] | select(.title == "YOUR EPIC TITLE") | .id')

# List tasks under that epic in beads and create in werk
bd list --type task --format json | while read -r line; do
  TITLE=$(echo "$line" | jq -r '.Title')
  PRIORITY=$(echo "$line" | jq -r '.Priority // 2')
  STATUS=$(echo "$line" | jq -r '.Status')
  NOTES=$(echo "$line" | jq -r '.Notes // empty')

  # Skip if a task with this exact title already exists under this epic
  EXISTING=$(werk task list --epic "$WERK_EPIC" | jq -r --arg t "$TITLE" '.[] | select(.title == $t) | .id')
  if [ -n "$EXISTING" ]; then
    echo "SKIP task (already exists as $EXISTING): $TITLE"
    continue
  fi

  TASK_JSON=$(werk task create "$TITLE" --epic "$WERK_EPIC" --priority "$PRIORITY" ${NOTES:+--notes "$NOTES"} --agent)
  TASK_ID=$(echo "$TASK_JSON" | jq -r '.id')

  # Set status if not open
  case "$STATUS" in
    in_progress) werk task start "$TASK_ID" --agent ;;
    blocked)     werk task block "$TASK_ID" --agent ;;
    closed)      werk task close "$TASK_ID" --agent ;;
  esac
done
```

### 4c. Migrate dependencies

```bash
# For each blocks dependency in beads, add it in werk
# You'll need to maintain a mapping of beads IDs → werk IDs
# Example:
werk dep add tk-upstream tk-downstream --agent
```

### 4d. Migrate key decisions

Review beads compacted summaries and any important comments for architectural decisions worth preserving:

```bash
werk decision create "Migrated from beads: <summary>" \
  --rationale "<original rationale or context>" --agent
```

## Step 5: Verify and deduplicate

### Verify counts

```bash
werk status --pretty
werk epic list --pretty
werk task ready --agent
```

Compare counts against beads:

```bash
bd stats
```

### Detect duplicates

If the migration was run more than once (or an agent re-ran parts of it), you may have duplicate epics or tasks. Check for them:

```bash
# Find duplicate epic titles
werk epic list | jq -r '[.[] | .title] | group_by(.) | map(select(length > 1)) | .[] | {title: .[0], count: length}'

# Find duplicate task titles within each epic
werk task list --status all | jq -r 'group_by(.epic_id) | .[] | [.[] | .title] | group_by(.) | map(select(length > 1)) | .[] | {title: .[0], count: length}'
```

### Clean up duplicates

1. **Identify which copy to keep** — prefer the one with dependencies attached or correct status.
2. **Delete the extras** — use `werk delete` to permanently remove duplicates:
   ```bash
   # Delete an open duplicate
   werk task delete <duplicate-id> --agent

   # Delete a duplicate that was already started/closed
   werk task delete <duplicate-id> --force --agent

   # For epics, delete child tasks first
   werk task delete <child-id> --force --agent
   werk epic delete <duplicate-id> --force --agent
   ```

**Tip for agents:** Before creating any item during migration, always check if it already exists by title. The scripts in Step 4 include these checks — if you're migrating manually, do the same:
```bash
# Before creating, check:
werk epic list | jq -r --arg t "My Epic Title" '.[] | select(.title == $t) | .id'
# If non-empty, skip creation
```

## Step 6: Record the migration

```bash
werk decision create "Migrated from Beads to Werk" \
  --rationale "Beads features like molecules, wisps, multi-agent sync, and named dependency types were not needed. Werk provides simpler local-first tracking with lower infrastructure overhead. Migration was lossy by design: related/duplicates/supersedes deps, messages, labels, and molecules were dropped." \
  --agent
```

## What's lost (by design)

- **Named dependency types** beyond "blocks" (related, duplicates, supersedes)
- **Messages and threads** — use task notes or decisions instead
- **Labels** — use priority levels and notes
- **Molecules and wisps** — not applicable to werk's model
- **Dolt version history** — werk has its own audit trail going forward
- **Multi-agent sync** — werk is single-agent, local-first
- **Comments** — merge important ones into task notes

## What's preserved

- All epics, tasks, and subtasks with their titles, notes, and priorities
- Status of each item (open, in_progress, blocked, done)
- Blocking dependencies
- Key architectural decisions
- Audit trail starts fresh in werk from the migration point forward

## Gotchas

### `.werk/.gitignore` has broken patterns (versions ≤ 0.1.1)

Early versions of `werk init` generated `.werk/*.db-wal` inside `.werk/.gitignore`. Since gitignore patterns are relative to the file's location, this never matched anything. After running `werk init`, verify the patterns are:

```
*.db-wal
*.db-shm
session.lock
serve.pid
```

If you see `.werk/`-prefixed patterns, fix them manually. Versions after 0.1.1 generate the correct patterns.

### Root `.gitignore` should ignore `.werk/`

The `.werk/` directory — including `tasks.db` — should be gitignored. The binary database is not committed; use the [snapshot workflow](../SKILL.md#snapshot-workflow) (`werk export > .werk/snapshot.json`) to commit a portable JSON representation instead.

`werk init` adds `.werk/` to your root `.gitignore` automatically. If it's missing:

```bash
echo '.werk/' >> .gitignore
```

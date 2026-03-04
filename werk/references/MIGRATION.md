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

## Step 3: Migration script

Run this workflow to migrate. Adjust the `bd list` commands based on your beads setup.

### 3a. Migrate epics

```bash
# List beads epics and create in werk
bd list --type epic --format json | while read -r line; do
  TITLE=$(echo "$line" | jq -r '.Title')
  PRIORITY=$(echo "$line" | jq -r '.Priority // 2')
  DESC=$(echo "$line" | jq -r '.Description // empty')
  NOTES=$(echo "$line" | jq -r '.Notes // empty')
  COMBINED="${DESC:+$DESC}${DESC:+$'\n'}${NOTES:+$NOTES}"

  if [ -n "$COMBINED" ]; then
    werk epic create "$TITLE" --priority "$PRIORITY" --notes "$COMBINED" --agent
  else
    werk epic create "$TITLE" --priority "$PRIORITY" --agent
  fi
done
```

### 3b. Migrate tasks

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

### 3c. Migrate dependencies

```bash
# For each blocks dependency in beads, add it in werk
# You'll need to maintain a mapping of beads IDs → werk IDs
# Example:
werk dep add tk-upstream tk-downstream --agent
```

### 3d. Migrate key decisions

Review beads compacted summaries and any important comments for architectural decisions worth preserving:

```bash
werk decision create "Migrated from beads: <summary>" \
  --rationale "<original rationale or context>" --agent
```

## Step 4: Verify

```bash
werk status --pretty
werk epic list --pretty
werk task ready --agent
```

Compare counts against beads:

```bash
bd stats
```

## Step 5: Record the migration

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

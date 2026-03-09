package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"werk/internal/db"
	"werk/internal/models"
)

func newTaskCmd() *cobra.Command {
	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
	}

	// create
	createCmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epicIDOrRef, _ := cmd.Flags().GetString("epic")
			if epicIDOrRef == "" {
				outputError("--epic is required")
				return nil
			}
			epicID := mustResolveID(epicIDOrRef)
			priority, _ := cmd.Flags().GetInt("priority")
			priority = applyPriorityShorthands(cmd, priority)
			notes, _ := cmd.Flags().GetString("notes")
			var notesPtr *string
			if cmd.Flags().Changed("notes") {
				notesPtr = &notes
			}

			t, err := database.CreateTask("task", args[0], &epicID, priority, notesPtr, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}
	createCmd.Flags().String("epic", "", "parent epic <id> (required)")
	createCmd.Flags().Int("priority", 2, "priority (0-4)")
	createCmd.Flags().String("notes", "", "notes")
	createCmd.Flags().Bool("critical", false, "set priority to 0 (critical)")
	createCmd.Flags().Bool("high", false, "set priority to 1 (high)")
	createCmd.Flags().Bool("low", false, "set priority to 3 (low)")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			epicIDOrRef, _ := cmd.Flags().GetString("epic")
			var parentPtr *string
			if epicIDOrRef != "" {
				epicID := mustResolveID(epicIDOrRef)
				parentPtr = &epicID
			}
			archived, _ := cmd.Flags().GetBool("archived")
			var opts []db.ListOption
			if archived {
				opts = append(opts, db.WithArchived())
			}
			tasks, err := database.ListTasks("task", parentPtr, status, opts...)
			if err != nil {
				outputError(err.Error())
				return nil
			}
			filter, _ := cmd.Flags().GetString("filter")
			if filter != "" {
				var filtered []models.Task
				lowerFilter := strings.ToLower(filter)
				for _, t := range tasks {
					if strings.Contains(strings.ToLower(t.Title), lowerFilter) || (t.Notes != nil && strings.Contains(strings.ToLower(*t.Notes), lowerFilter)) {
						filtered = append(filtered, t)
					}
				}
				tasks = filtered
			}
			outputJSON(toTaskJSONList(tasks))
			return nil
		},
	}
	listCmd.Flags().String("status", "", "filter by status")
	listCmd.Flags().String("epic", "", "filter by epic <id>")
	listCmd.Flags().String("filter", "", "filter by title/notes substring")
	listCmd.Flags().Bool("archived", false, "include archived items")

	// show
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			t, err := database.GetTask(id)
			if err != nil {
				outputError(err.Error())
				return nil
			}
			j := t.ToJSON()
			if t.Type == "task" || t.Type == "epic" {
				progress, _ := database.GetSubtaskProgress(id)
				if progress != nil {
					j.SubtaskProgress = progress
				}
			}
			outputJSON(j)
			return nil
		},
	}

	// update
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var titlePtr *string
			var notesPtr *string
			var priPtr *int

			if cmd.Flags().Changed("title") {
				v, _ := cmd.Flags().GetString("title")
				titlePtr = &v
			}
			if cmd.Flags().Changed("notes") {
				v, _ := cmd.Flags().GetString("notes")
				notesPtr = &v
			}
			if cmd.Flags().Changed("priority") {
				v, _ := cmd.Flags().GetInt("priority")
				priPtr = &v
			}

			id := mustResolveID(args[0])
			t, err := database.UpdateTask(id, titlePtr, notesPtr, priPtr, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}
	updateCmd.Flags().String("title", "", "new title")
	updateCmd.Flags().Int("priority", 2, "new priority")
	updateCmd.Flags().String("notes", "", "new notes")

	// start
	startCmd := &cobra.Command{
		Use:   "start <id> [id...]",
		Short: "Start a task (set to in_progress)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkStatus(args, "in_progress")
		},
	}

	// block
	blockCmd := &cobra.Command{
		Use:   "block <id>",
		Short: "Mark a task as blocked",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			t, err := database.SetTaskStatus(id, "blocked", changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}

	// close
	closeCmd := &cobra.Command{
		Use:   "close <id> [id...]",
		Short: "Close a task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkStatus(args, "done")
		},
	}

	// reopen
	reopenCmd := &cobra.Command{
		Use:   "reopen <id> [id...]",
		Short: "Reopen a closed task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkStatus(args, "open")
		},
	}

	// ready
	readyCmd := &cobra.Command{
		Use:   "ready",
		Short: "List tasks with no open blockers",
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := database.ReadyTasks()
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(toTaskJSONList(tasks))
			return nil
		},
	}

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			id := mustResolveID(args[0])
			ref, _ := resolveRef(id)
			err := database.DeleteTask(id, force, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(map[string]string{"status": "deleted", "id": id, "ref": ref})
			return nil
		},
	}
	deleteCmd.Flags().Bool("force", false, "delete even if not in open status")

	// find
	findCmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Search tasks by title or notes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := database.SearchTasks(args[0])
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(toTaskJSONList(tasks))
			return nil
		},
	}

	// move
	moveCmd := &cobra.Command{
		Use:   "move <id> --epic <id>",
		Short: "Move a task to a different epic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epicIDOrRef, _ := cmd.Flags().GetString("epic")
			if epicIDOrRef == "" {
				outputError("--epic is required")
				return nil
			}
			id := mustResolveID(args[0])
			newParentID := mustResolveID(epicIDOrRef)
			t, err := database.ReparentTask(id, newParentID, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}
	moveCmd.Flags().String("epic", "", "target epic <id> (required)")

	// archive
	archiveCmd := &cobra.Command{
		Use:   "archive <id>",
		Short: "Archive a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			t, err := database.ArchiveTask(id, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}

	// unarchive
	unarchiveCmd := &cobra.Command{
		Use:   "unarchive <id>",
		Short: "Unarchive a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			t, err := database.UnarchiveTask(id, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}

	// note
	noteCmd := &cobra.Command{
		Use:   "note <id> <text>",
		Short: "Append text to task notes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			t, err := database.AppendNote(id, args[1], changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}

	// link
	linkCmd := &cobra.Command{
		Use:   "link <id> <path-or-url>",
		Short: "Add a file or URL association",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			remove, _ := cmd.Flags().GetBool("remove")
			id := mustResolveID(args[0])
			var t *models.Task
			var err error
			if remove {
				t, err = database.RemoveLink(id, args[1], changedBy())
			} else {
				t, err = database.AddLink(id, args[1], changedBy())
			}
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}
	linkCmd.Flags().Bool("remove", false, "remove the link instead of adding")

	taskCmd.AddCommand(createCmd, listCmd, showCmd, updateCmd, startCmd, blockCmd, closeCmd, reopenCmd, readyCmd, findCmd, moveCmd, archiveCmd, unarchiveCmd, deleteCmd, noteCmd, linkCmd)
	return taskCmd
}

func applyPriorityShorthands(cmd *cobra.Command, priority int) int {
	if v, _ := cmd.Flags().GetBool("critical"); v {
		return 0
	}
	if v, _ := cmd.Flags().GetBool("high"); v {
		return 1
	}
	if v, _ := cmd.Flags().GetBool("low"); v {
		return 3
	}
	return priority
}

func bulkStatus(args []string, status string) error {
	if len(args) == 1 {
		id := mustResolveID(args[0])
		t, err := database.SetTaskStatus(id, status, changedBy())
		if err != nil {
			outputError(err.Error())
			return nil
		}
		outputJSON(t.ToJSON())
		return nil
	}

	var completed []map[string]string
	for _, arg := range args {
		id := mustResolveID(arg)
		t, err := database.SetTaskStatus(id, status, changedBy())
		if err != nil {
			outputJSON(map[string]interface{}{
				"completed": completed,
				"failed":    map[string]string{"ref": arg, "error": err.Error()},
			})
			return nil
		}
		completed = append(completed, map[string]string{"id": t.ID, "ref": t.Ref})
	}
	outputJSON(map[string]interface{}{"completed": completed})
	return nil
}

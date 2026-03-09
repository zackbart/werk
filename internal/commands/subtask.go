package commands

import (
	"github.com/spf13/cobra"
)

func newSubtaskCmd() *cobra.Command {
	subtaskCmd := &cobra.Command{
		Use:   "subtask",
		Short: "Manage subtasks",
	}

	// create
	createCmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new subtask",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskIDOrRef, _ := cmd.Flags().GetString("task")
			if taskIDOrRef == "" {
				outputError("--task is required")
				return nil
			}
			taskID := mustResolveID(taskIDOrRef)
			priority, _ := cmd.Flags().GetInt("priority")
			priority = applyPriorityShorthands(cmd, priority)
			notes, _ := cmd.Flags().GetString("notes")
			var notesPtr *string
			if cmd.Flags().Changed("notes") {
				notesPtr = &notes
			}

			t, err := database.CreateTask("subtask", args[0], &taskID, priority, notesPtr, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}
	createCmd.Flags().String("task", "", "parent task <id> (required)")
	createCmd.Flags().Int("priority", 2, "priority (0-4)")
	createCmd.Flags().String("notes", "", "notes")
	createCmd.Flags().Bool("critical", false, "set priority to 0 (critical)")
	createCmd.Flags().Bool("high", false, "set priority to 1 (high)")
	createCmd.Flags().Bool("low", false, "set priority to 3 (low)")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List subtasks for a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			taskIDOrRef, _ := cmd.Flags().GetString("task")
			if taskIDOrRef == "" {
				outputError("--task is required")
				return nil
			}
			taskID := mustResolveID(taskIDOrRef)
			tasks, err := database.ListTasks("subtask", &taskID, "")
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(toTaskJSONList(tasks))
			return nil
		},
	}
	listCmd.Flags().String("task", "", "parent task <id> (required)")

	// show
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show subtask details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			t, err := database.GetTask(id)
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}

	// update
	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a subtask",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var titlePtr *string
			var notesPtr *string

			if cmd.Flags().Changed("title") {
				v, _ := cmd.Flags().GetString("title")
				titlePtr = &v
			}
			if cmd.Flags().Changed("notes") {
				v, _ := cmd.Flags().GetString("notes")
				notesPtr = &v
			}

			id := mustResolveID(args[0])
			t, err := database.UpdateTask(id, titlePtr, notesPtr, nil, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}
	updateCmd.Flags().String("title", "", "new title")
	updateCmd.Flags().String("notes", "", "new notes")

	// start
	startCmd := &cobra.Command{
		Use:   "start <id> [id...]",
		Short: "Start a subtask",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkStatus(args, "in_progress")
		},
	}

	// close
	closeCmd := &cobra.Command{
		Use:   "close <id> [id...]",
		Short: "Close a subtask",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkStatus(args, "done")
		},
	}

	// reopen
	reopenCmd := &cobra.Command{
		Use:   "reopen <id> [id...]",
		Short: "Reopen a closed subtask",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkStatus(args, "open")
		},
	}

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a subtask",
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

	// move
	moveCmd := &cobra.Command{
		Use:   "move <id> --task <id>",
		Short: "Move a subtask to a different task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskIDOrRef, _ := cmd.Flags().GetString("task")
			if taskIDOrRef == "" {
				outputError("--task is required")
				return nil
			}
			id := mustResolveID(args[0])
			newParentID := mustResolveID(taskIDOrRef)
			t, err := database.ReparentTask(id, newParentID, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}
	moveCmd.Flags().String("task", "", "target task <id> (required)")

	subtaskCmd.AddCommand(createCmd, listCmd, showCmd, updateCmd, startCmd, closeCmd, reopenCmd, moveCmd, deleteCmd)
	return subtaskCmd
}

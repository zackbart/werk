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
	createCmd.Flags().String("task", "", "parent task <id-or-ref> (required)")
	createCmd.Flags().Int("priority", 2, "priority (0-4)")
	createCmd.Flags().String("notes", "", "notes")

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
			out := toTaskJSONList(tasks)
			if out == nil {
				outputJSON([]interface{}{})
			} else {
				outputJSON(out)
			}
			return nil
		},
	}
	listCmd.Flags().String("task", "", "parent task <id-or-ref> (required)")

	// show
	showCmd := &cobra.Command{
		Use:   "show <id-or-ref>",
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
		Use:   "update <id-or-ref>",
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
		Use:   "start <id-or-ref>",
		Short: "Start a subtask",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			t, err := database.SetTaskStatus(id, "in_progress", changedBy())
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
		Use:   "close <id-or-ref>",
		Short: "Close a subtask",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := mustResolveID(args[0])
			t, err := database.SetTaskStatus(id, "done", changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete <id-or-ref>",
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

	subtaskCmd.AddCommand(createCmd, listCmd, showCmd, updateCmd, startCmd, closeCmd, deleteCmd)
	return subtaskCmd
}

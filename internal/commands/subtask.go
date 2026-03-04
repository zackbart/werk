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
			taskID, _ := cmd.Flags().GetString("task")
			if taskID == "" {
				outputError("--task is required")
				return nil
			}
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
	createCmd.Flags().String("task", "", "parent task ID (required)")
	createCmd.Flags().Int("priority", 2, "priority (0-4)")
	createCmd.Flags().String("notes", "", "notes")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List subtasks for a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID, _ := cmd.Flags().GetString("task")
			if taskID == "" {
				outputError("--task is required")
				return nil
			}
			tasks, err := database.ListTasks("subtask", &taskID, "")
			if err != nil {
				outputError(err.Error())
				return nil
			}
			var out []interface{}
			for _, t := range tasks {
				out = append(out, t.ToJSON())
			}
			if out == nil {
				outputJSON([]interface{}{})
			} else {
				outputJSON(out)
			}
			return nil
		},
	}
	listCmd.Flags().String("task", "", "parent task ID (required)")

	// show
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show subtask details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := database.GetTask(args[0])
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

			t, err := database.UpdateTask(args[0], titlePtr, notesPtr, nil, changedBy())
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
		Use:   "start <id>",
		Short: "Start a subtask",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := database.SetTaskStatus(args[0], "in_progress", changedBy())
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
		Use:   "close <id>",
		Short: "Close a subtask",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := database.SetTaskStatus(args[0], "done", changedBy())
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
		Use:   "delete <id>",
		Short: "Permanently delete a subtask",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			err := database.DeleteTask(args[0], force, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(map[string]string{"status": "deleted", "id": args[0]})
			return nil
		},
	}
	deleteCmd.Flags().Bool("force", false, "delete even if not in open status")

	subtaskCmd.AddCommand(createCmd, listCmd, showCmd, updateCmd, startCmd, closeCmd, deleteCmd)
	return subtaskCmd
}

package commands

import (
	"github.com/spf13/cobra"
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
			epicID, _ := cmd.Flags().GetString("epic")
			if epicID == "" {
				outputError("--epic is required")
				return nil
			}
			priority, _ := cmd.Flags().GetInt("priority")
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
	createCmd.Flags().String("epic", "", "parent epic ID (required)")
	createCmd.Flags().Int("priority", 2, "priority (0-4)")
	createCmd.Flags().String("notes", "", "notes")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			epicID, _ := cmd.Flags().GetString("epic")
			var parentPtr *string
			if epicID != "" {
				parentPtr = &epicID
			}
			tasks, err := database.ListTasks("task", parentPtr, status)
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
	listCmd.Flags().String("status", "", "filter by status")
	listCmd.Flags().String("epic", "", "filter by epic ID")

	// show
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show task details",
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

			t, err := database.UpdateTask(args[0], titlePtr, notesPtr, priPtr, changedBy())
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
		Use:   "start <id>",
		Short: "Start a task (set to in_progress)",
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

	// block
	blockCmd := &cobra.Command{
		Use:   "block <id>",
		Short: "Mark a task as blocked",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := database.SetTaskStatus(args[0], "blocked", changedBy())
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
		Short: "Close a task",
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

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete a task",
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

	taskCmd.AddCommand(createCmd, listCmd, showCmd, updateCmd, startCmd, blockCmd, closeCmd, readyCmd, deleteCmd)
	return taskCmd
}

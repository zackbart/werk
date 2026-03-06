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
				outputErrorCode("INVALID_INPUT", "--epic is required")
				return nil
			}
			epicID = resolveTaskIDOrExit(epicID)
			priority, _ := cmd.Flags().GetInt("priority")
			notes, _ := cmd.Flags().GetString("notes")
			var notesPtr *string
			if cmd.Flags().Changed("notes") {
				notesPtr = &notes
			}

			t, err := database.CreateTask("task", args[0], &epicID, priority, notesPtr, changedBy())
			if err != nil {
				outputErr(err)
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}
	createCmd.Flags().String("epic", "", "parent epic id-or-ref (required)")
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
				epicID = resolveTaskIDOrExit(epicID)
				parentPtr = &epicID
			}
			tasks, err := database.ListTasks("task", parentPtr, status)
			if err != nil {
				outputErr(err)
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
	listCmd.Flags().String("epic", "", "filter by epic id-or-ref")

	// show
	showCmd := &cobra.Command{
		Use:   "show <id-or-ref>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := database.GetTask(resolveTaskIDOrExit(args[0]))
			if err != nil {
				outputErr(err)
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}

	// update
	updateCmd := &cobra.Command{
		Use:   "update <id-or-ref>",
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

			t, err := database.UpdateTask(resolveTaskIDOrExit(args[0]), titlePtr, notesPtr, priPtr, changedBy())
			if err != nil {
				outputErr(err)
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
		Use:   "start <id-or-ref>",
		Short: "Start a task (set to in_progress)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := database.SetTaskStatus(resolveTaskIDOrExit(args[0]), "in_progress", changedBy())
			if err != nil {
				outputErr(err)
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}

	// block
	blockCmd := &cobra.Command{
		Use:   "block <id-or-ref>",
		Short: "Mark a task as blocked",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := database.SetTaskStatus(resolveTaskIDOrExit(args[0]), "blocked", changedBy())
			if err != nil {
				outputErr(err)
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}

	// close
	closeCmd := &cobra.Command{
		Use:   "close <id-or-ref>",
		Short: "Close a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := database.SetTaskStatus(resolveTaskIDOrExit(args[0]), "done", changedBy())
			if err != nil {
				outputErr(err)
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
				outputErr(err)
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
		Use:   "delete <id-or-ref>",
		Short: "Permanently delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			id := resolveTaskIDOrExit(args[0])
			existing, _ := database.GetTask(id)
			err := database.DeleteTask(id, force, changedBy())
			if err != nil {
				outputErr(err)
				return nil
			}
			ref := ""
			if existing != nil {
				ref = existing.Ref
			}
			outputJSON(map[string]string{"status": "deleted", "id": id, "ref": ref})
			return nil
		},
	}
	deleteCmd.Flags().Bool("force", false, "delete even if not in open status")

	taskCmd.AddCommand(createCmd, listCmd, showCmd, updateCmd, startCmd, blockCmd, closeCmd, readyCmd, deleteCmd)
	return taskCmd
}

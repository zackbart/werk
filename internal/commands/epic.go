package commands

import (
	"github.com/spf13/cobra"
)

func newEpicCmd() *cobra.Command {
	epicCmd := &cobra.Command{
		Use:   "epic",
		Short: "Manage epics",
	}

	// create
	createCmd := &cobra.Command{
		Use:   "create <title>",
		Short: "Create a new epic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			priority, _ := cmd.Flags().GetInt("priority")
			notes, _ := cmd.Flags().GetString("notes")
			var notesPtr *string
			if cmd.Flags().Changed("notes") {
				notesPtr = &notes
			}

			t, err := database.CreateTask("epic", args[0], nil, priority, notesPtr, changedBy())
			if err != nil {
				outputError(err.Error())
				return nil
			}
			outputJSON(t.ToJSON())
			return nil
		},
	}
	createCmd.Flags().Int("priority", 2, "priority (0-4)")
	createCmd.Flags().String("notes", "", "notes")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List epics",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			tasks, err := database.ListTasks("epic", nil, status)
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
	listCmd.Flags().String("status", "", "filter by status (open|in_progress|done|all)")

	// show
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show epic details",
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
		Short: "Update an epic",
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

	// close
	closeCmd := &cobra.Command{
		Use:   "close <id>",
		Short: "Close an epic",
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

	epicCmd.AddCommand(createCmd, listCmd, showCmd, updateCmd, closeCmd)
	return epicCmd
}

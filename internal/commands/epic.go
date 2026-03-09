package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"werk/internal/db"
	"werk/internal/models"
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
			priority = applyPriorityShorthands(cmd, priority)
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
	createCmd.Flags().Bool("critical", false, "set priority to 0 (critical)")
	createCmd.Flags().Bool("high", false, "set priority to 1 (high)")
	createCmd.Flags().Bool("low", false, "set priority to 3 (low)")

	// list
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List epics",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			archived, _ := cmd.Flags().GetBool("archived")
			var opts []db.ListOption
			if archived {
				opts = append(opts, db.WithArchived())
			}
			tasks, err := database.ListTasks("epic", nil, status, opts...)
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
			out := toTaskJSONList(tasks)
			outputJSON(out)
			return nil
		},
	}
	listCmd.Flags().String("status", "", "filter by status (open|in_progress|done|all)")
	listCmd.Flags().String("filter", "", "filter by title/notes substring")
	listCmd.Flags().Bool("archived", false, "include archived items")

	// show
	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show epic details",
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

	// close
	closeCmd := &cobra.Command{
		Use:   "close <id> [id...]",
		Short: "Close an epic",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkStatus(args, "done")
		},
	}

	// reopen
	reopenCmd := &cobra.Command{
		Use:   "reopen <id> [id...]",
		Short: "Reopen a closed epic",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return bulkStatus(args, "open")
		},
	}

	// delete
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Permanently delete an epic",
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

	// archive
	archiveCmd := &cobra.Command{
		Use:   "archive <id>",
		Short: "Archive an epic",
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
		Short: "Unarchive an epic",
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

	epicCmd.AddCommand(createCmd, listCmd, showCmd, updateCmd, closeCmd, reopenCmd, archiveCmd, unarchiveCmd, deleteCmd)
	return epicCmd
}

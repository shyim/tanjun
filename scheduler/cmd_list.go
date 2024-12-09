package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

var cmdList = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all available commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		jobs, err := db.QueryContext(cmd.Context(), "SELECT name, schedule, last_execution, next_execution, last_exit_code FROM jobs")

		if err != nil {
			return err
		}

		t := table.New().
			Headers("Name", "Schedule", "Last Execution", "Next Execution", "Last Exit Code")

		for jobs.Next() {
			var name string
			var schedule string
			var lastExecution *string
			var nextExecution string
			var lastExitCode *string

			if err := jobs.Scan(&name, &schedule, &lastExecution, &nextExecution, &lastExitCode); err != nil {
				return err
			}

			empty := "-"

			if lastExecution == nil {
				lastExecution = &empty
			}

			if lastExitCode == nil {
				lastExitCode = &empty
			}

			t.Row(name, schedule, *lastExecution, nextExecution, *lastExitCode)
		}

		fmt.Println(t.Render())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cmdList)
}

package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"math"
	"strconv"
)

var cmdHistory = &cobra.Command{
	Use:   "history",
	Short: "Use the last executions of cronjobs",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		queries, err := db.QueryContext(cmd.Context(), "SELECT id, run_at, execution_time, exit_code FROM activity WHERE name = ? ORDER BY run_at DESC LIMIT 20", args[0])

		if err != nil {
			return err
		}

		log.Infof("Runs for %s", args[0])
		fmt.Println()

		t := table.New().
			Headers("ID", "Run at", "Execution time", "Exit code")

		for queries.Next() {
			var id int
			var runAt string
			var executionTime int
			var exitCode int

			if err := queries.Scan(&id, &runAt, &executionTime, &exitCode); err != nil {
				return err
			}

			t.Row(strconv.Itoa(id), runAt, formatDuration(executionTime), strconv.Itoa(exitCode))
		}

		fmt.Println(t.Render())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cmdHistory)
}

func formatDuration(milliseconds int) string {
	if milliseconds < 0 {
		return "invalid duration"
	}

	seconds := float64(milliseconds) / 1000
	minutes := seconds / 60

	if seconds < 60 {
		// Format with up to 2 decimal places for seconds
		return fmt.Sprintf("%.2fs", math.Floor(seconds*100)/100)
	}

	// For minutes, show both minutes and remaining seconds
	remainingSeconds := math.Mod(seconds, 60)
	return fmt.Sprintf("%.0fm %.2fs", math.Floor(minutes), math.Floor(remainingSeconds*100)/100)
}

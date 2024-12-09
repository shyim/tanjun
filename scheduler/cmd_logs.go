package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var cmdLog = &cobra.Command{
	Use:   "logs",
	Short: "Shows logs of one run",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rows, err := db.QueryContext(cmd.Context(), "SELECT log, run_at FROM activity WHERE id = ?", args[0])

		if err != nil {
			return err
		}

		for rows.Next() {
			var log string
			var runAt string
			err = rows.Scan(&log, &runAt)
			if err != nil {
				return err
			}

			fmt.Printf("Logs from %s:\n", runAt)

			print(log)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cmdLog)
}

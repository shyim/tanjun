package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
	"os"
)

var db *sql.DB

var rootCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Schedule Jobs",
}

func main() {
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	var err error
	db, err = sql.Open("sqlite", "database.db")
	if err != nil {
		panic(err)
	}

	_, _ = db.Exec(`PRAGMA journal_mode = WAL`)
	_, _ = db.Exec(`PRAGMA synchronous = NORMAL`)
	_, _ = db.Exec(`PRAGMA busy_timeout = 5000`)
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS activity\n(\n    id        INTEGER PRIMARY KEY,\n    name      text,\n    run_at    TEXT,\n    exit_code integer,\n    execution_time integer,\n    log       text\n);\n\nCREATE INDEX IF NOT EXISTS activity_name ON activity (name);\n\nCREATE INDEX IF NOT EXISTS activity_run_at_uindex\n    on activity (run_at desc);\n")

	if err != nil {
		panic(err)
	}

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

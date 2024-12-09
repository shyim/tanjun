package main

import (
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/docker/docker/client"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var c *cron.Cron

var cmdServer = &cobra.Command{
	Use:   "server",
	Short: "Run the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _ = db.Exec("DROP TABLE IF EXISTS jobs")
		_, _ = db.Exec(`CREATE TABLE jobs (name TEXT PRIMARY KEY, schedule TEXT NOT NULL, last_execution TEXT NULL, next_execution TEXT NOT NULL, last_exit_code INTEGER NULL)`)

		dockerClient, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())

		if err != nil {
			return err
		}

		_, err = dockerClient.Info(cmd.Context())

		if err != nil {
			return err
		}

		schedulerConfigEnv := os.Getenv("SCHEDULER_CONFIG")

		if schedulerConfigEnv == "" {
			return fmt.Errorf("no scheduler config found")
		}

		var schedulerConfig SchedulerConfig

		if err := json.Unmarshal([]byte(schedulerConfigEnv), &schedulerConfig); err != nil {
			return fmt.Errorf("cannot parse scheduler config: %w", err)
		}

		c = cron.New()

		for _, job := range schedulerConfig.Jobs {
			job.dockerClient = dockerClient
			job.ContainerID = schedulerConfig.ContainerID
			if _, err := c.AddJob(job.Cron, job); err != nil {
				return err
			}

			for _, entry := range c.Entries() {
				if (entry.Job).(Job).Name == job.Name {
					if _, err := db.Exec("INSERT INTO jobs (name, schedule, next_execution) VALUES (?,?, ?)", job.Name, job.Cron, entry.Schedule.Next(time.Now()).Format("2006-01-02 15:04:05")); err != nil {
						return err
					}
				}
			}

			log.Infof("Added job: %s", job.Name)
		}

		if _, err := c.AddFunc("@every 1h", func() {
			timeBeforeOneWeek := time.Now().AddDate(0, 0, -7)
			if _, err := db.Exec("DELETE FROM activity WHERE run_at < ?", timeBeforeOneWeek.Format("2006-01-02 15:04:05")); err != nil {
				log.Errorf("Could not delete old activities: %s", err)
			}
		}); err != nil {
			return err
		}

		c.Run()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cmdServer)
}

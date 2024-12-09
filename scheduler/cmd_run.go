package main

import (
	"encoding/json"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"os"
)

var cmdRun = &cobra.Command{
	Use:   "run",
	Short: "Run a job out of schedule",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		schedulerConfigEnv := os.Getenv("SCHEDULER_CONFIG")

		if schedulerConfigEnv == "" {
			return fmt.Errorf("no scheduler config found")
		}

		var schedulerConfig SchedulerConfig

		if err := json.Unmarshal([]byte(schedulerConfigEnv), &schedulerConfig); err != nil {
			return fmt.Errorf("cannot parse scheduler config: %w", err)
		}

		dockerClient, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())

		if err != nil {
			return err
		}

		found := false
		for _, job := range schedulerConfig.Jobs {
			if job.Name == args[0] {
				found = true
				job.dockerClient = dockerClient
				job.ContainerID = schedulerConfig.ContainerID
				job.ManualExecute = true
				job.Run()
			}
		}

		if !found {
			return fmt.Errorf("could not found job %s", args[0])
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cmdRun)
}

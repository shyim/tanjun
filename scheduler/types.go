package main

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type SchedulerConfig struct {
	Jobs        []Job  `json:"jobs"`
	ContainerID string `json:"container_id"`
}

type Job struct {
	ManualExecute bool
	ContainerID   string
	Name          string `json:"name"`
	Command       string `json:"command"`
	Cron          string `json:"schedule"`
	dockerClient  *client.Client
}

func (j Job) Run() {
	exec, err := j.dockerClient.ContainerExecCreate(context.Background(), j.ContainerID, container.ExecOptions{
		AttachStderr: true,
		AttachStdout: true,
		Cmd: []string{
			"sh",
			"-c",
			j.Command,
		},
	})

	if err != nil {
		log.Errorf("error creating exec: %s", err)
		return
	}

	now := time.Now()

	resp, err := j.dockerClient.ContainerExecAttach(context.Background(), exec.ID, container.ExecStartOptions{})

	if err != nil {
		log.Errorf("error attaching to exec: %s", err)
		return
	}

	pr, pw := io.Pipe()

	go func() {
		_, _ = stdcopy.StdCopy(pw, pw, resp.Reader)
		resp.Close()
		if err := pw.Close(); err != nil {
			log.Errorf("Failed to close pipe writer: %v", err)
		}
	}()

	buffer := bufio.NewScanner(pr)

	var strBufffer strings.Builder

	for buffer.Scan() {
		log.Infof("Job: %s, Output: %s", j.Name, buffer.Text())
		strBufffer.WriteString(buffer.Text() + "\n")
	}

	inspect, err := j.dockerClient.ContainerExecInspect(context.Background(), exec.ID)

	if err != nil {
		log.Errorf("error inspecting exec: %s", err)
		return
	}

	if j.ManualExecute {
		// dont persist manual executions into the database
		return
	}

	nextExecutionTime := ""

	for _, entry := range c.Entries() {
		if entry.Job.(Job).Name == j.Name {
			nextExecutionTime = entry.Schedule.Next(time.Now()).Format("2006-01-02 15:04:05")
		}
	}

	currentTime := time.Now().Format("2006-01-02 15:04:05")
	_, err = db.Exec("UPDATE jobs SET last_execution = ?, next_execution = ?, last_exit_code = ? WHERE name = ?", currentTime, nextExecutionTime, inspect.ExitCode, j.Name)

	if err != nil {
		log.Errorf("error updating job: %s", err)
		return
	}

	diff := time.Since(now).Milliseconds()

	_, err = db.Exec("INSERT INTO activity (name, run_at, exit_code, log, execution_time) VALUES (?, ?, ?, ?, ?)", j.Name, currentTime, inspect.ExitCode, strBufffer.String(), diff)

	if err != nil {
		log.Errorf("error inserting activity: %s", err)
		return
	}

	strBufffer.Reset()

	if inspect.ExitCode != 0 {
		log.Errorf("Job: %s, exited with error code: %d", j.Name, inspect.ExitCode)
	}
}

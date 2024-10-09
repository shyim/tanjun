package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type dockerMessage struct {
	ID          string `json:"id"`
	Stream      string `json:"stream"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string
	}
	Status   string `json:"status"`
	Progress string `json:"progress"`
}

func logDockerResponse(response io.ReadCloser) error {
	defer response.Close()

	scanner := bufio.NewScanner(response)
	msg := dockerMessage{}

	for scanner.Scan() {
		line := scanner.Bytes()

		msg.ID = ""
		msg.Stream = ""
		msg.Error = ""
		msg.ErrorDetail.Message = ""
		msg.Status = ""
		msg.Progress = ""

		if err := json.Unmarshal(line, &msg); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to unmarshal line [%s] ==> %v\n", string(line), err)
			continue
		}

		if msg.Error != "" {
			return fmt.Errorf("docker error: %s", msg.Error)
		}

		if msg.ErrorDetail.Message != "" {
			return fmt.Errorf("docker error: %s", msg.ErrorDetail.Message)
		}

		if msg.Status != "" {
			if msg.Progress != "" {
				fmt.Fprintf(os.Stdout, "%s :: %s :: %s\n", msg.Status, msg.ID, msg.Progress)
			} else {
				fmt.Fprintf(os.Stdout, "%s :: %s\n", msg.Status, msg.ID)
			}
		} else if msg.Stream != "" {
			fmt.Fprintf(os.Stdout, "%s\n", msg.Stream)
		}
	}

	return nil
}

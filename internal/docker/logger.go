package docker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pterm/pterm"
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

func logDockerResponse(name string, response io.ReadCloser) error {
	defer func() {
		if err := response.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close docker response: %v\n", err)
		}
	}()

	spinnerInfo, err := pterm.DefaultSpinner.Start(fmt.Sprintf("Pulling image: %s", name))

	if err != nil {
		return err
	}

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
			spinnerInfo.Fail(msg.Error)
			return fmt.Errorf("docker error: %s", msg.Error)
		}

		if msg.ErrorDetail.Message != "" {
			spinnerInfo.Fail(msg.ErrorDetail.Message)
			return fmt.Errorf("docker error: %s", msg.ErrorDetail.Message)
		}

		if strings.HasPrefix(msg.Status, "Pulling from") || strings.HasPrefix(msg.Status, "Digest:") || strings.HasPrefix(msg.Status, "Status:") {
			continue
		}

		if msg.Status != "" {
			if msg.Progress != "" {
				spinnerInfo.UpdateText(msg.Progress)
			} else {
				spinnerInfo.UpdateText(msg.Status)
			}
		} else if msg.Stream != "" {
			spinnerInfo.UpdateText(msg.Stream)
		}
	}

	spinnerInfo.Success(fmt.Sprintf("Image pulled: %s", name))

	return nil
}

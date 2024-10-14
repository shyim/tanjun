package onepassword

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/shyim/tanjun/internal/config"
)

func ResolveSecrets(ctx context.Context, secret config.ProjectOnePassword) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, "op", "--vault", secret.Vault, "item", "get", secret.Name, "--format", "json")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("error running 1password command: %w, %s", err, string(output))
	}

	var response OnePasswordResponse

	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("error unmarshalling 1password response: %w, %s", err, string(output))
	}

	onePasswordSecrets := make(map[string]string)

	for _, field := range response.Fields {
		if field.Value == "" || field.Label == "" {
			continue
		}

		onePasswordSecrets[field.Label] = field.Value
	}

	for _, field := range secret.OmitFields {
		delete(onePasswordSecrets, field)
	}

	for key, value := range secret.RemapFields {
		if _, ok := onePasswordSecrets[value]; ok {
			onePasswordSecrets[key] = onePasswordSecrets[value]
			delete(onePasswordSecrets, value)
		}
	}

	return onePasswordSecrets, nil
}

type OnePasswordResponse struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version int    `json:"version"`
	Vault   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"vault"`
	Category     string    `json:"category"`
	LastEditedBy string    `json:"last_edited_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Sections     []struct {
		ID string `json:"id"`
	} `json:"sections"`
	Fields []struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Purpose   string `json:"purpose,omitempty"`
		Label     string `json:"label"`
		Reference string `json:"reference"`
		Section   struct {
			ID string `json:"id"`
		} `json:"section,omitempty"`
		Value string `json:"value,omitempty"`
	} `json:"fields"`
}

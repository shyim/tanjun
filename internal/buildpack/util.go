package buildpack

import (
	"encoding/json"
	"os"
)

func readJSONFile(path string, p interface{}) error {
	bytes, err := os.ReadFile(path)

	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, p)
}

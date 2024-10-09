package main

import (
	"bytes"
	"encoding/json"
	"github.com/invopop/jsonschema"
	"github.com/shyim/tanjun/internal/config"
	"log"
	"os"
)

func main() {
	r := new(jsonschema.Reflector)
	if err := r.AddGoComments("github.com/shyim/tanjun/internal/config", "./internal/config"); err != nil {
		log.Fatal(err)
	}
	r.FieldNameTag = "yaml"
	r.RequiredFromJSONSchemaTags = true
	schema := r.Reflect(config.ProjectConfig{})
	b := new(bytes.Buffer)
	enc := json.NewEncoder(b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(schema); err != nil {
		log.Fatal(err)
	}
	//nolint:gosec  // gosec wants us to use 0600, but making this globally readable is preferred.
	if err := os.WriteFile("schema.json", b.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}
}

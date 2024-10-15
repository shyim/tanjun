package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/shyim/tanjun/internal/config"
	"github.com/shyim/tanjun/internal/docker"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func main() {
	allServices := docker.GetAllServices()

	properties := orderedmap.New[string, *jsonschema.Schema]()

	types := make([]interface{}, 0)
	allOf := []*jsonschema.Schema{}

	for _, svc := range allServices {
		for _, t := range svc.SupportedTypes() {
			types = append(types, t)

			allOf = append(allOf, &jsonschema.Schema{
				If: &jsonschema.Schema{
					Properties: newOrderedMap(map[string]*jsonschema.Schema{
						"type": {
							Const: t,
						},
					}),
				},
				Then: &jsonschema.Schema{
					Properties: newOrderedMap(map[string]*jsonschema.Schema{
						"settings": svc.ConfigSchema(t),
					}),
				},
			})
		}
	}

	properties.Set("type", &jsonschema.Schema{
		Type: "string",
		Enum: types,
	})

	config.SetServiceSchema(&jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"type"},
		AllOf:      allOf,
	})

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

func newOrderedMap(schemaMap map[string]*jsonschema.Schema) *orderedmap.OrderedMap[string, *jsonschema.Schema] {
	om := orderedmap.New[string, *jsonschema.Schema]()

	for key, value := range schemaMap {
		om.Set(key, value)
	}

	return om
}

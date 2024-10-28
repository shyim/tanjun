package buildpack

import (
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type ConfigSettings map[string]interface{}

type Config struct {
	Type     string         `json:"type"`
	Settings ConfigSettings `json:"settings"`
}

func (e Config) JSONSchema() *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()

	types := make([]interface{}, 0)
	var allOf []*jsonschema.Schema

	for _, lang := range supportedLanguages {
		types = append(types, lang.Name())

		allOf = append(allOf, &jsonschema.Schema{
			If: &jsonschema.Schema{
				Properties: newOrderedMap(map[string]*jsonschema.Schema{
					"type": {
						Const: lang.Name(),
					},
				}),
			},
			Then: &jsonschema.Schema{
				Properties: newOrderedMap(map[string]*jsonschema.Schema{
					"settings": lang.Schema(),
				}),
			},
		})
	}

	properties.Set("type", &jsonschema.Schema{
		Type: "string",
		Enum: types,
	})

	return &jsonschema.Schema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"type"},
		AllOf:      allOf,
	}
}

func newOrderedMap(schemaMap map[string]*jsonschema.Schema) *orderedmap.OrderedMap[string, *jsonschema.Schema] {
	om := orderedmap.New[string, *jsonschema.Schema]()

	for key, value := range schemaMap {
		om.Set(key, value)
	}

	return om
}

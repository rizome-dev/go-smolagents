package smolagentsgo

import "fmt"

type (
	Tool struct {
		name        string
		description string
		inputs      map[string]any
		output_type string
	}
)

func (t *Tool) toString() string {
	return fmt.Sprintf("name: %s, description: %s, inputs: %s, output_type: %s", t.name, t.description, t.inputs, t.output_type)
}

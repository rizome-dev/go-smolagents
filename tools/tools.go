// Package tools provides interfaces and implementations for agent tools.
// It defines the base Tool interface and provides concrete implementations
// that agents can use to interact with external systems and resources.
package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// AuthorizedTypes defines the allowed types for tool inputs.
// These types control what data can be passed to tools.
var AuthorizedTypes = []string{
	"string",
	"boolean",
	"integer",
	"number",
	"image",
	"audio",
	"array",
	"object",
	"any",
	"null",
}

// ConversionDict maps Go types to JSON schema types.
// This is used when generating tool schemas.
var ConversionDict = map[string]string{
	"string": "string",
	"bool":   "boolean",
	"int":    "integer",
	"float":  "number",
}

// InputProperty defines a property for a tool input.
// Each property has a type, description, and optional nullable flag.
type InputProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Nullable    bool   `json:"nullable,omitempty"`
}

// Tool defines the interface for all tools that can be used by agents.
// Any type implementing this interface can be used as a tool by agents.
type Tool interface {
	// Name returns the name of the tool
	Name() string

	// Description returns the description of the tool
	Description() string

	// Inputs returns the inputs schema for the tool
	Inputs() map[string]InputProperty

	// OutputType returns the output type of the tool
	OutputType() string

	// Setup performs any necessary initialization
	Setup() error

	// Forward executes the tool with the given arguments
	Forward(args map[string]interface{}) (interface{}, error)

	// Call is a convenience method that combines setup and forward
	Call(args map[string]interface{}, sanitizeInputsOutputs bool) (interface{}, error)
}

// BaseTool provides a base implementation for tools.
// It handles common tool functionality like validation and execution.
type BaseTool struct {
	name        string
	description string
	inputs      map[string]InputProperty
	outputType  string
	initialized bool
	forward     func(args map[string]interface{}) (interface{}, error)
}

// NewBaseTool creates a new BaseTool with the provided parameters.
// It validates the tool name, description, inputs, and forward function.
func NewBaseTool(
	name string,
	description string,
	inputs map[string]InputProperty,
	outputType string,
	forward func(args map[string]interface{}) (interface{}, error),
) (*BaseTool, error) {
	if name == "" {
		return nil, fmt.Errorf("tool name cannot be empty")
	}

	if description == "" {
		return nil, fmt.Errorf("tool description cannot be empty")
	}

	if forward == nil {
		return nil, fmt.Errorf("forward function cannot be nil")
	}

	// Validate output type
	if outputType == "" {
		outputType = "any"
	}

	// Validate input properties
	for name, prop := range inputs {
		if prop.Type == "" {
			return nil, fmt.Errorf("type cannot be empty for input %s", name)
		}

		validType := false
		for _, t := range AuthorizedTypes {
			if t == prop.Type {
				validType = true
				break
			}
		}

		if !validType {
			return nil, fmt.Errorf("invalid type %s for input %s", prop.Type, name)
		}

		if prop.Description == "" {
			return nil, fmt.Errorf("description cannot be empty for input %s", name)
		}
	}

	return &BaseTool{
		name:        name,
		description: description,
		inputs:      inputs,
		outputType:  outputType,
		initialized: false,
		forward:     forward,
	}, nil
}

// Name returns the name of the tool.
func (t *BaseTool) Name() string {
	return t.name
}

// Description returns the description of the tool.
func (t *BaseTool) Description() string {
	return t.description
}

// Inputs returns the inputs schema for the tool.
func (t *BaseTool) Inputs() map[string]InputProperty {
	return t.inputs
}

// OutputType returns the output type of the tool.
func (t *BaseTool) OutputType() string {
	return t.outputType
}

// Setup performs any necessary initialization.
// For BaseTool, this just marks the tool as initialized.
func (t *BaseTool) Setup() error {
	t.initialized = true
	return nil
}

// Forward executes the tool with the given arguments.
// It delegates to the provided forward function.
func (t *BaseTool) Forward(args map[string]interface{}) (interface{}, error) {
	return t.forward(args)
}

// Call is a convenience method that combines setup and forward.
// It handles initialization, input validation, and output sanitization.
func (t *BaseTool) Call(args map[string]interface{}, sanitizeInputsOutputs bool) (interface{}, error) {
	if !t.initialized {
		if err := t.Setup(); err != nil {
			return nil, fmt.Errorf("failed to setup tool: %w", err)
		}
	}

	// Validate inputs if sanitization is enabled
	if sanitizeInputsOutputs {
		if err := t.validateInputs(args); err != nil {
			return nil, err
		}
	}

	// Call the forward function
	result, err := t.Forward(args)
	if err != nil {
		return nil, err
	}

	// Sanitize output if needed
	if sanitizeInputsOutputs {
		result = sanitizeOutput(result, t.outputType)
	}

	return result, nil
}

// validateInputs checks if the provided arguments match the expected inputs schema.
// It validates presence, type, and nullability of each argument.
func (t *BaseTool) validateInputs(args map[string]interface{}) error {
	// Check for missing required inputs
	for name, prop := range t.inputs {
		if _, ok := args[name]; !ok && !prop.Nullable {
			return fmt.Errorf("missing required input %s", name)
		}
	}

	// Check for unexpected inputs
	for name := range args {
		if _, ok := t.inputs[name]; !ok {
			return fmt.Errorf("unexpected input %s", name)
		}
	}

	// Validate input types
	for name, value := range args {
		prop, ok := t.inputs[name]
		if !ok {
			continue
		}

		if value == nil {
			if !prop.Nullable {
				return fmt.Errorf("input %s cannot be null", name)
			}
			continue
		}

		// Check type
		switch prop.Type {
		case "string":
			_, ok := value.(string)
			if !ok {
				return fmt.Errorf("input %s must be a string", name)
			}
		case "boolean":
			_, ok := value.(bool)
			if !ok {
				return fmt.Errorf("input %s must be a boolean", name)
			}
		case "integer":
			switch v := value.(type) {
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
				// Valid integer type
			case float32, float64:
				// Check if the float is actually an integer
				f := reflect.ValueOf(v).Float()
				if float64(int(f)) != f {
					return fmt.Errorf("input %s must be an integer", name)
				}
			default:
				return fmt.Errorf("input %s must be an integer", name)
			}
		case "number":
			switch value.(type) {
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
				// Valid number type
			default:
				return fmt.Errorf("input %s must be a number", name)
			}
		case "image":
			switch value.(type) {
			case interface{ GetType() string }, string: // Allow image or URL string
				// Valid image type
			default:
				return fmt.Errorf("input %s must be an image", name)
			}
		case "audio":
			switch value.(type) {
			case interface{ GetType() string }, string: // Allow audio or URL string
				// Valid audio type
			default:
				return fmt.Errorf("input %s must be an audio", name)
			}
		case "array":
			v := reflect.ValueOf(value)
			if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
				return fmt.Errorf("input %s must be an array", name)
			}
		case "object":
			v := reflect.ValueOf(value)
			if v.Kind() != reflect.Map && v.Kind() != reflect.Struct {
				return fmt.Errorf("input %s must be an object", name)
			}
		case "any":
			// Any type is allowed
		}
	}

	return nil
}

// sanitizeOutput ensures the output matches the expected type.
// It converts the output to the appropriate type if necessary.
func sanitizeOutput(output interface{}, outputType string) interface{} {
	if output == nil {
		return nil
	}

	switch outputType {
	case "string":
		if s, ok := output.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", output)
	case "boolean":
		if b, ok := output.(bool); ok {
			return b
		}
		return false
	case "integer":
		switch v := output.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return v
		case float32, float64:
			return int(reflect.ValueOf(v).Float())
		default:
			return 0
		}
	case "number":
		switch v := output.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return v
		default:
			return 0.0
		}
	case "array":
		v := reflect.ValueOf(output)
		if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
			return output
		}
		return []interface{}{output}
	case "object":
		v := reflect.ValueOf(output)
		if v.Kind() == reflect.Map || v.Kind() == reflect.Struct {
			return output
		}
		return map[string]interface{}{"value": output}
	default:
		return output
	}
}

// FinalAnswerTool is a special tool that provides the final answer from an agent.
// This is used to terminate agent execution with a result.
type FinalAnswerTool struct {
	*BaseTool
}

// NewFinalAnswerTool creates a new FinalAnswerTool.
// This tool takes an answer parameter and returns it as the final answer.
func NewFinalAnswerTool() (*FinalAnswerTool, error) {
	baseTool, err := NewBaseTool(
		"final_answer",
		"Use this tool to provide the final answer to the task.",
		map[string]InputProperty{
			"answer": {
				Type:        "any",
				Description: "The final answer to the task.",
			},
		},
		"string",
		func(args map[string]interface{}) (interface{}, error) {
			answer, ok := args["answer"]
			if !ok {
				return nil, fmt.Errorf("missing required input 'answer'")
			}
			return answer, nil
		},
	)

	if err != nil {
		return nil, err
	}

	return &FinalAnswerTool{
		BaseTool: baseTool,
	}, nil
}

// GetToolJSONSchema generates a JSON schema for a tool.
// This is used to create a schema that can be provided to language models.
func GetToolJSONSchema(tool Tool) (string, error) {
	schema := map[string]interface{}{
		"name":        tool.Name(),
		"description": tool.Description(),
		"parameters": map[string]interface{}{
			"type": "object",
			"properties": func() map[string]interface{} {
				props := make(map[string]interface{})
				for name, prop := range tool.Inputs() {
					propSchema := map[string]interface{}{
						"type":        prop.Type,
						"description": prop.Description,
					}
					if prop.Nullable {
						propSchema["nullable"] = true
					}
					props[name] = propSchema
				}
				return props
			}(),
			"required": func() []string {
				var required []string
				for name, prop := range tool.Inputs() {
					if !prop.Nullable {
						required = append(required, name)
					}
				}
				return required
			}(),
		},
	}

	jsonSchema, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to generate JSON schema: %w", err)
	}

	return string(jsonSchema), nil
}

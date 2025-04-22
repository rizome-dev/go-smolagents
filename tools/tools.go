// Package tools provides interfaces and implementations for agent tools
package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"github.com/rizome-dev/smolagentsgo/agent_types"
)

// AuthorizedTypes defines the allowed types for tool inputs
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

// ConversionDict maps Go types to JSON schema types
var ConversionDict = map[string]string{
	"string": "string",
	"bool":   "boolean",
	"int":    "integer",
	"float":  "number",
}

// InputProperty defines a property for a tool input
type InputProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Nullable    bool   `json:"nullable,omitempty"`
}

// Tool defines the interface for all tools that can be used by agents
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

// BaseTool provides a base implementation for tools
type BaseTool struct {
	name        string
	description string
	inputs      map[string]InputProperty
	outputType  string
	initialized bool
	forward     func(args map[string]interface{}) (interface{}, error)
}

// NewBaseTool creates a new BaseTool
func NewBaseTool(
	name string,
	description string,
	inputs map[string]InputProperty,
	outputType string,
	forward func(args map[string]interface{}) (interface{}, error),
) *BaseTool {
	if err := validateToolName(name); err != nil {
		panic(err)
	}

	if err := validateToolInputs(inputs); err != nil {
		panic(err)
	}

	if err := validateToolOutputType(outputType); err != nil {
		panic(err)
	}

	return &BaseTool{
		name:        name,
		description: description,
		inputs:      inputs,
		outputType:  outputType,
		forward:     forward,
	}
}

// Name returns the name of the tool
func (t *BaseTool) Name() string {
	return t.name
}

// Description returns the description of the tool
func (t *BaseTool) Description() string {
	return t.description
}

// Inputs returns the inputs schema for the tool
func (t *BaseTool) Inputs() map[string]InputProperty {
	return t.inputs
}

// OutputType returns the output type of the tool
func (t *BaseTool) OutputType() string {
	return t.outputType
}

// Setup performs any necessary initialization
func (t *BaseTool) Setup() error {
	t.initialized = true
	return nil
}

// Forward executes the tool with the given arguments
func (t *BaseTool) Forward(args map[string]interface{}) (interface{}, error) {
	return t.forward(args)
}

// Call is a convenience method that combines setup and forward
func (t *BaseTool) Call(args map[string]interface{}, sanitizeInputsOutputs bool) (interface{}, error) {
	if !t.initialized {
		if err := t.Setup(); err != nil {
			return nil, fmt.Errorf("failed to initialize tool: %w", err)
		}
	}

	// Handle the case where args might be a map or a struct
	argsMap := args
	if reflect.TypeOf(args).Kind() != reflect.Map {
		// Convert struct to map
		data, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal args: %w", err)
		}

		if err := json.Unmarshal(data, &argsMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal args to map: %w", err)
		}
	}

	if sanitizeInputsOutputs {
		// Process arguments
		for k, v := range argsMap {
			if at, ok := v.(agent_types.AgentType); ok {
				argsMap[k] = at.ToRaw()
			}
		}
	}

	output, err := t.Forward(argsMap)
	if err != nil {
		return nil, err
	}

	if sanitizeInputsOutputs {
		output = agent_types.HandleAgentOutputTypes(output, t.outputType)
	}

	return output, nil
}

// FinalAnswerTool is a special tool that signals the agent has reached a final answer
type FinalAnswerTool struct {
	*BaseTool
}

// NewFinalAnswerTool creates a new FinalAnswerTool
func NewFinalAnswerTool() *FinalAnswerTool {
	tool := &FinalAnswerTool{
		BaseTool: NewBaseTool(
			"final_answer",
			"Use this to provide your final answer to the user's query.",
			map[string]InputProperty{
				"answer": {
					Type:        "string",
					Description: "The final answer to provide to the user.",
				},
			},
			"string",
			func(args map[string]interface{}) (interface{}, error) {
				answer, ok := args["answer"]
				if !ok {
					return nil, fmt.Errorf("missing required 'answer' argument")
				}
				return answer, nil
			},
		),
	}

	return tool
}

// Validation helper functions

// validateToolName checks if a tool name is valid
func validateToolName(name string) error {
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	// Check for valid identifier name
	matched, err := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	if err != nil {
		return fmt.Errorf("error checking tool name validity: %w", err)
	}

	if !matched {
		return fmt.Errorf("invalid tool name '%s': must be a valid identifier", name)
	}

	// Check for reserved keywords
	for _, keyword := range []string{
		"break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough",
		"for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range",
		"return", "select", "struct", "switch", "type", "var",
	} {
		if name == keyword {
			return fmt.Errorf("invalid tool name '%s': cannot be a reserved keyword", name)
		}
	}

	return nil
}

// validateToolInputs checks if tool inputs are valid
func validateToolInputs(inputs map[string]InputProperty) error {
	if len(inputs) == 0 {
		return fmt.Errorf("tool inputs cannot be empty")
	}

	for name, input := range inputs {
		if input.Type == "" || input.Description == "" {
			return fmt.Errorf("input '%s' must have both 'type' and 'description'", name)
		}

		typeValid := false
		for _, validType := range AuthorizedTypes {
			if input.Type == validType {
				typeValid = true
				break
			}
		}

		if !typeValid {
			return fmt.Errorf("input '%s': type '%s' is not an authorized value, should be one of %v", name, input.Type, AuthorizedTypes)
		}
	}

	return nil
}

// validateToolOutputType checks if a tool output type is valid
func validateToolOutputType(outputType string) error {
	if outputType == "" {
		return fmt.Errorf("tool output type cannot be empty")
	}

	typeValid := false
	for _, validType := range AuthorizedTypes {
		if outputType == validType {
			typeValid = true
			break
		}
	}

	if !typeValid {
		return fmt.Errorf("output type '%s' is not an authorized value, should be one of %v", outputType, AuthorizedTypes)
	}

	return nil
}

// GetToolJSONSchema returns the JSON schema for a tool
func GetToolJSONSchema(tool Tool) map[string]interface{} {
	properties := make(map[string]interface{})
	required := []string{}

	for key, value := range tool.Inputs() {
		prop := map[string]interface{}{
			"type":        value.Type,
			"description": value.Description,
		}

		if value.Type == "any" {
			prop["type"] = "string"
		}

		if !value.Nullable {
			required = append(required, key)
		}

		properties[key] = prop
	}

	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}

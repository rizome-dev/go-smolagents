// Copyright 2025 Rizome Labs, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tools provides the tool system for smolagents.
//
// This includes the core Tool interface, base implementations, input validation,
// type conversion, serialization, and integration capabilities.
package tools

import (
	"fmt"
	"reflect"

	"github.com/rizome-dev/go-smolagents/pkg/agent_types"
	"github.com/rizome-dev/go-smolagents/pkg/utils"
)

// AuthorizedTypes lists all valid input/output types for tools
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
	"string":  "string",
	"int":     "integer",
	"int32":   "integer",
	"int64":   "integer",
	"float32": "number",
	"float64": "number",
	"bool":    "boolean",
}

// ToolInput represents an input parameter for a tool
type ToolInput struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Nullable    bool        `json:"nullable,omitempty"`
	Required    bool        `json:"required,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// NewToolInput creates a new ToolInput
func NewToolInput(inputType, description string, nullable ...bool) *ToolInput {
	isNullable := false
	if len(nullable) > 0 {
		isNullable = nullable[0]
	}

	return &ToolInput{
		Type:        inputType,
		Description: description,
		Nullable:    isNullable,
	}
}

// ToDict converts the tool input to a dictionary representation
func (ti *ToolInput) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"type":        ti.Type,
		"description": ti.Description,
	}

	if ti.Nullable {
		result["nullable"] = ti.Nullable
	}

	if ti.Required {
		result["required"] = ti.Required
	}

	if ti.Default != nil {
		result["default"] = ti.Default
	}

	return result
}

// Tool represents the main interface for all tools used by agents
type Tool interface {
	// Core execution method - must be implemented by all tools
	Forward(args ...interface{}) (interface{}, error)

	// Entry point that handles input/output sanitization
	Call(args ...interface{}) (interface{}, error)

	// Lazy initialization for expensive operations
	Setup() error

	// Validates tool configuration
	Validate() error

	// Serialization methods
	ToDict() map[string]interface{}

	// Metadata accessors
	GetName() string
	GetDescription() string
	GetInputs() map[string]*ToolInput
	GetOutputType() string

	// State management
	IsSetup() bool
	SetSetup(bool)
}

// BaseTool provides default implementations for the Tool interface
type BaseTool struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Inputs      map[string]*ToolInput `json:"inputs"`
	OutputType  string                `json:"output_type"`

	// Internal state
	isSetup bool

	// Implementation function (for function-based tools)
	ForwardFunc func(args ...interface{}) (interface{}, error) `json:"-"`
}

// NewBaseTool creates a new BaseTool instance
func NewBaseTool(name, description string, inputs map[string]*ToolInput, outputType string) *BaseTool {
	if inputs == nil {
		inputs = make(map[string]*ToolInput)
	}

	return &BaseTool{
		Name:        name,
		Description: description,
		Inputs:      inputs,
		OutputType:  outputType,
		isSetup:     false,
	}
}

// GetName implements Tool
func (bt *BaseTool) GetName() string {
	return bt.Name
}

// GetDescription implements Tool
func (bt *BaseTool) GetDescription() string {
	return bt.Description
}

// GetInputs implements Tool
func (bt *BaseTool) GetInputs() map[string]*ToolInput {
	return bt.Inputs
}

// GetOutputType implements Tool
func (bt *BaseTool) GetOutputType() string {
	return bt.OutputType
}

// IsSetup implements Tool
func (bt *BaseTool) IsSetup() bool {
	return bt.isSetup
}

// SetSetup implements Tool
func (bt *BaseTool) SetSetup(setup bool) {
	bt.isSetup = setup
}

// Setup implements Tool (default implementation)
func (bt *BaseTool) Setup() error {
	if !bt.isSetup {
		bt.isSetup = true
	}
	return nil
}

// Validate implements Tool
func (bt *BaseTool) Validate() error {
	// Validate name
	if bt.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if !utils.IsValidName(bt.Name) {
		return fmt.Errorf("tool name '%s' is not a valid identifier", bt.Name)
	}

	// Validate description
	if bt.Description == "" {
		return fmt.Errorf("tool description cannot be empty")
	}

	// Validate output type
	if bt.OutputType == "" {
		return fmt.Errorf("tool output_type cannot be empty")
	}

	if !isValidType(bt.OutputType) {
		return fmt.Errorf("invalid output_type '%s', must be one of: %v", bt.OutputType, AuthorizedTypes)
	}

	// Validate inputs
	for name, input := range bt.Inputs {
		if !utils.IsValidName(name) {
			return fmt.Errorf("input name '%s' is not a valid identifier", name)
		}

		if !isValidType(input.Type) {
			return fmt.Errorf("invalid input type '%s' for parameter '%s', must be one of: %v",
				input.Type, name, AuthorizedTypes)
		}

		if input.Description == "" {
			return fmt.Errorf("input '%s' must have a description", name)
		}
	}

	return nil
}

// Forward implements Tool (default implementation)
func (bt *BaseTool) Forward(args ...interface{}) (interface{}, error) {
	if bt.ForwardFunc != nil {
		return bt.ForwardFunc(args...)
	}
	return nil, fmt.Errorf("Forward method not implemented")
}

// convertArgsToMap converts various argument formats to a map
func (bt *BaseTool) convertArgsToMap(args []interface{}) map[string]interface{} {
	argsMap := make(map[string]interface{})
	
	if len(args) == 0 {
		return argsMap
	}
	
	if len(args) == 1 {
		// Check if it's already a map
		if argMap, ok := args[0].(map[string]interface{}); ok {
			return argMap
		}
		// Single argument - map to first input parameter
		inputNames := bt.getInputNames()
		if len(inputNames) > 0 {
			argsMap[inputNames[0]] = args[0]
		}
		return argsMap
	}
	
	// Multiple arguments - map to input parameters in order
	inputNames := bt.getInputNames()
	for i, arg := range args {
		if i < len(inputNames) {
			argsMap[inputNames[i]] = arg
		}
	}
	
	return argsMap
}

// callForwardMethod calls the Forward method with appropriate arguments
func (bt *BaseTool) callForwardMethod(convertedMap map[string]interface{}) (interface{}, error) {
	if len(convertedMap) == 0 {
		return bt.Forward()
	}
	
	if len(convertedMap) == 1 {
		// Single argument - extract the value
		for _, v := range convertedMap {
			return bt.Forward(v)
		}
	}
	
	// Multiple arguments - convert to slice in correct order
	argSlice := make([]interface{}, 0, len(convertedMap))
	inputNames := bt.getInputNames()
	for _, name := range inputNames {
		if val, exists := convertedMap[name]; exists {
			argSlice = append(argSlice, val)
		}
	}
	return bt.Forward(argSlice...)
}

// Call implements Tool
func (bt *BaseTool) Call(args ...interface{}) (interface{}, error) {
	// Ensure tool is set up
	if !bt.isSetup {
		if err := bt.Setup(); err != nil {
			return nil, fmt.Errorf("tool setup failed: %w", err)
		}
	}

	// Convert args to map format
	argsMap := bt.convertArgsToMap(args)

	// Handle agent input types
	_, convertedMap := agent_types.HandleAgentInputTypes(nil, argsMap)

	// Validate inputs
	if err := bt.validateInputs(convertedMap); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Call the forward method
	result, err := bt.callForwardMethod(convertedMap)
	if err != nil {
		return nil, err
	}

	// Handle agent output types
	return agent_types.HandleAgentOutputTypes(result, bt.OutputType), nil
}

// ToDict implements Tool
func (bt *BaseTool) ToDict() map[string]interface{} {
	inputs := make(map[string]interface{})
	for name, input := range bt.Inputs {
		inputs[name] = map[string]interface{}{
			"type":        input.Type,
			"description": input.Description,
			"nullable":    input.Nullable,
		}
	}

	return map[string]interface{}{
		"name":        bt.Name,
		"description": bt.Description,
		"inputs":      inputs,
		"output_type": bt.OutputType,
	}
}

// getInputNames returns input parameter names in a consistent order
func (bt *BaseTool) getInputNames() []string {
	names := make([]string, 0, len(bt.Inputs))
	for name := range bt.Inputs {
		names = append(names, name)
	}
	return names
}

// ToOpenAITool converts the tool to OpenAI tool format
func (bt *BaseTool) ToOpenAITool() map[string]interface{} {
	properties := make(map[string]interface{})
	required := make([]string, 0)

	for name, input := range bt.Inputs {
		properties[name] = map[string]interface{}{
			"type":        input.Type,
			"description": input.Description,
		}

		if input.Required {
			required = append(required, name)
		}
	}

	parameters := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		parameters["required"] = required
	}

	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        bt.Name,
			"description": bt.Description,
			"parameters":  parameters,
		},
	}
}

// GetInput returns a specific input parameter
func (bt *BaseTool) GetInput(name string) *ToolInput {
	return bt.Inputs[name]
}

// Teardown cleans up any resources used by the tool
func (bt *BaseTool) Teardown() error {
	// Default implementation does nothing
	return nil
}

// validateInputs validates input arguments against the tool's input schema
func (bt *BaseTool) validateInputs(args map[string]interface{}) error {
	// Check for required parameters
	for name, input := range bt.Inputs {
		value, exists := args[name]

		if !exists {
			if !input.Nullable {
				return fmt.Errorf("required parameter '%s' is missing", name)
			}
			continue
		}

		// Check for null values
		if value == nil {
			if !input.Nullable {
				return fmt.Errorf("parameter '%s' cannot be null", name)
			}
			continue
		}

		// Type validation
		if err := validateValueType(name, value, input.Type); err != nil {
			return err
		}
	}

	// Check for unknown parameters
	for name := range args {
		if _, exists := bt.Inputs[name]; !exists {
			return fmt.Errorf("unknown parameter '%s'", name)
		}
	}

	return nil
}

// validateInteger checks if a value is a valid integer
func validateInteger(name string, value interface{}) error {
	switch value.(type) {
	case int, int32, int64:
		return nil
	case float64:
		// JSON unmarshaling gives us float64 for numbers, check if it's actually an integer
		if v := value.(float64); v != float64(int64(v)) {
			return fmt.Errorf("parameter '%s' must be an integer, got float %v", name, v)
		}
		return nil
	default:
		return fmt.Errorf("parameter '%s' must be an integer, got %T", name, value)
	}
}

// validateNumber checks if a value is a valid number
func validateNumber(name string, value interface{}) error {
	switch value.(type) {
	case int, int32, int64, float32, float64:
		return nil
	default:
		return fmt.Errorf("parameter '%s' must be a number, got %T", name, value)
	}
}

// validateArray checks if a value is a valid array
func validateArray(name string, value interface{}) error {
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return fmt.Errorf("parameter '%s' must be an array, got %T", name, value)
	}
	return nil
}

// validateMediaType checks if a value is a valid media type (image/audio)
func validateMediaType(name string, value interface{}, mediaType string) error {
	switch mediaType {
	case "image":
		switch value.(type) {
		case *agent_types.AgentImage, string, []byte:
			return nil
		default:
			return fmt.Errorf("parameter '%s' must be an image, got %T", name, value)
		}
	case "audio":
		switch value.(type) {
		case *agent_types.AgentAudio, string, []byte:
			return nil
		default:
			return fmt.Errorf("parameter '%s' must be audio, got %T", name, value)
		}
	}
	return fmt.Errorf("unknown media type '%s'", mediaType)
}

// Type validator functions map
var typeValidators = map[string]func(string, interface{}) error{
	"string": func(name string, value interface{}) error {
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter '%s' must be a string, got %T", name, value)
		}
		return nil
	},
	"boolean": func(name string, value interface{}) error {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter '%s' must be a boolean, got %T", name, value)
		}
		return nil
	},
	"integer": validateInteger,
	"number":  validateNumber,
	"array":   validateArray,
	"object": func(name string, value interface{}) error {
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("parameter '%s' must be an object, got %T", name, value)
		}
		return nil
	},
	"any": func(name string, value interface{}) error {
		return nil // Any type is allowed
	},
	"null": func(name string, value interface{}) error {
		return nil // Any type is allowed
	},
}

// validateValueType validates a value against a type constraint
func validateValueType(name string, value interface{}, expectedType string) error {
	// Check if it's a media type
	if expectedType == "image" || expectedType == "audio" {
		return validateMediaType(name, value, expectedType)
	}
	
	// Check if we have a validator for this type
	validator, ok := typeValidators[expectedType]
	if !ok {
		return fmt.Errorf("unknown type constraint '%s' for parameter '%s'", expectedType, name)
	}
	
	return validator(name, value)
}

// isValidType checks if a type is in the authorized types list
func isValidType(typeStr string) bool {
	for _, validType := range AuthorizedTypes {
		if typeStr == validType {
			return true
		}
	}
	return false
}

// FunctionTool represents a tool created from a Go function
type FunctionTool struct {
	*BaseTool
	function interface{}
}

// NewFunctionTool creates a tool from a Go function with metadata
func NewFunctionTool(
	name string,
	description string,
	inputs map[string]*ToolInput,
	outputType string,
	function interface{},
) (*FunctionTool, error) {
	baseTool := NewBaseTool(name, description, inputs, outputType)

	ft := &FunctionTool{
		BaseTool: baseTool,
		function: function,
	}

	// Set up the forward function to call the provided function
	ft.ForwardFunc = ft.callFunction

	// Validate the tool
	if err := ft.Validate(); err != nil {
		return nil, fmt.Errorf("tool validation failed: %w", err)
	}

	return ft, nil
}

// handleSingleArgument processes a single argument for function call
func (ft *FunctionTool) handleSingleArgument(arg interface{}, funcType reflect.Type, numIn int) []reflect.Value {
	var callArgs []reflect.Value
	
	if argMap, ok := arg.(map[string]interface{}); ok {
		// Map arguments to function parameters
		inputNames := ft.getInputNames()
		for i := 0; i < numIn && i < len(inputNames); i++ {
			if val, exists := argMap[inputNames[i]]; exists {
				callArgs = append(callArgs, reflect.ValueOf(val))
			} else {
				// Use zero value for missing arguments
				callArgs = append(callArgs, reflect.Zero(funcType.In(i)))
			}
		}
	} else {
		// Single direct argument
		callArgs = append(callArgs, reflect.ValueOf(arg))
	}
	
	return callArgs
}

// handleMultipleArguments processes multiple arguments for function call
func (ft *FunctionTool) handleMultipleArguments(args []interface{}, funcType reflect.Type, numIn int) []reflect.Value {
	var callArgs []reflect.Value
	
	for i, arg := range args {
		if i < numIn {
			callArgs = append(callArgs, reflect.ValueOf(arg))
		}
	}
	
	return callArgs
}

// prepareCallArguments prepares arguments for function call
func (ft *FunctionTool) prepareCallArguments(args []interface{}, funcType reflect.Type) []reflect.Value {
	numIn := funcType.NumIn()
	if numIn == 0 {
		return []reflect.Value{}
	}
	
	var callArgs []reflect.Value
	
	if len(args) == 1 {
		callArgs = ft.handleSingleArgument(args[0], funcType, numIn)
	} else {
		callArgs = ft.handleMultipleArguments(args, funcType, numIn)
	}
	
	// Pad with zero values if not enough arguments
	for len(callArgs) < numIn {
		callArgs = append(callArgs, reflect.Zero(funcType.In(len(callArgs))))
	}
	
	return callArgs
}

// processReturnValues processes function return values
func (ft *FunctionTool) processReturnValues(results []reflect.Value) (interface{}, error) {
	if len(results) == 0 {
		return nil, nil
	}
	
	if len(results) == 1 {
		return results[0].Interface(), nil
	}
	
	// Check if last result is an error
	lastResult := results[len(results)-1]
	if lastResult.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		// Last return value is an error
		if !lastResult.IsNil() {
			return nil, lastResult.Interface().(error)
		}
		
		// Return other values
		if len(results) == 2 {
			return results[0].Interface(), nil
		}
		
		// Multiple non-error return values
		retVals := make([]interface{}, len(results)-1)
		for i := 0; i < len(results)-1; i++ {
			retVals[i] = results[i].Interface()
		}
		return retVals, nil
	}
	
	// Multiple return values, none are errors
	retVals := make([]interface{}, len(results))
	for i, result := range results {
		retVals[i] = result.Interface()
	}
	return retVals, nil
}

// callFunction calls the underlying Go function with proper argument handling
func (ft *FunctionTool) callFunction(args ...interface{}) (interface{}, error) {
	if ft.function == nil {
		return nil, fmt.Errorf("no function provided")
	}

	funcValue := reflect.ValueOf(ft.function)
	funcType := funcValue.Type()

	if funcType.Kind() != reflect.Func {
		return nil, fmt.Errorf("provided function is not a function")
	}

	// Prepare arguments
	callArgs := ft.prepareCallArguments(args, funcType)

	// Call the function
	results := funcValue.Call(callArgs)

	// Process return values
	return ft.processReturnValues(results)
}

// ToDict implements Tool for FunctionTool
func (ft *FunctionTool) ToDict() map[string]interface{} {
	result := ft.BaseTool.ToDict()
	result["type"] = "function_tool"
	return result
}

// Tool creation helper functions

// CreateSimpleTool creates a simple tool with basic validation
func CreateSimpleTool(name, description, outputType string, function interface{}) (*FunctionTool, error) {
	inputs := make(map[string]*ToolInput)

	// Try to infer inputs from function signature
	if function != nil {
		funcType := reflect.TypeOf(function)
		if funcType.Kind() == reflect.Func {
			numIn := funcType.NumIn()
			for i := 0; i < numIn; i++ {
				paramType := funcType.In(i)
				paramName := fmt.Sprintf("param_%d", i)

				// Convert Go type to schema type
				schemaType := "any"
				if goType, exists := ConversionDict[paramType.String()]; exists {
					schemaType = goType
				}

				inputs[paramName] = NewToolInput(schemaType, fmt.Sprintf("Parameter %d", i))
			}
		}
	}

	return NewFunctionTool(name, description, inputs, outputType, function)
}

// ToolFromMap creates a tool from a map representation
func ToolFromMap(toolMap map[string]interface{}) (Tool, error) {
	name, ok := toolMap["name"].(string)
	if !ok {
		return nil, fmt.Errorf("tool name is required")
	}

	description, ok := toolMap["description"].(string)
	if !ok {
		return nil, fmt.Errorf("tool description is required")
	}

	outputType, ok := toolMap["output_type"].(string)
	if !ok {
		outputType = "any"
	}

	inputs := make(map[string]*ToolInput)
	if inputsMap, ok := toolMap["inputs"].(map[string]interface{}); ok {
		for inputName, inputData := range inputsMap {
			if inputMap, ok := inputData.(map[string]interface{}); ok {
				inputType, _ := inputMap["type"].(string)
				inputDesc, _ := inputMap["description"].(string)
				nullable, _ := inputMap["nullable"].(bool)

				inputs[inputName] = &ToolInput{
					Type:        inputType,
					Description: inputDesc,
					Nullable:    nullable,
				}
			}
		}
	}

	return NewBaseTool(name, description, inputs, outputType), nil
}

// GetToolJSONSchema converts a tool to OpenAI-compatible JSON schema
func GetToolJSONSchema(tool Tool) map[string]interface{} {
	properties := make(map[string]interface{})
	required := []string{}

	for name, input := range tool.GetInputs() {
		properties[name] = map[string]interface{}{
			"type":        input.Type,
			"description": input.Description,
		}

		if !input.Nullable {
			required = append(required, name)
		}
	}

	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        tool.GetName(),
			"description": tool.GetDescription(),
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}

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

// Package tools - ToolCollection and Pipeline functionality
package tools

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ToolCollection manages a collection of tools
type ToolCollection struct {
	Tools       []Tool          `json:"tools"`
	ToolsMap    map[string]Tool `json:"-"`
	orderedKeys []string        // Maintains insertion order
}

// NewToolCollection creates a new tool collection
func NewToolCollection(tools ...[]Tool) *ToolCollection {
	tc := &ToolCollection{
		Tools:       make([]Tool, 0),
		ToolsMap:    make(map[string]Tool),
		orderedKeys: make([]string, 0),
	}

	// Add initial tools if provided
	if len(tools) > 0 {
		for _, tool := range tools[0] {
			tc.AddTool(tool)
		}
	}

	return tc
}

// Add adds a tool to the collection
func (tc *ToolCollection) Add(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("cannot add nil tool")
	}

	// Validate the tool
	if err := tool.Validate(); err != nil {
		return fmt.Errorf("tool validation failed: %w", err)
	}

	name := tool.GetName()

	// Check if tool already exists
	if _, exists := tc.ToolsMap[name]; exists {
		return fmt.Errorf("tool with name '%s' already exists", name)
	}

	tc.Tools = append(tc.Tools, tool)
	tc.ToolsMap[name] = tool
	tc.orderedKeys = append(tc.orderedKeys, name)

	return nil
}

// AddTool is an alias for Add to match test expectations
func (tc *ToolCollection) AddTool(tool Tool) error {
	return tc.Add(tool)
}

// Remove removes a tool from the collection
func (tc *ToolCollection) Remove(name string) bool {
	return tc.RemoveTool(name)
}

// RemoveTool removes a tool from the collection
func (tc *ToolCollection) RemoveTool(name string) bool {
	if _, exists := tc.ToolsMap[name]; !exists {
		return false
	}

	delete(tc.ToolsMap, name)

	// Remove from Tools slice
	for i, tool := range tc.Tools {
		if tool.GetName() == name {
			tc.Tools = append(tc.Tools[:i], tc.Tools[i+1:]...)
			break
		}
	}

	// Remove from ordered keys
	for i, key := range tc.orderedKeys {
		if key == name {
			tc.orderedKeys = append(tc.orderedKeys[:i], tc.orderedKeys[i+1:]...)
			break
		}
	}

	return true
}

// Get retrieves a tool by name
func (tc *ToolCollection) Get(name string) (Tool, bool) {
	return tc.GetTool(name)
}

// GetTool retrieves a tool by name
func (tc *ToolCollection) GetTool(name string) (Tool, bool) {
	tool, exists := tc.ToolsMap[name]
	return tool, exists
}

// Has checks if a tool exists in the collection
func (tc *ToolCollection) Has(name string) bool {
	_, exists := tc.ToolsMap[name]
	return exists
}

// List returns all tools in the collection in insertion order
func (tc *ToolCollection) List() []Tool {
	return tc.Tools
}

// ListNames returns all tool names in insertion order
func (tc *ToolCollection) ListNames() []string {
	names := make([]string, len(tc.Tools))
	for i, tool := range tc.Tools {
		names[i] = tool.GetName()
	}
	return names
}

// ListTools is an alias for ListNames to match test expectations
func (tc *ToolCollection) ListTools() []string {
	return tc.ListNames()
}

// Count returns the number of tools in the collection
func (tc *ToolCollection) Count() int {
	return len(tc.Tools)
}

// Clear removes all tools from the collection
func (tc *ToolCollection) Clear() {
	tc.Tools = make([]Tool, 0)
	tc.ToolsMap = make(map[string]Tool)
	tc.orderedKeys = make([]string, 0)
}

// Merge merges another tool collection into this one
func (tc *ToolCollection) Merge(other *ToolCollection) error {
	for _, tool := range other.List() {
		if err := tc.Add(tool); err != nil {
			// If tool already exists, skip it (don't return error)
			if tc.Has(tool.GetName()) {
				continue
			}
			return err
		}
	}
	return nil
}

// Filter returns a new collection with tools that match the predicate
func (tc *ToolCollection) Filter(predicate func(Tool) bool) *ToolCollection {
	filtered := NewToolCollection()

	for _, tool := range tc.List() {
		if predicate(tool) {
			filtered.Add(tool) // Ignore errors since we're filtering existing valid tools
		}
	}

	return filtered
}

// FindByType returns all tools with the specified output type
func (tc *ToolCollection) FindByType(outputType string) []Tool {
	return tc.Filter(func(tool Tool) bool {
		return tool.GetOutputType() == outputType
	}).List()
}

// FindByPrefix returns all tools whose names start with the given prefix
func (tc *ToolCollection) FindByPrefix(prefix string) []Tool {
	return tc.Filter(func(tool Tool) bool {
		return len(tool.GetName()) >= len(prefix) && tool.GetName()[:len(prefix)] == prefix
	}).List()
}

// ToDict converts the collection to a dictionary representation
func (tc *ToolCollection) ToDict() map[string]interface{} {
	tools := make(map[string]interface{})

	for name, tool := range tc.ToolsMap {
		tools[name] = tool.ToDict()
	}

	return map[string]interface{}{
		"tools": tools,
		"count": len(tc.Tools),
		"order": tc.orderedKeys,
	}
}

// FromDict creates a tool collection from a dictionary representation
func (tc *ToolCollection) FromDict(data map[string]interface{}) error {
	tc.Clear()

	toolsData, ok := data["tools"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid tools data format")
	}

	// Get ordering if available
	var order []string
	if orderData, ok := data["order"].([]interface{}); ok {
		order = make([]string, len(orderData))
		for i, name := range orderData {
			if nameStr, ok := name.(string); ok {
				order[i] = nameStr
			}
		}
	} else {
		// No order specified, use alphabetical
		for name := range toolsData {
			order = append(order, name)
		}
		sort.Strings(order)
	}

	// Add tools in the specified order
	for _, name := range order {
		if toolData, exists := toolsData[name]; exists {
			if toolMap, ok := toolData.(map[string]interface{}); ok {
				tool, err := ToolFromMap(toolMap)
				if err != nil {
					return fmt.Errorf("failed to create tool '%s': %w", name, err)
				}

				if err := tc.Add(tool); err != nil {
					return fmt.Errorf("failed to add tool '%s': %w", name, err)
				}
			}
		}
	}

	return nil
}

// JSON serialization
func (tc *ToolCollection) MarshalJSON() ([]byte, error) {
	return json.Marshal(tc.ToDict())
}

func (tc *ToolCollection) UnmarshalJSON(data []byte) error {
	var dict map[string]interface{}
	if err := json.Unmarshal(data, &dict); err != nil {
		return err
	}

	return tc.FromDict(dict)
}

// ValidateAll validates all tools in the collection
func (tc *ToolCollection) ValidateAll() error {
	for _, tool := range tc.Tools {
		if err := tool.Validate(); err != nil {
			return fmt.Errorf("tool '%s' validation failed: %w", tool.GetName(), err)
		}
	}
	return nil
}

// PipelineTool represents a tool that processes data through a pipeline
type PipelineTool struct {
	*BaseTool

	// Pipeline configuration
	ModelClass         string `json:"model_class,omitempty"`
	DefaultCheckpoint  string `json:"default_checkpoint,omitempty"`
	PreProcessorClass  string `json:"pre_processor_class,omitempty"`
	PostProcessorClass string `json:"post_processor_class,omitempty"`

	// Runtime components (not serialized)
	model         interface{} `json:"-"`
	preProcessor  interface{} `json:"-"`
	postProcessor interface{} `json:"-"`
	device        string      `json:"-"`

	// Pipeline functions
	EncodeFunc func(interface{}) (interface{}, error) `json:"-"`
	DecodeFunc func(interface{}) (interface{}, error) `json:"-"`
}

// NewPipelineTool creates a new pipeline tool
func NewPipelineTool(
	name, description string,
	inputs map[string]*ToolInput,
	outputType string,
	options map[string]interface{},
) *PipelineTool {
	baseTool := NewBaseTool(name, description, inputs, outputType)

	pt := &PipelineTool{
		BaseTool: baseTool,
		device:   "cpu", // Default device
	}

	if options != nil {
		if modelClass, ok := options["model_class"].(string); ok {
			pt.ModelClass = modelClass
		}
		if checkpoint, ok := options["default_checkpoint"].(string); ok {
			pt.DefaultCheckpoint = checkpoint
		}
		if preProcessor, ok := options["pre_processor_class"].(string); ok {
			pt.PreProcessorClass = preProcessor
		}
		if postProcessor, ok := options["post_processor_class"].(string); ok {
			pt.PostProcessorClass = postProcessor
		}
		if device, ok := options["device"].(string); ok {
			pt.device = device
		}
	}

	return pt
}

// Setup implements Tool for PipelineTool
func (pt *PipelineTool) Setup() error {
	if pt.isSetup {
		return nil
	}

	// This is a placeholder for model loading logic
	// In a real implementation, this would:
	// 1. Load the model from the checkpoint
	// 2. Initialize preprocessor and postprocessor
	// 3. Move model to specified device
	// 4. Set up encoding/decoding functions

	pt.isSetup = true
	return nil
}

// Encode processes input through the preprocessor
func (pt *PipelineTool) Encode(input interface{}) (interface{}, error) {
	if !pt.isSetup {
		if err := pt.Setup(); err != nil {
			return nil, fmt.Errorf("pipeline setup failed: %w", err)
		}
	}

	if pt.EncodeFunc != nil {
		return pt.EncodeFunc(input)
	}

	// Default encoding (pass-through)
	return input, nil
}

// Decode processes output through the postprocessor
func (pt *PipelineTool) Decode(output interface{}) (interface{}, error) {
	if pt.DecodeFunc != nil {
		return pt.DecodeFunc(output)
	}

	// Default decoding (pass-through)
	return output, nil
}

// Forward implements Tool for PipelineTool
func (pt *PipelineTool) Forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("pipeline tool requires at least one input")
	}

	// Encode input
	encoded, err := pt.Encode(args[0])
	if err != nil {
		return nil, fmt.Errorf("encoding failed: %w", err)
	}

	// Process (placeholder - in real implementation this would run the model)
	processed := encoded

	// Decode output
	decoded, err := pt.Decode(processed)
	if err != nil {
		return nil, fmt.Errorf("decoding failed: %w", err)
	}

	return decoded, nil
}

// SetDevice sets the device for the pipeline
func (pt *PipelineTool) SetDevice(device string) {
	pt.device = device
	// In real implementation, would move model to new device
}

// GetDevice returns the current device
func (pt *PipelineTool) GetDevice() string {
	return pt.device
}

// ToDict implements Tool for PipelineTool
func (pt *PipelineTool) ToDict() map[string]interface{} {
	result := pt.BaseTool.ToDict()
	result["type"] = "pipeline_tool"
	result["model_class"] = pt.ModelClass
	result["default_checkpoint"] = pt.DefaultCheckpoint
	result["pre_processor_class"] = pt.PreProcessorClass
	result["post_processor_class"] = pt.PostProcessorClass
	return result
}

// ToolRegistry provides a global registry for tools
type ToolRegistry struct {
	*ToolCollection
}

// Global tool registry instance
var globalRegistry = &ToolRegistry{
	ToolCollection: NewToolCollection(),
}

// RegisterTool registers a tool globally
func RegisterTool(tool Tool) error {
	return globalRegistry.Add(tool)
}

// GetRegisteredTool retrieves a globally registered tool
func GetRegisteredTool(name string) (Tool, bool) {
	return globalRegistry.Get(name)
}

// ListRegisteredTools returns all globally registered tools
func ListRegisteredTools() []Tool {
	return globalRegistry.List()
}

// ClearRegistry clears the global tool registry
func ClearRegistry() {
	globalRegistry.Clear()
}

// GetRegistry returns the global tool registry
func GetRegistry() *ToolRegistry {
	return globalRegistry
}

// Utility functions for tool creation

// ToolDecorator represents metadata for creating tools from functions
type ToolDecorator struct {
	Name        string
	Description string
	OutputType  string
	Inputs      map[string]*ToolInput
}

// NewToolDecorator creates a new tool decorator
func NewToolDecorator(name, description, outputType string) *ToolDecorator {
	return &ToolDecorator{
		Name:        name,
		Description: description,
		OutputType:  outputType,
		Inputs:      make(map[string]*ToolInput),
	}
}

// AddInput adds an input parameter to the decorator
func (td *ToolDecorator) AddInput(name, inputType, description string, nullable ...bool) *ToolDecorator {
	td.Inputs[name] = NewToolInput(inputType, description, nullable...)
	return td
}

// Decorate creates a tool from a function using this decorator
func (td *ToolDecorator) Decorate(function interface{}) (*FunctionTool, error) {
	return NewFunctionTool(td.Name, td.Description, td.Inputs, td.OutputType, function)
}

// Tool creation convenience functions

// StringTool creates a simple tool that returns a string
func StringTool(name, description string, function func(string) (string, error)) (*FunctionTool, error) {
	inputs := map[string]*ToolInput{
		"input": NewToolInput("string", "Input text"),
	}

	return NewFunctionTool(name, description, inputs, "string", func(args ...interface{}) (interface{}, error) {
		if len(args) == 0 {
			return function("")
		}

		if str, ok := args[0].(string); ok {
			return function(str)
		}

		return function(fmt.Sprintf("%v", args[0]))
	})
}

// NumberTool creates a simple tool that processes numbers
func NumberTool(name, description string, function func(float64) (float64, error)) (*FunctionTool, error) {
	inputs := map[string]*ToolInput{
		"input": NewToolInput("number", "Input number"),
	}

	return NewFunctionTool(name, description, inputs, "number", func(args ...interface{}) (interface{}, error) {
		if len(args) == 0 {
			return function(0)
		}

		switch v := args[0].(type) {
		case float64:
			return function(v)
		case float32:
			return function(float64(v))
		case int:
			return function(float64(v))
		case int32:
			return function(float64(v))
		case int64:
			return function(float64(v))
		default:
			return nil, fmt.Errorf("input must be a number, got %T", args[0])
		}
	})
}

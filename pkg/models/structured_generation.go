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

// Package models - Structured generation and response format handling
package models

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ResponseFormat represents different structured output formats
type ResponseFormat struct {
	Type        string                 `json:"type"`                  // "json_object", "json_schema", "text"
	JSONSchema  *JSONSchema            `json:"json_schema,omitempty"` // For structured JSON output
	Schema      map[string]interface{} `json:"schema,omitempty"`      // Raw schema definition
	Strict      bool                   `json:"strict,omitempty"`      // Whether to enforce strict schema compliance
	Name        string                 `json:"name,omitempty"`        // Name for the response format
	Description string                 `json:"description,omitempty"` // Description of the format
}

// JSONSchema represents a JSON schema for structured generation
type JSONSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema"`
	Strict      bool                   `json:"strict,omitempty"`
}

// StructuredOutput represents a parsed structured output
type StructuredOutput struct {
	Content  interface{}            `json:"content"`  // The parsed content
	Raw      string                 `json:"raw"`      // Raw text output
	Format   *ResponseFormat        `json:"format"`   // The format used
	Valid    bool                   `json:"valid"`    // Whether output is valid according to schema
	Errors   []string               `json:"errors"`   // Validation errors if any
	Metadata map[string]interface{} `json:"metadata"` // Additional metadata
}

// SchemaValidator provides schema validation functionality
type SchemaValidator struct {
	schemas map[string]*JSONSchema // Registered schemas
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{
		schemas: make(map[string]*JSONSchema),
	}
}

// RegisterSchema registers a JSON schema for validation
func (sv *SchemaValidator) RegisterSchema(schema *JSONSchema) {
	sv.schemas[schema.Name] = schema
}

// ValidateJSON validates JSON data against a schema
func (sv *SchemaValidator) ValidateJSON(data interface{}, schemaName string) (bool, []string) {
	schema, exists := sv.schemas[schemaName]
	if !exists {
		return false, []string{fmt.Sprintf("schema %s not found", schemaName)}
	}

	return sv.validateAgainstSchema(data, schema.Schema)
}

// validateAgainstSchema performs the actual schema validation
func (sv *SchemaValidator) validateAgainstSchema(data interface{}, schema map[string]interface{}) (bool, []string) {
	var errors []string

	// Get schema type
	schemaType, ok := schema["type"].(string)
	if !ok {
		return false, []string{"schema missing type"}
	}

	// Validate based on type
	switch schemaType {
	case "object":
		if !sv.isObject(data) {
			errors = append(errors, "expected object type")
			return false, errors
		}
		objErrors := sv.validateObject(data, schema)
		errors = append(errors, objErrors...)

	case "array":
		if !sv.isArray(data) {
			errors = append(errors, "expected array type")
			return false, errors
		}
		arrayErrors := sv.validateArray(data, schema)
		errors = append(errors, arrayErrors...)

	case "string":
		if !sv.isString(data) {
			errors = append(errors, "expected string type")
			return false, errors
		}
		stringErrors := sv.validateString(data, schema)
		errors = append(errors, stringErrors...)

	case "number", "integer":
		if !sv.isNumber(data) {
			errors = append(errors, fmt.Sprintf("expected %s type", schemaType))
			return false, errors
		}
		numberErrors := sv.validateNumber(data, schema)
		errors = append(errors, numberErrors...)

	case "boolean":
		if !sv.isBoolean(data) {
			errors = append(errors, "expected boolean type")
			return false, errors
		}

	default:
		errors = append(errors, fmt.Sprintf("unsupported schema type: %s", schemaType))
	}

	return len(errors) == 0, errors
}

// validateObject validates an object against object schema
// extractRequiredProperties extracts required property names from schema
func extractRequiredProperties(schema map[string]interface{}) []string {
	var required []string
	
	if reqInterface, exists := schema["required"]; exists {
		switch req := reqInterface.(type) {
		case []interface{}:
			for _, r := range req {
				if str, ok := r.(string); ok {
					required = append(required, str)
				}
			}
		case []string:
			required = req
		}
	}
	
	return required
}

// validateRequiredProperties checks if all required properties exist
func validateRequiredProperties(dataMap map[string]interface{}, required []string) []string {
	var errors []string
	for _, reqStr := range required {
		if _, exists := dataMap[reqStr]; !exists {
			errors = append(errors, fmt.Sprintf("required property '%s' missing", reqStr))
		}
	}
	return errors
}

// validateProperty validates a single property against its schema
func (sv *SchemaValidator) validateProperty(key string, value interface{}, propSchema interface{}) []string {
	var errors []string
	
	if propSchemaMap, ok := propSchema.(map[string]interface{}); ok {
		isValid, propErrors := sv.validateAgainstSchema(value, propSchemaMap)
		if !isValid {
			for _, err := range propErrors {
				errors = append(errors, fmt.Sprintf("property '%s': %s", key, err))
			}
		}
	}
	
	return errors
}

func (sv *SchemaValidator) validateObject(data interface{}, schema map[string]interface{}) []string {
	var errors []string

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return []string{"data is not an object"}
	}

	// Check required properties
	required := extractRequiredProperties(schema)
	errors = append(errors, validateRequiredProperties(dataMap, required)...)

	// Validate properties
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for key, value := range dataMap {
			if propSchema, exists := properties[key]; exists {
				errors = append(errors, sv.validateProperty(key, value, propSchema)...)
			} else {
				// Check if additional properties are allowed
				if additionalProps, ok := schema["additionalProperties"]; ok {
					if allow, ok := additionalProps.(bool); ok && !allow {
						errors = append(errors, fmt.Sprintf("additional property '%s' not allowed", key))
					}
				}
			}
		}
	}

	return errors
}

// checkArrayLengthConstraint checks array length against min/max constraints
func checkArrayLengthConstraint(length int, constraint interface{}, isMin bool) error {
	if val, ok := toFloat64(constraint); ok {
		intVal := int(val)
		if isMin && length < intVal {
			return fmt.Errorf("array length %d is less than minItems %d", length, intVal)
		} else if !isMin && length > intVal {
			return fmt.Errorf("array length %d exceeds maxItems %d", length, intVal)
		}
	}
	return nil
}

// validateArrayItems validates each item in the array
func (sv *SchemaValidator) validateArrayItems(dataArray reflect.Value, itemSchema map[string]interface{}) []string {
	var errors []string
	length := dataArray.Len()
	
	for i := 0; i < length; i++ {
		item := dataArray.Index(i).Interface()
		isValid, itemErrors := sv.validateAgainstSchema(item, itemSchema)
		if !isValid {
			for _, err := range itemErrors {
				errors = append(errors, fmt.Sprintf("item[%d]: %s", i, err))
			}
		}
	}
	
	return errors
}

// validateArray validates an array against array schema
func (sv *SchemaValidator) validateArray(data interface{}, schema map[string]interface{}) []string {
	var errors []string

	dataArray := reflect.ValueOf(data)
	if dataArray.Kind() != reflect.Slice && dataArray.Kind() != reflect.Array {
		return []string{"data is not an array"}
	}

	length := dataArray.Len()
	
	// Check array length constraints
	if minItems, exists := schema["minItems"]; exists {
		if err := checkArrayLengthConstraint(length, minItems, true); err != nil {
			errors = append(errors, err.Error())
		}
	}
	
	if maxItems, exists := schema["maxItems"]; exists {
		if err := checkArrayLengthConstraint(length, maxItems, false); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate items
	if itemSchema, ok := schema["items"].(map[string]interface{}); ok {
		errors = append(errors, sv.validateArrayItems(dataArray, itemSchema)...)
	}

	return errors
}

// validateString validates a string against string schema
func (sv *SchemaValidator) validateString(data interface{}, schema map[string]interface{}) []string {
	var errors []string

	str, ok := data.(string)
	if !ok {
		return []string{"data is not a string"}
	}

	// Check string length constraints
	if minLength, ok := schema["minLength"].(float64); ok {
		if len(str) < int(minLength) {
			errors = append(errors, fmt.Sprintf("string length %d is less than minLength %d", len(str), int(minLength)))
		}
	}
	if maxLength, ok := schema["maxLength"].(float64); ok {
		if len(str) > int(maxLength) {
			errors = append(errors, fmt.Sprintf("string length %d exceeds maxLength %d", len(str), int(maxLength)))
		}
	}

	// Check pattern (simplified - would use regex in full implementation)
	if pattern, ok := schema["pattern"].(string); ok {
		// In a full implementation, this would use regexp.MatchString
		if !strings.Contains(str, pattern) {
			errors = append(errors, fmt.Sprintf("string does not match pattern: %s", pattern))
		}
	}

	// Check enum values
	if enum, ok := schema["enum"].([]interface{}); ok {
		found := false
		for _, enumValue := range enum {
			if enumStr, ok := enumValue.(string); ok && enumStr == str {
				found = true
				break
			}
		}
		if !found {
			errors = append(errors, fmt.Sprintf("string value not in enum: %v", enum))
		}
	}

	return errors
}

// validateNumber validates a number against number schema
// toFloat64 converts numeric types to float64
func toFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	default:
		return 0, false
	}
}

// extractNumber converts data to float64 and determines if it's an integer
func extractNumber(data interface{}) (float64, bool, error) {
	switch v := data.(type) {
	case int:
		return float64(v), true, nil
	case int64:
		return float64(v), true, nil
	case float32:
		return float64(v), false, nil
	case float64:
		return v, false, nil
	default:
		return 0, false, fmt.Errorf("data is not a number")
	}
}

// checkNumericConstraint checks if a number violates min/max constraints
func checkNumericConstraint(num float64, constraint interface{}, isMin bool) (float64, error) {
	if val, ok := toFloat64(constraint); ok {
		if isMin && num < val {
			return val, fmt.Errorf("less than minimum")
		} else if !isMin && num > val {
			return val, fmt.Errorf("exceeds maximum")
		}
		return val, nil
	}
	return 0, fmt.Errorf("invalid constraint type")
}

func (sv *SchemaValidator) validateNumber(data interface{}, schema map[string]interface{}) []string {
	var errors []string

	num, isInt, err := extractNumber(data)
	if err != nil {
		return []string{err.Error()}
	}

	// Check minimum
	if minimum, exists := schema["minimum"]; exists {
		if minVal, err := checkNumericConstraint(num, minimum, true); err != nil && err.Error() == "less than minimum" {
			errors = append(errors, fmt.Sprintf("number %f is less than minimum %f", num, minVal))
		}
	}

	// Check maximum
	if maximum, exists := schema["maximum"]; exists {
		if maxVal, err := checkNumericConstraint(num, maximum, false); err != nil && err.Error() == "exceeds maximum" {
			errors = append(errors, fmt.Sprintf("number %f exceeds maximum %f", num, maxVal))
		}
	}

	// Check multipleOf
	if multipleOf, exists := schema["multipleOf"]; exists {
		if multVal, ok := toFloat64(multipleOf); ok && isInt {
			if int(num)%int(multVal) != 0 {
				errors = append(errors, fmt.Sprintf("number %f is not a multiple of %f", num, multVal))
			}
		}
	}

	return errors
}

// Type checking helper methods
func (sv *SchemaValidator) isObject(data interface{}) bool {
	_, ok := data.(map[string]interface{})
	return ok
}

func (sv *SchemaValidator) isArray(data interface{}) bool {
	v := reflect.ValueOf(data)
	return v.Kind() == reflect.Slice || v.Kind() == reflect.Array
}

func (sv *SchemaValidator) isString(data interface{}) bool {
	_, ok := data.(string)
	return ok
}

func (sv *SchemaValidator) isNumber(data interface{}) bool {
	switch data.(type) {
	case int, int64, float32, float64:
		return true
	default:
		return false
	}
}

func (sv *SchemaValidator) isBoolean(data interface{}) bool {
	_, ok := data.(bool)
	return ok
}

// StructuredGenerator provides structured generation capabilities
type StructuredGenerator struct {
	validator *SchemaValidator
}

// NewStructuredGenerator creates a new structured generator
func NewStructuredGenerator() *StructuredGenerator {
	return &StructuredGenerator{
		validator: NewSchemaValidator(),
	}
}

// ParseStructuredOutput parses model output according to specified format
func (sg *StructuredGenerator) ParseStructuredOutput(output string, format *ResponseFormat) (*StructuredOutput, error) {
	result := &StructuredOutput{
		Raw:      output,
		Format:   format,
		Valid:    false,
		Errors:   []string{},
		Metadata: make(map[string]interface{}),
	}

	if format == nil {
		// No format specified, return as-is
		result.Content = output
		result.Valid = true
		return result, nil
	}

	switch format.Type {
	case "json_object", "json_schema":
		return sg.parseJSONOutput(output, format, result)
	case "text":
		result.Content = output
		result.Valid = true
		return result, nil
	default:
		result.Errors = append(result.Errors, fmt.Sprintf("unsupported response format: %s", format.Type))
		return result, fmt.Errorf("unsupported response format: %s", format.Type)
	}
}

// parseJSONOutput parses JSON output and validates against schema
func (sg *StructuredGenerator) parseJSONOutput(output string, format *ResponseFormat, result *StructuredOutput) (*StructuredOutput, error) {
	// Extract JSON from output (handle markdown code blocks, etc.)
	jsonStr := sg.extractJSON(output)

	// Parse JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonData); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid JSON: %s", err.Error()))
		return result, fmt.Errorf("invalid JSON: %w", err)
	}

	result.Content = jsonData

	// Validate against schema if provided
	if format.JSONSchema != nil {
		sg.validator.RegisterSchema(format.JSONSchema)
		isValid, errors := sg.validator.ValidateJSON(jsonData, format.JSONSchema.Name)
		result.Valid = isValid
		result.Errors = append(result.Errors, errors...)
	} else if format.Schema != nil {
		isValid, errors := sg.validator.validateAgainstSchema(jsonData, format.Schema)
		result.Valid = isValid
		result.Errors = append(result.Errors, errors...)
	} else {
		// No schema provided, consider valid JSON as valid
		result.Valid = true
	}

	return result, nil
}

// extractJSON extracts JSON from potentially formatted text (e.g., markdown code blocks)
func (sg *StructuredGenerator) extractJSON(text string) string {
	// Remove markdown code blocks
	text = strings.TrimSpace(text)

	// Handle ```json code blocks
	if strings.HasPrefix(text, "```json") {
		lines := strings.Split(text, "\n")
		if len(lines) > 2 {
			// Remove first and last lines (```json and ```)
			jsonLines := lines[1 : len(lines)-1]
			return strings.Join(jsonLines, "\n")
		}
	}

	// Handle ``` code blocks
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) > 2 {
			// Remove first and last lines
			jsonLines := lines[1 : len(lines)-1]
			return strings.Join(jsonLines, "\n")
		}
	}

	// Look for JSON object patterns
	if strings.Contains(text, "{") && strings.Contains(text, "}") {
		start := strings.Index(text, "{")
		end := strings.LastIndex(text, "}") + 1
		if start < end {
			return text[start:end]
		}
	}

	// Look for JSON array patterns
	if strings.Contains(text, "[") && strings.Contains(text, "]") {
		start := strings.Index(text, "[")
		end := strings.LastIndex(text, "]") + 1
		if start < end {
			return text[start:end]
		}
	}

	return text
}

// GenerateStructuredPrompt generates a prompt that encourages structured output
func (sg *StructuredGenerator) GenerateStructuredPrompt(basePrompt string, format *ResponseFormat) string {
	if format == nil {
		return basePrompt
	}

	var structuredPrompt strings.Builder
	structuredPrompt.WriteString(basePrompt)
	structuredPrompt.WriteString("\n\n")

	switch format.Type {
	case "json_object":
		structuredPrompt.WriteString("Please respond with a valid JSON object.")
		if format.Description != "" {
			structuredPrompt.WriteString(" ")
			structuredPrompt.WriteString(format.Description)
		}

	case "json_schema":
		structuredPrompt.WriteString("Please respond with a JSON object that follows this exact schema:\n\n")
		if format.JSONSchema != nil {
			if schemaJSON, err := json.MarshalIndent(format.JSONSchema.Schema, "", "  "); err == nil {
				structuredPrompt.WriteString("```json\n")
				structuredPrompt.WriteString(string(schemaJSON))
				structuredPrompt.WriteString("\n```\n\n")
			}
			if format.JSONSchema.Description != "" {
				structuredPrompt.WriteString("Description: ")
				structuredPrompt.WriteString(format.JSONSchema.Description)
				structuredPrompt.WriteString("\n\n")
			}
			if format.Strict {
				structuredPrompt.WriteString("IMPORTANT: Your response must strictly follow this schema. Do not include any additional fields or deviate from the structure.")
			}
		}

	case "text":
		if format.Description != "" {
			structuredPrompt.WriteString("Response format: ")
			structuredPrompt.WriteString(format.Description)
		}
	}

	return structuredPrompt.String()
}

// CreateJSONSchema creates a JSON schema from a Go struct or map
func CreateJSONSchema(name, description string, example interface{}) *JSONSchema {
	schema := &JSONSchema{
		Name:        name,
		Description: description,
		Schema:      make(map[string]interface{}),
	}

	schema.Schema = generateSchemaFromValue(example)
	return schema
}

// generateSchemaFromValue generates a JSON schema from a Go value
func generateSchemaFromValue(value interface{}) map[string]interface{} {
	schema := make(map[string]interface{})

	switch v := value.(type) {
	case map[string]interface{}:
		schema["type"] = "object"
		properties := make(map[string]interface{})
		for key, val := range v {
			properties[key] = generateSchemaFromValue(val)
		}
		schema["properties"] = properties

	case []interface{}:
		schema["type"] = "array"
		if len(v) > 0 {
			schema["items"] = generateSchemaFromValue(v[0])
		}

	case string:
		schema["type"] = "string"

	case int, int64:
		schema["type"] = "integer"

	case float32, float64:
		schema["type"] = "number"

	case bool:
		schema["type"] = "boolean"

	default:
		// Use reflection for complex types
		t := reflect.TypeOf(value)
		if t != nil {
			switch t.Kind() {
			case reflect.Struct:
				schema["type"] = "object"
				properties := make(map[string]interface{})
				v := reflect.ValueOf(value)
				for i := 0; i < t.NumField(); i++ {
					field := t.Field(i)
					fieldValue := v.Field(i)
					if fieldValue.CanInterface() {
						properties[field.Name] = generateSchemaFromValue(fieldValue.Interface())
					}
				}
				schema["properties"] = properties

			case reflect.Slice, reflect.Array:
				schema["type"] = "array"
				if t.Elem() != nil {
					// Create a zero value of the element type for schema generation
					elemValue := reflect.Zero(t.Elem()).Interface()
					schema["items"] = generateSchemaFromValue(elemValue)
				}

			default:
				schema["type"] = "string" // Fallback
			}
		}
	}

	return schema
}

// Common response formats for convenience
var (
	JSONObjectFormat = &ResponseFormat{
		Type:        "json_object",
		Description: "A valid JSON object",
	}

	TextFormat = &ResponseFormat{
		Type:        "text",
		Description: "Plain text response",
	}
)

// CreateToolCallSchema creates a schema for tool calls
func CreateToolCallSchema() *JSONSchema {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tool_calls": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "The name of the tool to call",
						},
						"arguments": map[string]interface{}{
							"type":        "object",
							"description": "The arguments to pass to the tool",
						},
					},
					"required": []string{"name", "arguments"},
				},
			},
		},
		"required": []string{"tool_calls"},
	}

	return &JSONSchema{
		Name:        "tool_calls",
		Description: "Schema for tool calls",
		Schema:      schema,
		Strict:      true,
	}
}

// CreateFunctionCallSchema creates a schema for function calls
func CreateFunctionCallSchema() *JSONSchema {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"function_call": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "The name of the function to call",
					},
					"arguments": map[string]interface{}{
						"type":        "string",
						"description": "The arguments to pass to the function as a JSON string",
					},
				},
				"required": []string{"name", "arguments"},
			},
		},
		"required": []string{"function_call"},
	}

	return &JSONSchema{
		Name:        "function_call",
		Description: "Schema for function calls",
		Schema:      schema,
		Strict:      true,
	}
}

// Default structured generator instance
var DefaultStructuredGenerator = NewStructuredGenerator()

// Convenience functions using the default generator
func ParseStructuredOutput(output string, format *ResponseFormat) (*StructuredOutput, error) {
	return DefaultStructuredGenerator.ParseStructuredOutput(output, format)
}

func GenerateStructuredPrompt(basePrompt string, format *ResponseFormat) string {
	return DefaultStructuredGenerator.GenerateStructuredPrompt(basePrompt, format)
}

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

// Package models - Tests for structured generation functionality
package models

import (
	"testing"
)

func TestSchemaValidator(t *testing.T) {
	validator := NewSchemaValidator()

	// Test object schema
	objectSchema := &JSONSchema{
		Name: "test_object",
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":      "string",
					"minLength": 2,
				},
				"age": map[string]interface{}{
					"type":    "integer",
					"minimum": 0,
				},
			},
			"required": []string{"name"},
		},
	}

	validator.RegisterSchema(objectSchema)

	// Test valid data
	validData := map[string]interface{}{
		"name": "John",
		"age":  25,
	}

	valid, errors := validator.ValidateJSON(validData, "test_object")
	if !valid {
		t.Errorf("Expected valid data to pass validation, got errors: %v", errors)
	}

	// Test invalid data - missing required field
	invalidData := map[string]interface{}{
		"age": 25,
	}

	valid, _ = validator.ValidateJSON(invalidData, "test_object")
	if valid {
		t.Error("Expected invalid data to fail validation")
	}

	// Test invalid data - wrong type
	invalidTypeData := map[string]interface{}{
		"name": "John",
		"age":  "twenty-five", // Should be integer
	}

	valid, _ = validator.ValidateJSON(invalidTypeData, "test_object")
	if valid {
		t.Error("Expected invalid type data to fail validation")
	}
}

func TestArrayValidation(t *testing.T) {
	validator := NewSchemaValidator()

	arraySchema := &JSONSchema{
		Name: "test_array",
		Schema: map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "string",
			},
			"minItems": 1,
			"maxItems": 3,
		},
	}

	validator.RegisterSchema(arraySchema)

	// Test valid array
	validArray := []interface{}{"item1", "item2"}
	valid, errors := validator.ValidateJSON(validArray, "test_array")
	if !valid {
		t.Errorf("Expected valid array to pass validation, got errors: %v", errors)
	}

	// Test empty array (violates minItems)
	emptyArray := []interface{}{}
	valid, _ = validator.ValidateJSON(emptyArray, "test_array")
	if valid {
		t.Error("Expected empty array to fail validation (minItems)")
	}

	// Test too many items
	longArray := []interface{}{"item1", "item2", "item3", "item4"}
	valid, _ = validator.ValidateJSON(longArray, "test_array")
	if valid {
		t.Error("Expected long array to fail validation (maxItems)")
	}
}

func TestStringValidation(t *testing.T) {
	validator := NewSchemaValidator()

	stringSchema := &JSONSchema{
		Name: "test_string",
		Schema: map[string]interface{}{
			"type":      "string",
			"minLength": 3,
			"maxLength": 10,
			"enum":      []interface{}{"short", "medium", "long"},
		},
	}

	validator.RegisterSchema(stringSchema)

	// Test valid string
	valid, errors := validator.ValidateJSON("medium", "test_string")
	if !valid {
		t.Errorf("Expected valid string to pass validation, got errors: %v", errors)
	}

	// Test invalid enum value
	valid, _ = validator.ValidateJSON("extra-long", "test_string")
	if valid {
		t.Error("Expected string not in enum to fail validation")
	}

	// Test too short
	valid, _ = validator.ValidateJSON("hi", "test_string")
	if valid {
		t.Error("Expected short string to fail validation")
	}
}

func TestNumberValidation(t *testing.T) {
	validator := NewSchemaValidator()

	numberSchema := &JSONSchema{
		Name: "test_number",
		Schema: map[string]interface{}{
			"type":       "number",
			"minimum":    0,
			"maximum":    100,
			"multipleOf": 5,
		},
	}

	validator.RegisterSchema(numberSchema)

	// Test valid number
	valid, errors := validator.ValidateJSON(25, "test_number")
	if !valid {
		t.Errorf("Expected valid number to pass validation, got errors: %v", errors)
	}

	// Test number below minimum
	valid, _ = validator.ValidateJSON(-5, "test_number")
	if valid {
		t.Error("Expected number below minimum to fail validation")
	}

	// Test number above maximum
	valid, _ = validator.ValidateJSON(150, "test_number")
	if valid {
		t.Error("Expected number above maximum to fail validation")
	}

	// Test number not multiple of 5
	valid, _ = validator.ValidateJSON(23, "test_number")
	if valid {
		t.Error("Expected number not multiple of 5 to fail validation")
	}
}

func TestStructuredGenerator(t *testing.T) {
	generator := NewStructuredGenerator()

	// Test JSON object parsing
	jsonObjectFormat := &ResponseFormat{
		Type: "json_object",
	}

	jsonOutput := `{"name": "John", "age": 30, "city": "New York"}`

	result, err := generator.ParseStructuredOutput(jsonOutput, jsonObjectFormat)
	if err != nil {
		t.Errorf("Failed to parse JSON object: %v", err)
	}

	if !result.Valid {
		t.Error("Expected JSON object to be valid")
	}

	if result.Content == nil {
		t.Error("Expected parsed content to be non-nil")
	}
}

func TestJSONExtraction(t *testing.T) {
	generator := NewStructuredGenerator()

	// Test markdown code block extraction
	markdownInput := "```json\n{\"key\": \"value\"}\n```"
	extracted := generator.extractJSON(markdownInput)
	expected := "{\"key\": \"value\"}"

	if extracted != expected {
		t.Errorf("Expected %q, got %q", expected, extracted)
	}

	// Test plain JSON extraction
	jsonInput := `Here is the result: {"answer": 42} and that's it.`
	extracted = generator.extractJSON(jsonInput)
	expected = `{"answer": 42}`

	if extracted != expected {
		t.Errorf("Expected %q, got %q", expected, extracted)
	}
}

func TestResponseFormatConversion(t *testing.T) {
	baseModel := &BaseModel{}

	// Test JSON schema format conversion
	schema := &JSONSchema{
		Name:        "test_schema",
		Description: "A test schema",
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "string",
				},
			},
		},
		Strict: true,
	}

	format := &ResponseFormat{
		Type:       "json_schema",
		JSONSchema: schema,
		Strict:     true,
	}

	converted := baseModel.ConvertResponseFormat(format)

	if converted["type"] != "json_schema" {
		t.Errorf("Expected type json_schema, got %v", converted["type"])
	}

	jsonSchema, ok := converted["json_schema"].(map[string]interface{})
	if !ok {
		t.Error("Expected json_schema field to be a map")
	}

	if jsonSchema["name"] != "test_schema" {
		t.Errorf("Expected schema name test_schema, got %v", jsonSchema["name"])
	}

	if jsonSchema["strict"] != true {
		t.Errorf("Expected strict to be true, got %v", jsonSchema["strict"])
	}
}

func TestCreateJSONSchema(t *testing.T) {
	// Test schema creation from struct
	type TestStruct struct {
		Name   string   `json:"name"`
		Age    int      `json:"age"`
		Tags   []string `json:"tags"`
		Active bool     `json:"active"`
	}

	example := TestStruct{
		Name:   "test",
		Age:    25,
		Tags:   []string{"tag1"},
		Active: true,
	}

	schema := CreateJSONSchema("test_struct", "A test structure", example)

	if schema.Name != "test_struct" {
		t.Errorf("Expected schema name test_struct, got %s", schema.Name)
	}

	if schema.Description != "A test structure" {
		t.Errorf("Expected description 'A test structure', got %s", schema.Description)
	}

	properties, ok := schema.Schema["properties"].(map[string]interface{})
	if !ok {
		t.Error("Expected schema to have properties")
	}

	// Check that properties were generated
	expectedProps := []string{"Name", "Age", "Tags", "Active"}
	for _, prop := range expectedProps {
		if _, exists := properties[prop]; !exists {
			t.Errorf("Expected property %s to exist in schema", prop)
		}
	}
}

func TestToolCallSchema(t *testing.T) {
	schema := CreateToolCallSchema()

	if schema.Name != "tool_calls" {
		t.Errorf("Expected schema name tool_calls, got %s", schema.Name)
	}

	if !schema.Strict {
		t.Error("Expected tool call schema to be strict")
	}

	// Validate the schema structure
	validator := NewSchemaValidator()
	validator.RegisterSchema(schema)

	// Test valid tool call data
	validToolCall := map[string]interface{}{
		"tool_calls": []interface{}{
			map[string]interface{}{
				"name": "calculator",
				"arguments": map[string]interface{}{
					"expression": "2 + 2",
				},
			},
		},
	}

	valid, errors := validator.ValidateJSON(validToolCall, "tool_calls")
	if !valid {
		t.Errorf("Expected valid tool call to pass validation, got errors: %v", errors)
	}
}

func TestFunctionCallSchema(t *testing.T) {
	schema := CreateFunctionCallSchema()

	if schema.Name != "function_call" {
		t.Errorf("Expected schema name function_call, got %s", schema.Name)
	}

	// Validate the schema structure
	validator := NewSchemaValidator()
	validator.RegisterSchema(schema)

	// Test valid function call data
	validFunctionCall := map[string]interface{}{
		"function_call": map[string]interface{}{
			"name":      "get_weather",
			"arguments": `{"location": "San Francisco"}`,
		},
	}

	valid, errors := validator.ValidateJSON(validFunctionCall, "function_call")
	if !valid {
		t.Errorf("Expected valid function call to pass validation, got errors: %v", errors)
	}
}

func TestStructuredPromptGeneration(t *testing.T) {
	generator := NewStructuredGenerator()

	basePrompt := "Analyze the following data"

	// Test JSON object format
	jsonFormat := &ResponseFormat{
		Type:        "json_object",
		Description: "Return results as a JSON object",
	}

	structuredPrompt := generator.GenerateStructuredPrompt(basePrompt, jsonFormat)

	if !contains(structuredPrompt, basePrompt) {
		t.Error("Expected structured prompt to contain base prompt")
	}

	if !contains(structuredPrompt, "JSON object") {
		t.Error("Expected structured prompt to mention JSON object")
	}

	// Test JSON schema format
	schema := &JSONSchema{
		Name:        "analysis",
		Description: "Analysis results schema",
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"summary": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}

	schemaFormat := &ResponseFormat{
		Type:       "json_schema",
		JSONSchema: schema,
		Strict:     true,
	}

	structuredPrompt = generator.GenerateStructuredPrompt(basePrompt, schemaFormat)

	if !contains(structuredPrompt, "schema") {
		t.Error("Expected structured prompt to mention schema")
	}

	if !contains(structuredPrompt, "strictly follow") {
		t.Error("Expected structured prompt to mention strict compliance")
	}
}

func TestGenerateSchemaFromValue(t *testing.T) {
	// Test map generation
	testMap := map[string]interface{}{
		"name":   "test",
		"count":  42,
		"active": true,
		"tags":   []interface{}{"tag1", "tag2"},
	}

	schema := generateSchemaFromValue(testMap)

	if schema["type"] != "object" {
		t.Errorf("Expected type object, got %v", schema["type"])
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Error("Expected properties to be a map")
	}

	// Check individual property types
	if nameSchema, ok := properties["name"].(map[string]interface{}); ok {
		if nameSchema["type"] != "string" {
			t.Errorf("Expected name type to be string, got %v", nameSchema["type"])
		}
	} else {
		t.Error("Expected name property to exist")
	}

	if countSchema, ok := properties["count"].(map[string]interface{}); ok {
		if countSchema["type"] != "integer" {
			t.Errorf("Expected count type to be integer, got %v", countSchema["type"])
		}
	} else {
		t.Error("Expected count property to exist")
	}

	if activeSchema, ok := properties["active"].(map[string]interface{}); ok {
		if activeSchema["type"] != "boolean" {
			t.Errorf("Expected active type to be boolean, got %v", activeSchema["type"])
		}
	} else {
		t.Error("Expected active property to exist")
	}

	if tagsSchema, ok := properties["tags"].(map[string]interface{}); ok {
		if tagsSchema["type"] != "array" {
			t.Errorf("Expected tags type to be array, got %v", tagsSchema["type"])
		}
	} else {
		t.Error("Expected tags property to exist")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(len(substr) == 0 ||
			s[:len(substr)] == substr ||
			(len(s) > len(substr) && contains(s[1:], substr)))
}

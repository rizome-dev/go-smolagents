// Package main demonstrates structured generation capabilities in smolagents-go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/rizome-dev/smolagentsgo/models"
)

// Task represents a structured task format
type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Tags        []string `json:"tags"`
	Completed   bool     `json:"completed"`
}

// AnalysisResult represents a structured analysis output
type AnalysisResult struct {
	Summary    string                 `json:"summary"`
	KeyPoints  []string               `json:"key_points"`
	Confidence float64                `json:"confidence"`
	Metadata   map[string]interface{} `json:"metadata"`
}

func main() {
	// Get API token from environment
	apiToken := os.Getenv("HF_API_TOKEN")
	if apiToken == "" {
		log.Fatal("HF_API_TOKEN environment variable not set")
	}

	// Create a model with structured generation support
	model := models.NewInferenceClientModel("microsoft/DialoGPT-medium", apiToken, map[string]interface{}{
		"api_base": "https://api-inference.huggingface.co/models",
	})

	fmt.Println("=== Structured Generation Examples ===\n")

	// Example 1: JSON Object Response
	fmt.Println("1. JSON Object Generation:")
	demonstrateJSONObject(model)

	// Example 2: JSON Schema Validation
	fmt.Println("\n2. JSON Schema Validation:")
	demonstrateJSONSchema(model)

	// Example 3: Task Creation with Validation
	fmt.Println("\n3. Task Creation with Validation:")
	demonstrateTaskCreation(model)

	// Example 4: Analysis with Retry on Failure
	fmt.Println("\n4. Analysis with Retry on Failure:")
	demonstrateAnalysisWithRetry(model)

	// Example 5: Multimodal Content
	fmt.Println("\n5. Multimodal Content Processing:")
	demonstrateMultimodalContent()
}

func demonstrateJSONObject(model models.Model) {
	// Create a simple JSON object response format
	format := &models.ResponseFormat{
		Type:        "json_object",
		Description: "Respond with a JSON object containing user information",
	}

	// Create options with structured format
	options := &models.GenerateOptions{
		ResponseFormat: format,
		MaxTokens:      intPtr(500),
		Temperature:    floatPtr(0.7),
	}

	// Generate structured prompt
	baseModel := &models.BaseModel{}
	prompt := baseModel.PrepareStructuredPrompt(
		"Create a user profile for a software developer named John who likes hiking and coding.",
		format,
	)

	// Prepare messages
	messages := []interface{}{
		map[string]interface{}{
			"role":    "user",
			"content": prompt,
		},
	}

	// Generate response with structured output
	structuredOutput, err := baseModel.GenerateWithStructuredOutput(model, messages, options)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Generated JSON: %s\n", structuredOutput.Raw)
	fmt.Printf("Valid: %t\n", structuredOutput.Valid)
	if len(structuredOutput.Errors) > 0 {
		fmt.Printf("Errors: %v\n", structuredOutput.Errors)
	}
}

func demonstrateJSONSchema(model models.Model) {
	// Create a JSON schema for task creation
	taskSchema := models.CreateJSONSchema(
		"task_creation",
		"Schema for creating a new task",
		Task{
			ID:          "string",
			Title:       "string",
			Description: "string",
			Priority:    "string",
			Tags:        []string{},
			Completed:   false,
		},
	)

	// Enhance the schema with validation rules
	taskSchema.Schema["properties"].(map[string]interface{})["priority"].(map[string]interface{})["enum"] = []string{"low", "medium", "high", "urgent"}
	taskSchema.Schema["required"] = []string{"id", "title", "description", "priority"}

	format := &models.ResponseFormat{
		Type:       "json_schema",
		JSONSchema: taskSchema,
		Strict:     true,
	}

	options := &models.GenerateOptions{
		ResponseFormat: format,
		ValidateOutput: true,
		RetryOnFailure: true,
		MaxRetries:     2,
		MaxTokens:      intPtr(300),
	}

	baseModel := &models.BaseModel{}
	prompt := baseModel.PrepareStructuredPrompt(
		"Create a task for implementing user authentication with high priority.",
		format,
	)

	messages := []interface{}{
		map[string]interface{}{
			"role":    "user",
			"content": prompt,
		},
	}

	structuredOutput, err := baseModel.GenerateWithStructuredOutput(model, messages, options)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Generated Task JSON: %s\n", structuredOutput.Raw)
	fmt.Printf("Schema Valid: %t\n", structuredOutput.Valid)

	// Parse into Go struct
	var task Task
	if taskData, ok := structuredOutput.Content.(map[string]interface{}); ok {
		jsonBytes, _ := json.Marshal(taskData)
		json.Unmarshal(jsonBytes, &task)
		fmt.Printf("Parsed Task: ID=%s, Title=%s, Priority=%s\n", task.ID, task.Title, task.Priority)
	}
}

func demonstrateTaskCreation(model models.Model) {
	// Create validator and register schema
	validator := models.NewSchemaValidator()

	taskSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":    "string",
				"pattern": "^task-[0-9]+$",
			},
			"title": map[string]interface{}{
				"type":      "string",
				"minLength": 5,
				"maxLength": 100,
			},
			"priority": map[string]interface{}{
				"type": "string",
				"enum": []string{"low", "medium", "high", "urgent"},
			},
		},
		"required": []string{"id", "title", "priority"},
	}

	schema := &models.JSONSchema{
		Name:   "validated_task",
		Schema: taskSchema,
		Strict: true,
	}

	validator.RegisterSchema(schema)

	// Test validation with sample data
	testData := map[string]interface{}{
		"id":       "task-123",
		"title":    "Implement user authentication system",
		"priority": "high",
	}

	valid, errors := validator.ValidateJSON(testData, "validated_task")
	fmt.Printf("Test Data Valid: %t\n", valid)
	if !valid {
		fmt.Printf("Validation Errors: %v\n", errors)
	} else {
		fmt.Printf("Successfully validated task: %v\n", testData)
	}
}

func demonstrateAnalysisWithRetry(model models.Model) {
	// Create analysis schema
	analysisSchema := models.CreateJSONSchema(
		"analysis_result",
		"Schema for analysis results",
		AnalysisResult{
			Summary:    "string",
			KeyPoints:  []string{},
			Confidence: 0.0,
			Metadata:   map[string]interface{}{},
		},
	)

	// Add validation constraints
	if props, ok := analysisSchema.Schema["properties"].(map[string]interface{}); ok {
		if conf, ok := props["confidence"].(map[string]interface{}); ok {
			conf["minimum"] = 0.0
			conf["maximum"] = 1.0
		}
		if keyPoints, ok := props["key_points"].(map[string]interface{}); ok {
			keyPoints["minItems"] = 1
			keyPoints["maxItems"] = 10
		}
	}
	analysisSchema.Schema["required"] = []string{"summary", "key_points", "confidence"}

	format := &models.ResponseFormat{
		Type:       "json_schema",
		JSONSchema: analysisSchema,
		Strict:     true,
	}

	options := &models.GenerateOptions{
		ResponseFormat: format,
		ValidateOutput: true,
		RetryOnFailure: true,
		MaxRetries:     3,
		MaxTokens:      intPtr(400),
		Temperature:    floatPtr(0.3), // Lower temperature for more structured output
	}

	baseModel := &models.BaseModel{}
	prompt := baseModel.PrepareStructuredPrompt(
		"Analyze the impact of artificial intelligence on software development. Provide a structured analysis.",
		format,
	)

	messages := []interface{}{
		map[string]interface{}{
			"role":    "user",
			"content": prompt,
		},
	}

	structuredOutput, err := baseModel.GenerateWithStructuredOutput(model, messages, options)
	if err != nil {
		fmt.Printf("Error after retries: %v\n", err)
		return
	}

	fmt.Printf("Analysis Result: %s\n", structuredOutput.Raw)
	fmt.Printf("Validation Status: %t\n", structuredOutput.Valid)
	fmt.Printf("Attempts: %v\n", structuredOutput.Metadata["attempt"])
}

func demonstrateMultimodalContent() {
	// Create multimodal support
	multimodal := models.NewMultimodalSupport()

	// Create text content
	textContent := multimodal.CreateTextContent("Analyze this image for objects and describe what you see.")

	// Create a multimodal message (simulating image analysis)
	message := multimodal.CreateMultimodalMessage(
		"user",
		textContent,
		// In a real scenario, you would load an image:
		// imageContent, err := multimodal.LoadImageFromFile("/path/to/image.jpg")
	)

	// Convert to standard format
	standardMessages := multimodal.ConvertToStandardFormat([]*models.MultimodalMessage{message})

	fmt.Printf("Multimodal message created with %d content items\n", len(message.Content))
	fmt.Printf("Standard format: %v\n", standardMessages[0])

	// Validate the message
	if err := multimodal.ValidateMessage(message); err != nil {
		fmt.Printf("Validation error: %v\n", err)
	} else {
		fmt.Printf("Message is valid\n")
	}

	// Get media count
	mediaCounts := multimodal.GetMediaCount(message)
	fmt.Printf("Media counts: %v\n", mediaCounts)

	// Extract text from multimodal message
	extractedText := multimodal.ExtractText(message)
	fmt.Printf("Extracted text: %s\n", extractedText)
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}

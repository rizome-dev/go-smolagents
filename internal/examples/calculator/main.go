// Package main demonstrates a simple calculator agent using the smolagents library
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rizome-dev/go-smolagents/pkg/agents"
	"github.com/rizome-dev/go-smolagents/pkg/default_tools"
	"github.com/rizome-dev/go-smolagents/pkg/models"
	"github.com/rizome-dev/go-smolagents/pkg/tools"
)

func main() {
	fmt.Println("🧮 Calculator Agent Example")
	fmt.Println("===========================")

	// Check for required environment variables
	if os.Getenv("HF_API_TOKEN") == "" && os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("Please set HF_API_TOKEN or OPENAI_API_KEY environment variable")
	}

	// Create model
	var model models.Model
	var err error

	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		model, err = models.CreateModel(models.ModelTypeOpenAIServer, "gpt-3.5-turbo", map[string]interface{}{
			"api_key":     apiKey,
			"temperature": 0.1, // Low temperature for accurate calculations
		})
	} else {
		model, err = models.CreateModel(models.ModelTypeInferenceClient, "meta-llama/Llama-2-7b-chat-hf", map[string]interface{}{
			"api_token":   os.Getenv("HF_API_TOKEN"),
			"temperature": 0.1,
		})
	}

	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Create tools for mathematical operations
	agentTools := []tools.Tool{
		default_tools.NewGoInterpreterTool(), // For complex calculations
		default_tools.NewFinalAnswerTool(),   // For providing the final result
	}

	// Create a tool-calling agent
	agent, err := agents.NewToolCallingAgent(model, agentTools, "system", map[string]interface{}{
		"max_steps": 5,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Test calculations
	calculations := []string{
		"Calculate the area of a circle with radius 5",
		"What is the factorial of 10?",
		"Solve the quadratic equation x^2 - 5x + 6 = 0",
		"Calculate the compound interest for $1000 at 5% annually for 3 years",
		"What is the derivative of 3x^2 + 2x + 1?",
	}

	for i, calculation := range calculations {
		fmt.Printf("\n%d. %s\n", i+1, calculation)
		fmt.Println(strings.Repeat("-", len(calculation)+3))

		maxSteps := 5
		result, err := agent.Run(&agents.RunOptions{
			Task:     calculation,
			MaxSteps: &maxSteps,
		})

		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		fmt.Printf("✅ Result: %v\n", result.Output)
	}

	fmt.Println("\n🎉 Calculator agent demonstration completed!")
}

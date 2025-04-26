package main

import (
	"fmt"
	"log"
	"os"

	"github.com/rizome-dev/smolagentsgo/agents"
	"github.com/rizome-dev/smolagentsgo/models"
	"github.com/rizome-dev/smolagentsgo/tools"
)

func main() {
	// Check for HuggingFace API token
	hfToken := os.Getenv("HF_API_TOKEN")
	if hfToken == "" {
		log.Fatal("Please set the HF_API_TOKEN environment variable")
	}

	// Create a simple calculator tool
	calculatorTool, err := tools.NewBaseTool(
		"calculator",
		"Performs basic arithmetic operations",
		map[string]tools.InputProperty{
			"operation": {
				Type:        "string",
				Description: "The operation to perform (add, subtract, multiply, divide)",
			},
			"a": {
				Type:        "number",
				Description: "First number",
			},
			"b": {
				Type:        "number",
				Description: "Second number",
			},
		},
		"number",
		func(args map[string]interface{}) (interface{}, error) {
			op := args["operation"].(string)
			a := args["a"].(float64)
			b := args["b"].(float64)

			switch op {
			case "add":
				return a + b, nil
			case "subtract":
				return a - b, nil
			case "multiply":
				return a * b, nil
			case "divide":
				if b == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				return a / b, nil
			default:
				return nil, fmt.Errorf("unknown operation: %s", op)
			}
		},
	)
	if err != nil {
		log.Fatalf("Failed to create calculator tool: %v", err)
	}

	// Create a web search tool
	searchTool, err := tools.NewDuckDuckGoSearchTool()
	if err != nil {
		log.Fatalf("Failed to create web search tool: %v", err)
	}

	// Create a HuggingFace model client
	hfModel, err := models.NewHuggingFaceModel(
		"mistralai/Mixtral-8x7B-Instruct-v0.1", // Use a capable HF model
		hfToken,
		false, // No streaming
		nil,   // Default model parameters
	)
	if err != nil {
		log.Fatalf("Failed to create HuggingFace model: %v", err)
	}

	// Define the model function that will use the HuggingFace model
	modelFunc := func(messages []models.Message, stopSequences []string) (*models.ChatMessage, error) {
		return hfModel.Call(
			models.GetCleanMessageList(messages),
			stopSequences,
			"",  // No specific grammar
			nil, // No tools to call from
			nil, // No additional arguments
		)
	}

	// Set up prompt templates
	promptTemplates := agents.EmptyPromptTemplates()
	promptTemplates.SystemPrompt = "You are a helpful assistant that can solve math problems and search the web for information. Use the calculator tool to perform arithmetic operations and the web_search tool to find information online when needed."

	// Create a tool-calling agent with both tools
	agent, err := agents.NewToolCallingAgent(
		[]tools.Tool{calculatorTool, searchTool},
		modelFunc,
		promptTemplates,
		0,   // Planning interval (0 means no planning)
		10,  // Max 10 steps
		nil, // Default grammar
		nil, // No managed agents
		nil, // No callbacks
		"research_calculator_agent",
		"An agent that can search the web and perform calculations",
		false, // No run summary
		nil,   // No final answer checks
	)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Run the agent with a task that requires both search and calculation
	fmt.Println("Running the research calculator agent...")
	fmt.Println("Task: What is the population of Tokyo? Then calculate what would be 0.1% of that number.")

	result, err := agent.Run(
		"What is the population of Tokyo? Then calculate what would be 0.1% of that number.",
		false, // No streaming
		true,  // Reset memory
		nil,   // No images
		nil,   // No additional args
		0,     // Use default max steps
	)
	if err != nil {
		log.Fatalf("Error running agent: %v", err)
	}

	fmt.Printf("Result: %v\n", result)

	// Run another example that combines web search and calculation
	fmt.Println("\nRunning another example...")
	fmt.Println("Task: What is the distance between Earth and Mars at their closest approach? Convert that to kilometers and then calculate how long it would take to travel that distance at 50,000 km/h.")

	result, err = agent.Run(
		"What is the distance between Earth and Mars at their closest approach? Convert that to kilometers and then calculate how long it would take to travel that distance at 50,000 km/h.",
		false, // No streaming
		true,  // Reset memory
		nil,   // No images
		nil,   // No additional args
		0,     // Use default max steps
	)
	if err != nil {
		log.Fatalf("Error running agent: %v", err)
	}

	fmt.Printf("Result: %v\n", result)
}

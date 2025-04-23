# smolagentsgo

[![GoDoc](https://pkg.go.dev/badge/github.com/rizome-dev/smolagentsgo)](https://pkg.go.dev/github.com/rizome-dev/smolagentsgo)
[![Go Report Card](https://goreportcard.com/badge/github.com/rizome-dev/smolagentsgo)](https://goreportcard.com/report/github.com/rizome-dev/smolagentsgo)

```shell
go get github.com/rizome-dev/smolagentsgo
```

built by: [rizome labs](https://rizome.dev)

contact us: [hi (at) rizome.dev](mailto:hi@rizome.dev)

## Example

```go
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rizome-dev/smolagentsgo"
	"github.com/rizome-dev/smolagentsgo/models"
)

// Simple model function that returns a fixed response
func simpleModel(messages []models.Message, stopSequences []string) (*models.ChatMessage, error) {
	// For demo purposes - pretends to be the LLM and returns a response
	// In real usage, this would call an actual LLM API

	// Simulate a model that responds with a final answer
	response := "Thought: I'll use the calculator tool to compute 2 + 2\nAction: final_answer(answer=\"The result of 2 + 2 is 4\")"

	// Check if we should stop generation at a specific sequence
	if stopSequences != nil && len(stopSequences) > 0 {
		for _, seq := range stopSequences {
			if strings.Contains(response, seq) {
				response = strings.Split(response, seq)[0]
			}
		}
	}

	return &models.ChatMessage{
		Role:    "assistant",
		Content: response,
	}, nil
}

// Implementation of a simple calculator function
func calculateExpression(args map[string]interface{}) (interface{}, error) {
	expr, ok := args["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("expression must be a string")
	}

	// Very simple calculator that can only handle addition
	parts := strings.Split(expr, "+")
	if len(parts) != 2 {
		return nil, fmt.Errorf("only addition is supported in format: a + b")
	}

	// Parse the numbers
	a, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid first number: %w", err)
	}

	b, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid second number: %w", err)
	}

	result := a + b
	return fmt.Sprintf("%.2f", result), nil
}

func main() {
	calculator := smolagentsgo.NewBaseTool(
		"calculator",
		"A simple calculator that can add, subtract, multiply, and divide",
		map[string]smolagentsgo.InputProperty{
			"expression": {
				Type:        "string",
				Description: "The mathematical expression to evaluate",
			},
		},
		"string",
		calculateExpression,
	)

	agent, err := smolagentsgo.NewToolCallingAgent(
		[]smolagentsgo.Tool{calculator},
		simpleModel, // Provide a model function
		smolagentsgo.EmptyPromptTemplates(),
		0,
		20,
		nil,
		nil,
		nil,
		"math_agent",
		"An agent that can solve math problems",
		false,
		nil,
	)
	if err != nil {
		fmt.Printf("Error creating agent: %v\n", err)
		return
	}

	result, err := agent.Run("Calculate 2 + 2", false, true, nil, nil, 0)
	if err != nil {
		fmt.Printf("Error running agent: %v\n", err)
		return
	}

	fmt.Printf("Result: %v\n", result)
}
```
 
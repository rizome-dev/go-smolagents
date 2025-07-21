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

// Example of using the ReactCodeAgent with full ReAct implementation
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/agents"
	"github.com/rizome-dev/go-smolagents/pkg/models"
)

func main() {
	// Get HuggingFace API token from environment
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		log.Fatal("Please set HF_TOKEN environment variable")
	}

	// Create model with explicit provider
	model := models.NewInferenceClientModel(
		"moonshotai/Kimi-K2-Instruct",
		token,
		map[string]interface{}{
		},
	)

	// Create ReactCodeAgent with default options
	agent, err := agents.NewReactCodeAgent(model, nil, "", nil)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	// Example tasks to demonstrate the agent's capabilities
	tasks := []string{
		"Calculate 2 + 2 and print the result",
		"Calculate the factorial of 5",
	}

	for i, task := range tasks {
		if i > 0 {
			fmt.Println() // Add spacing between tasks
		}

		// Run the agent with a longer timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := agent.Run(&agents.RunOptions{
			Task:    task,
			Context: ctx,
			Reset:   i > 0, // Reset for all tasks after the first
		})
		cancel()

		if err != nil {
			fmt.Printf("\nError: %v\n", err)
			if result != nil {
				fmt.Printf("Status: %s\n", result.State)
				fmt.Printf("Steps taken: %d\n", result.StepCount)
			}
			continue
		}

		// Results are displayed by the agent itself
		if i < len(tasks)-1 {
			fmt.Println() // Add spacing between tasks
		}
	}

	// Example with custom options
	fmt.Println()

	// Create agent with custom options
	customOptions := &agents.ReactCodeAgentOptions{
		AuthorizedPackages: []string{"fmt", "math", "strings", "time"},
		CodeBlockTags:      [2]string{"```go", "```"},
		MaxCodeLength:      100000, // Support up to ~1000 lines of code
		MaxSteps:           20,
		EnablePlanning:     true,
		PlanningInterval:   5,
		Verbose:            true,
	}

	// Use the same model instance
	customAgent, err := agents.NewReactCodeAgent(model, nil, "", customOptions)
	if err != nil {
		log.Fatalf("Failed to create custom agent: %v", err)
	}
	defer customAgent.Close()

	// Complex task that benefits from planning
	complexTask := "Write a Go function to check if a number is prime and test it with the number 17"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	result, err := customAgent.Run(&agents.RunOptions{
		Task:    complexTask,
		Context: ctx,
	})
	cancel()

	if err != nil {
		fmt.Printf("\nError: %v\n", err)
		if result != nil {
			fmt.Printf("Status: %s\n", result.State)
			fmt.Printf("Steps taken: %d\n", result.StepCount)
		}
	}
}

# smolagentsgo

[![GoDoc](https://pkg.go.dev/badge/github.com/rizome-dev/smolagentsgo)](https://pkg.go.dev/github.com/rizome-dev/smolagentsgo)
[![Go Report Card](https://goreportcard.com/badge/github.com/rizome-dev/smolagentsgo)](https://goreportcard.com/report/github.com/rizome-dev/smolagentsgo)

```shell
go get github.com/rizome-dev/smolagentsgo
```

built by: [rizome labs](https://rizome.dev)

contact us: [hi (at) rizome.dev](mailto:hi@rizome.dev)

## Examples

- [Calculator](examples/calculator/main.go)
- [Web Search](examples/web_search/main.go)

## Creating a Go Code Agent

```go
package main

import (
	"fmt"
	"log"

	"github.com/rizome-dev/smolagentsgo"
	"github.com/rizome-dev/smolagentsgo/agents"
	"github.com/rizome-dev/smolagentsgo/models"
)

func main() {
	// Create a Go code executor
	goExecutor, err := agents.NewGoSandboxExecutor()
	if err != nil {
		log.Fatalf("Failed to create Go executor: %v", err)
	}
	defer goExecutor.Cleanup()

	// Define the model function (placeholder)
	modelFunc := func(messages []models.Message, stopSequences []string) (*models.ChatMessage, error) {
		// In a real implementation, this would call an actual LLM
		return &models.ChatMessage{
			Role:    models.RoleAssistant,
			Content: "Thought: I need to calculate the factorial of 5.\nCode:\n```go\nfunc factorial(n int) int {\n  if n <= 1 {\n    return 1\n  }\n  return n * factorial(n-1)\n}\n\nfmt.Printf(\"Factorial of 5 is %d\\n\", factorial(5))\n```<end_code>",
		}, nil
	}

	// Create a code agent
	agent, err := smolagentsgo.NewCodeAgent(
		nil,                                // No extra tools
		modelFunc,
		smolagentsgo.EmptyPromptTemplates(), // Use default prompt templates
		0,                                   // No planning
		10,                                  // Max 10 steps
		nil,                                 // Default grammar
		nil,                                 // No managed agents
		nil,                                 // No callbacks
		"code_agent",                       // Agent name
		"A Go code agent",                  // Description
		false,                              // No run summary
		nil,                                // No final answer checks
		goExecutor,                         // Go executor
	)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Run the agent
	result, err := agent.Run("Calculate the factorial of 5", false, true, nil, nil, 0)
	if err != nil {
		log.Fatalf("Error running agent: %v", err)
	}

	fmt.Printf("Result: %v\n", result)
}
```

## Multi-Agent Systems

```go
package main

import (
	"fmt"
	"log"

	"github.com/rizome-dev/smolagentsgo"
	"github.com/rizome-dev/smolagentsgo/agents"
	"github.com/rizome-dev/smolagentsgo/models"
	"github.com/rizome-dev/smolagentsgo/tools"
)

func main() {
	// Define the model function (placeholder)
	modelFunc := func(messages []models.Message, stopSequences []string) (*models.ChatMessage, error) {
		// Simplified for example
		return &models.ChatMessage{
			Role:    models.RoleAssistant,
			Content: "Thought: I'll delegate to the math_agent.\nAction: math_agent(task=\"Calculate 123 * 456\")",
		}, nil
	}

	// Create a specialized math agent
	mathAgent, err := smolagentsgo.NewToolCallingAgent(
		[]tools.Tool{}, // Add math tools here
		modelFunc,
		smolagentsgo.EmptyPromptTemplates(),
		0, 10, nil, nil, nil,
		"math_solver",
		"Solves math problems",
		false, nil,
	)
	if err != nil {
		log.Fatalf("Failed to create math agent: %v", err)
	}

	// Create a managed agent wrapper
	managedMathAgent, err := smolagentsgo.NewManagedAgent(
		mathAgent,
		"math_agent",
		"A specialized agent for solving mathematical problems",
	)
	if err != nil {
		log.Fatalf("Failed to create managed agent: %v", err)
	}

	// Create the manager agent
	managerAgent, err := smolagentsgo.NewToolCallingAgent(
		[]tools.Tool{},
		modelFunc,
		smolagentsgo.EmptyPromptTemplates(),
		0, 10, nil,
		[]agents.ManagedAgent{managedMathAgent},
		nil,
		"manager_agent",
		"Coordinates other agents",
		false, nil,
	)
	if err != nil {
		log.Fatalf("Failed to create manager agent: %v", err)
	}

	// Run the manager agent
	result, err := managerAgent.Run("What is 123 * 456?", false, true, nil, nil, 0)
	if err != nil {
		log.Fatalf("Error running agent: %v", err)
	}

	fmt.Printf("Result: %v\n", result)
}
```

## Concurrency with Go

```go
package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/rizome-dev/smolagentsgo"
	// Import other necessary packages
)

func main() {
	// Create multiple agents
	// (agent setup code omitted for brevity)

	var wg sync.WaitGroup
	results := make(chan interface{}, 3)
	errors := make(chan error, 3)

	// Run multiple agents concurrently
	for i, agent := range []smolagentsgo.MultiStepAgent{agent1, agent2, agent3} {
		wg.Add(1)
		go func(i int, a smolagentsgo.MultiStepAgent) {
			defer wg.Done()
			
			// Each agent works on a different task
			task := fmt.Sprintf("Task for agent %d", i)
			result, err := a.Run(task, false, true, nil, nil, 0)
			
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i, agent)
	}

	// Wait for all agents to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All agents completed successfully
		close(results)
		close(errors)
		
		// Process results
		for result := range results {
			fmt.Printf("Result: %v\n", result)
		}
		
		// Check for errors
		for err := range errors {
			fmt.Printf("Error: %v\n", err)
		}
		
	case <-time.After(5 * time.Minute):
		fmt.Println("Timeout: Some agents took too long")
	}
}
```

// Package main demonstrates a web search agent using the smolagents library
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
	fmt.Println("🔍 Web Search Agent Example")
	fmt.Println("============================")

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
			"temperature": 0.3,
		})
	} else {
		model, err = models.CreateModel(models.ModelTypeInferenceClient, "meta-llama/Llama-2-7b-chat-hf", map[string]interface{}{
			"api_token":   os.Getenv("HF_API_TOKEN"),
			"temperature": 0.3,
		})
	}

	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Create tools for web search and research
	agentTools := []tools.Tool{
		default_tools.NewWebSearchTool(),        // Primary web search
		default_tools.NewDuckDuckGoSearchTool(), // Alternative search
		default_tools.NewWikipediaSearchTool(),  // Knowledge base
		default_tools.NewVisitWebpageTool(),     // Deep content extraction
		default_tools.NewFinalAnswerTool(),      // Final response
	}

	// Create a tool-calling agent
	agent, err := agents.NewToolCallingAgent(model, agentTools, "system", map[string]interface{}{
		"max_steps": 10,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Test different types of search queries
	searchQueries := []string{
		"What are the latest developments in artificial intelligence in 2024?",
		"How does quantum computing work and what are its applications?",
		"What is the current status of renewable energy adoption globally?",
		"Find information about the Mars Perseverance rover recent discoveries",
		"What are the health benefits and risks of intermittent fasting?",
	}

	for i, query := range searchQueries {
		fmt.Printf("\n%d. %s\n", i+1, query)
		fmt.Println(strings.Repeat("-", len(query)+3))

		maxSteps := 10
		result, err := agent.Run(&agents.RunOptions{
			Task:     query,
			MaxSteps: &maxSteps,
		})

		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		fmt.Printf("✅ Result: %v\n", result.Output)
	}

	fmt.Println("\n🎉 Web search agent demonstration completed!")
	fmt.Println("\nNote: Search quality depends on:")
	fmt.Println("- Available search APIs (set SERP_API_KEY or SERPER_API_KEY for enhanced results)")
	fmt.Println("- Model capabilities and internet access")
	fmt.Println("- Rate limits of search providers")
}

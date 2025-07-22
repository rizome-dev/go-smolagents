# go-smolagents

<img src="logo.png" alt="Logo" width="215" align="right">

[![GoDoc](https://pkg.go.dev/badge/github.com/rizome-dev/go-smolagents)](https://pkg.go.dev/github.com/rizome-dev/go-smolagents)
[![Go Report Card](https://goreportcard.com/badge/github.com/rizome-dev/go-smolagents)](https://goreportcard.com/report/github.com/rizome-dev/go-smolagents)

```shell
go get github.com/rizome-dev/go-smolagents
```

built by: [rizome labs](https://rizome.dev)

contact us: [hi (at) rizome.dev](mailto:hi@rizome.dev)

## Quick Start

### Simple Usage

```go
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

    // Run the agent with a longer timeout
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    result, err := agent.Run(&agents.RunOptions{
        Task:    "What is 5 factorial?",
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
```

### ReactCodeAgent with Custom Options

```go
package main

import (
    "github.com/rizome-dev/go-smolagents/pkg/agents"
    "github.com/rizome-dev/go-smolagents/pkg/models"
)

func main() {
    // Create model
    model, _ := models.CreateModel(models.ModelTypeOpenAIServer, "gpt-4", map[string]interface{}{
        "api_key": "your-api-key",
    })
    
    // Custom agent options
    options := &agents.ReactCodeAgentOptions{
        AuthorizedPackages: []string{"fmt", "strings", "math", "json"},
        EnablePlanning:     true,
        PlanningInterval:   3,
        MaxSteps:          15,
        Verbose:           true,
    }
    
    // Create agent
    agent, err := agents.NewReactCodeAgent(model, nil, "You are a helpful coding assistant.", options)
    if err != nil {
        panic(err)
    }
    
    // Run task
    result, _ := agent.Run(&agents.RunOptions{
        Task: "Write a function to check if a string is a palindrome and test it",
    })
    
    fmt.Printf("Result: %v\n", result.Output)
}
```

### Available Tools

```go
// Web & Search
default_tools.NewWebSearchTool()        // Multi-engine web search
default_tools.NewDuckDuckGoSearchTool() // DuckDuckGo search
default_tools.NewGoogleSearchTool()     // Google search (API key required)
default_tools.NewWikipediaSearchTool()  // Wikipedia search
default_tools.NewVisitWebpageTool()     // Web page content extraction

// Code Execution
default_tools.NewGoInterpreterTool()    // Sandboxed Go execution

// Communication
default_tools.NewSpeechToTextTool()     // Audio transcription
default_tools.NewUserInputTool()        // Interactive user input
default_tools.NewFinalAnswerTool()      // Final response

// Processing
default_tools.NewPipelineTool()         // HuggingFace pipelines
tools.NewVisionWebBrowser()             // Web automation with screenshots
```

### Supported Models
```go
// OpenAI
models.CreateModel(models.ModelTypeOpenAIServer, "gpt-4", options)

// HuggingFace
models.CreateModel(models.ModelTypeInferenceClient, "meta-llama/Llama-2-7b-chat-hf", options)

// LiteLLM (100+ providers)
models.CreateModel(models.ModelTypeLiteLLM, "claude-3-sonnet", options)

// AWS Bedrock
models.CreateModel(models.ModelTypeBedrockModel, "anthropic.claude-v2", options)

```

## Advanced Features

### Remote Execution
```go
// Distributed code execution
executor := executors.NewRemoteExecutor([]string{
    "http://executor1:8080",
    "http://executor2:8080",
}, options)

result, err := executor.Execute("python", code, options)
```

### Vision & Web Automation
```go
// Web browser automation
browser := tools.NewVisionWebBrowser()
result, _ := browser.Forward("navigate", map[string]interface{}{
    "url": "https://example.com",
})
```

### Memory Management
```go
// Persistent conversation memory
memory := memory.NewConversationMemory()
agent.SetMemory(memory)
```

## Testing

```bash
# Run all tests
go test ./pkg/smolagents/...

# Test specific components
go test ./pkg/smolagents/agents
go test ./pkg/smolagents/tools
go test ./pkg/smolagents/models

# Run example
cd examples/react_code_agent && go run main.go
```

### Custom Tools
```go
// Implement the Tool interface
type MyTool struct {
    *tools.BaseTool
}

func (mt *MyTool) forward(args ...interface{}) (interface{}, error) {
    // Your tool logic here
    return "result", nil
}

// Register and use
default_tools.RegisterTool("my_tool", func() tools.Tool {
    return NewMyTool()
})
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.

## Credits

Based on the original [smolagents](https://github.com/huggingface/smolagents) Python library by HuggingFace.

---

**Built with ❤️  by Rizome Labs, Inc.**

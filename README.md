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

## Usage Guide

```bash
# Required: Set up at least one model API key
export HF_TOKEN="your-huggingface-token"         # For HuggingFace models
export OPENAI_API_KEY="your-openai-key"          # For OpenAI models
export AWS_REGION="us-east-1"                    # For AWS Bedrock

# Optional: For web search tools
export SERP_API_KEY="your-serpapi-key"
export SERPER_API_KEY="your-serper-key"
```

### Creating Agents

#### Basic Agent Creation
```go
import (
    "github.com/rizome-dev/go-smolagents/pkg/agents"
    "github.com/rizome-dev/go-smolagents/pkg/models"
)

// Using HuggingFace
model := models.NewInferenceClientModel(
    "Qwen/Qwen2.5-Coder-32B-Instruct",
    os.Getenv("HF_TOKEN"),
    nil,
)

// Using OpenAI
model, err := models.CreateModel(
    models.ModelTypeOpenAIServer,
    "gpt-4o",
    map[string]interface{}{
        "api_key": os.Getenv("OPENAI_API_KEY"),
    },
)

// Create agent
agent, err := agents.NewReactCodeAgent(model, nil, "", nil)
if err != nil {
    log.Fatal(err)
}
defer agent.Close() // Always close agents
```

#### Advanced Configuration
```go
options := &agents.ReactCodeAgentOptions{
    // Security: Limit packages available in code execution
    AuthorizedPackages: []string{"fmt", "strings", "math", "json", "time"},
    
    // Execution limits
    MaxCodeLength: 100000,  // Characters
    MaxSteps:      15,      // Iterations
    
    // Features
    EnablePlanning:   true,  // Multi-step planning
    PlanningInterval: 3,     // Steps between re-planning
    Verbose:          true,  // Detailed logs
    StreamOutputs:    true,  // Real-time output
}

agent, err := agents.NewReactCodeAgent(model, nil, systemPrompt, options)
```

### Running Tasks

```go
// Basic execution
result, err := agent.Run(&agents.RunOptions{
    Task: "Write a function to find prime numbers up to 100",
})

// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

result, err := agent.Run(&agents.RunOptions{
    Task:    "Analyze this data and create visualizations",
    Context: ctx,
})

// Handle results
if err != nil {
    log.Printf("Error: %v", err)
    if result != nil {
        log.Printf("Completed %d steps before failure", result.StepCount)
    }
} else {
    fmt.Printf("Success! Output: %v\n", result.Output)
}
```

### Working with Tools

#### Built-in Tools
```go
import "github.com/rizome-dev/go-smolagents/pkg/default_tools"

// Available tools
interpreter := default_tools.NewGoInterpreterTool()  // Execute Go code
webSearch := default_tools.NewWebSearchTool()        // Search the web
wiki := default_tools.NewWikipediaSearchTool()       // Search Wikipedia
webpage := default_tools.NewVisitWebpageTool()       // Visit URLs
finalAnswer := default_tools.NewFinalAnswerTool()    // Provide final answer

// Create agent with specific tools
agent, _ := agents.NewReactCodeAgent(
    model,
    []tools.Tool{interpreter, webSearch},
    "",
    nil,
)
```

#### Creating Custom Tools
```go
type CustomTool struct {
    *tools.BaseTool
}

func NewCustomTool() *CustomTool {
    inputs := map[string]*tools.ToolInput{
        "query": tools.NewToolInput("string", "The query to process"),
    }
    
    baseTool := tools.NewBaseTool(
        "custom_tool",
        "Description of what this tool does",
        inputs,
        "string", // output type
    )
    
    tool := &CustomTool{BaseTool: baseTool}
    tool.ForwardFunc = tool.execute
    return tool
}

func (t *CustomTool) execute(args ...interface{}) (interface{}, error) {
    query := args[0].(string)
    // Tool logic here
    return result, nil
}
```

### Model Providers

#### OpenAI and Compatible APIs
```go
// OpenAI
model, _ := models.CreateModel(models.ModelTypeOpenAIServer, "gpt-4o", map[string]interface{}{
    "api_key": os.Getenv("OPENAI_API_KEY"),
})

// Anthropic via OpenAI-compatible endpoint
model, _ := models.CreateModel(models.ModelTypeOpenAIServer, "claude-3-opus", map[string]interface{}{
    "api_key":  os.Getenv("ANTHROPIC_API_KEY"),
    "base_url": "https://api.anthropic.com/v1",
})

// Local models via Ollama
model, _ := models.CreateModel(models.ModelTypeOpenAIServer, "llama3", map[string]interface{}{
    "base_url": "http://localhost:11434/v1",
})
```

#### HuggingFace
```go
// Public models
model := models.NewInferenceClientModel(
    "Qwen/Qwen2.5-Coder-32B-Instruct",
    os.Getenv("HF_TOKEN"),
    nil,
)

// Dedicated endpoints
model := models.NewInferenceClientModel(
    "your-model-id",
    os.Getenv("HF_TOKEN"),
    map[string]interface{}{
        "base_url": "https://your-endpoint.endpoints.huggingface.cloud",
    },
)
```

#### AWS Bedrock
```go
// Claude on Bedrock
model, _ := models.CreateModel(models.ModelTypeBedrock, "anthropic.claude-3-opus", map[string]interface{}{
    "region": "us-east-1",
})

// Llama on Bedrock
model, _ := models.CreateModel(models.ModelTypeBedrock, "meta.llama3-70b-instruct", map[string]interface{}{
    "region": "us-west-2",
})
```

### Advanced Patterns

#### Error Handling
```go
result, err := agent.Run(&agents.RunOptions{Task: task})

if err != nil {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        log.Println("Task timed out")
    case strings.Contains(err.Error(), "rate limit"):
        log.Println("Rate limited, retrying...")
        time.Sleep(60 * time.Second)
        // Retry logic
    default:
        log.Printf("Error: %v", err)
    }
}
```

#### Concurrent Execution
```go
func runConcurrentTasks(tasks []string, model models.Model) {
    var wg sync.WaitGroup
    results := make(chan Result, len(tasks))
    
    for _, task := range tasks {
        wg.Add(1)
        go func(t string) {
            defer wg.Done()
            
            agent, _ := agents.NewReactCodeAgent(model, nil, "", nil)
            defer agent.Close()
            
            result, err := agent.Run(&agents.RunOptions{Task: t})
            results <- Result{Output: result, Error: err}
        }(task)
    }
    
    wg.Wait()
    close(results)
}
```

#### Memory Access
```go
// Access conversation history
memory := agent.GetMemory()
messages := memory.GetMessages()

// Access execution steps
steps := memory.GetSteps()
for _, step := range steps {
    fmt.Printf("Step: %v\n", step)
}

// Clear memory for fresh start
memory.Clear()
```

#### Multi-Agent Systems
```go
// Create specialized agents
dataAnalyst, _ := agents.NewReactCodeAgent(model, nil, 
    "You are a data analysis expert.", nil)
    
codeReviewer, _ := agents.NewReactCodeAgent(model, nil, 
    "You are a code review expert.", nil)

// Create managed agents for coordination
analyst := agents.NewManagedAgent("analyst", "Data analysis", dataAnalyst)
reviewer := agents.NewManagedAgent("reviewer", "Code review", codeReviewer)

// Coordinate between agents
// ... coordination logic
```

### Security Best Practices

1. **Sandbox Code Execution**
   ```go
   options := &agents.ReactCodeAgentOptions{
       AuthorizedPackages: []string{"fmt", "math"}, // Only safe packages
       MaxCodeLength:      10000,                   // Limit code size
   }
   ```

2. **API Key Management**
   ```go
   // Always use environment variables
   apiKey := os.Getenv("OPENAI_API_KEY")
   if apiKey == "" {
       log.Fatal("API key not set")
   }
   ```

3. **Input Validation**
   ```go
   if len(task) > 10000 {
       return fmt.Errorf("task too long")
   }
   ```

### Performance Optimization

1. **Reuse Model Instances**
   ```go
   // Create model once
   model := models.NewInferenceClientModel(modelID, token, nil)
   
   // Use for multiple agents
   for _, task := range tasks {
       agent, _ := agents.NewReactCodeAgent(model, nil, "", nil)
       // Use agent
       agent.Close()
   }
   ```

2. **Context Management**
   ```go
   // Set appropriate timeouts
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   ```

3. **Limit Memory Growth**
   ```go
   if len(memory.GetMessages()) > 100 {
       // Summarize and reset
       memory.Clear()
   }
   ```

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.

## Credits

Based on the original [smolagents](https://github.com/huggingface/smolagents) Python library by HuggingFace.

---

**Built with ❤️  by Rizome Labs, Inc.**

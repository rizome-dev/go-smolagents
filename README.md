# go-smolagents

[![GoDoc](https://pkg.go.dev/badge/github.com/rizome-dev/go-smolagents)](https://pkg.go.dev/github.com/rizome-dev/go-smolagents)
[![Go Report Card](https://goreportcard.com/badge/github.com/rizome-dev/go-smolagents)](https://goreportcard.com/report/github.com/rizome-dev/go-smolagents)

```shell
go get github.com/rizome-dev/go-smolagents
```

built by: [rizome labs](https://rizome.dev)

contact us: [hi (at) rizome.dev](mailto:hi@rizome.dev)

## 🚀 Quick Start

### From Source

```bash
git clone https://github.com/rizome-dev/go-smolagents
cd go-smolagents
go mod download

# Build and test
go build ./...
go test ./...
```

### Environment Variables
```bash
# Required: At least one model API key
export OPENAI_API_KEY="sk-..."              # Recommended
# OR
export HF_API_TOKEN="hf_..."                # HuggingFace alternative

# Optional: Enhanced search capabilities
export SERP_API_KEY="..."                   # Google search via SerpAPI
export SERPER_API_KEY="..."                # Alternative Google search
```

## 🛠️ Library Usage

### Basic Example
```go
package main

import (
    "fmt"
    "os"
    "github.com/rizome-dev/go-smolagents/pkg/agents"
    "github.com/rizome-dev/go-smolagents/pkg/models"
)

func main() {
    token := os.Getenv("HF_API_TOKEN")
    model := models.NewInferenceClientModel("Qwen/Qwen2.5-Coder-32B-Instruct", token)
    
    agent, err := agents.NewReactCodeAgent(model, nil, "", nil)
    if err != nil {
        panic(err)
    }
    
    result, err := agent.Run(&agents.RunOptions{
        Task: "Calculate the factorial of 10",
    })
    
    fmt.Printf("Result: %v\n", result.Output)
}

```

## 📚 Library Features

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
default_tools.NewPythonInterpreterTool() // Python code execution

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

// Local models
models.CreateModel(models.ModelTypeVLLM, "local-model", options)
models.CreateModel(models.ModelTypeMLX, "mlx-model", options)
```

## 🔬 Examples

### ReactCodeAgent Example
```bash
cd examples/react_code_agent
export HF_API_TOKEN="your_token_here"
go run main.go
```

The ReactCodeAgent demonstrates:
- **ReAct Framework**: Thought → Code → Observation cycles
- **Dynamic Parsing**: Extracts reasoning and code from LLM responses
- **Sandboxed Execution**: Safe Go code execution with security restrictions
- **Planning System**: Step-by-step reasoning for complex tasks
- **YAML Prompts**: Customizable prompt templates for different behaviors

## 🏗️ Advanced Features

### Multi-Agent Coordination
```go
// Create research manager with multiple workers
manager, err := NewResearchManager(model, 3) // 3 worker agents
report, err := manager.ResearchProject("quantum computing", 15*time.Minute)
```

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

## 🧪 Testing

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

## 🔧 Configuration

### Environment Variables
```bash
# Model APIs
export OPENAI_API_KEY="sk-..."
export HF_API_TOKEN="hf_..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Search APIs (optional)
export SERP_API_KEY="..."
export SERPER_API_KEY="..."
export GOOGLE_API_KEY="..."

# AWS (for Bedrock)
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"
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

## 📈 Use Cases

- **Research & Analysis**: Multi-source information gathering
- **Code Development**: AI-assisted programming with execution
- **Content Creation**: Research-backed writing and analysis
- **Data Processing**: Automated analysis and visualization
- **Business Intelligence**: Market research and competitive analysis
- **Educational**: Interactive learning and explanation

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes following Go conventions
4. Add tests for new functionality
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## 📄 License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.

## 🙏 Credits

Based on the original [smolagents](https://github.com/huggingface/smolagents) Python library by HuggingFace.

---

**Get started in 30 seconds**: Create a simple Go file with the ReactCodeAgent example above and run it!

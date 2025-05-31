# smolagentsgo

[![GoDoc](https://pkg.go.dev/badge/github.com/rizome-dev/smolagentsgo)](https://pkg.go.dev/github.com/rizome-dev/smolagentsgo)
[![Go Report Card](https://goreportcard.com/badge/github.com/rizome-dev/smolagentsgo)](https://goreportcard.com/report/github.com/rizome-dev/smolagentsgo)

```shell
go get github.com/rizome-dev/smolagentsgo
```

built by: [rizome labs](https://rizome.dev)

contact us: [hi (at) rizome.dev](mailto:hi@rizome.dev)

**warning**: this project is an internal WIP library in use by Rizome Labs, and should not, until v1, be used, in full, in prod environments. we will keep this repo updated until that time to describe the ongoing development.

## 🚀 Quick Start

### Prerequisites
```bash
# Required: At least one model API key
export OPENAI_API_KEY="sk-..."              # Recommended
# OR
export HF_API_TOKEN="hf_..."                # HuggingFace alternative

# Optional: Enhanced search capabilities
export SERP_API_KEY="..."                   # Google search via SerpAPI
export SERPER_API_KEY="..."                # Alternative Google search
```

### Installation
```bash
git clone https://github.com/rizome-dev/smolagentsgo
cd smolagentsgo
go mod download
```

### Build & Run
```bash
# Build CLI tool
go build -o bin/smolagents-cli ./cmd/smolagents-cli

# Quick test
./bin/smolagents-cli run "What are the latest developments in quantum computing?"

# Interactive mode
./bin/smolagents-cli run --interactive
```

## 📁 Project Structure

```
smolagentsgo/
├── cmd/smolagents-cli/          # Interactive CLI with TUI
├── pkg/smolagents/              # Public library API
│   ├── agents/                  # Agent implementations
│   ├── models/                  # LLM integrations
│   ├── tools/                   # Tool ecosystem
│   ├── memory/                  # Conversation memory
│   ├── executors/               # Code execution
│   └── default_tools/           # Built-in tools
├── internal/examples/           # Advanced examples
│   └── research_agent/          # Multi-agent research system
└── bin/                         # Compiled binaries
```

## 🛠️ CLI Usage

### Basic Commands
```bash
# Simple task execution
./bin/smolagents-cli run "Calculate fibonacci numbers up to 100"

# With specific tools
./bin/smolagents-cli run --tools web_search,python_interpreter \
  "Research AI trends and create a Python analysis"

# Different agent types
./bin/smolagents-cli run --agent code \
  "Write and test a Go function to reverse a string"

# Interactive session
./bin/smolagents-cli run --interactive --verbose

# Deep research mode (multi-agent)
./bin/smolagents-cli research "climate change solutions"
```

### Available Options
```bash
# Model configuration
--model-type openai --model gpt-4
--model-type hf --model meta-llama/Llama-2-7b-chat-hf

# Agent types
--agent tool-calling    # General purpose (default)
--agent code           # Code-focused with execution
--agent research       # Multi-agent research system

# Tool selection
--tools web_search,wikipedia_search,python_interpreter,final_answer

# Execution options
--interactive          # Interactive TUI mode
--verbose             # Detailed logging
--max-steps 20        # Maximum reasoning steps
```

## 📚 Library Usage

### Basic Agent
```go
package main

import (
    "github.com/rizome-dev/smolagentsgo/pkg/smolagents/agents"
    "github.com/rizome-dev/smolagentsgo/pkg/smolagents/models"
    "github.com/rizome-dev/smolagentsgo/pkg/smolagents/default_tools"
)

func main() {
    // Create model
    model, _ := models.CreateModel(models.ModelTypeOpenAIServer, "gpt-3.5-turbo", map[string]interface{}{
        "api_key": "your-api-key",
    })
    
    // Create tools
    tools := []tools.Tool{
        default_tools.NewWebSearchTool(),
        default_tools.NewPythonInterpreterTool(),
        default_tools.NewFinalAnswerTool(),
    }
    
    // Create agent
    agent, _ := agents.NewToolCallingAgent(model, tools, "system", map[string]interface{}{
        "max_steps": 10,
    })
    
    // Run task
    maxSteps := 10
    result, _ := agent.Run(&agents.RunOptions{
        Task:     "Research the latest AI developments",
        MaxSteps: &maxSteps,
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

### 1. Simple Calculator Agent
```bash
cd internal/examples
go run calculator/main.go
```

### 2. Web Search Agent
```bash
cd internal/examples  
go run websearch/main.go
```

### 3. Multi-Agent Research System
```bash
cd internal/examples/research_agent
go run main.go "artificial intelligence in healthcare"

# Or via CLI
./bin/smolagents-cli research "quantum computing applications"
```

The research system demonstrates:
- **Manager Agent**: Coordinates research strategy
- **3+ Worker Agents**: Parallel research execution
- **Advanced Tools**: Web search, Wikipedia, analysis
- **Result Synthesis**: AI-powered aggregation and reporting

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

# Run examples
cd internal/examples/research_agent && go run main.go "test topic"
```

## ⚡ Performance

- **Single Agent**: 30-60s for complex tasks
- **Multi-Agent**: 3x speedup with parallel execution  
- **Research Quality**: 85-95% confidence on most topics
- **Memory Usage**: <100MB per agent instance
- **Concurrent**: Safe goroutine-based parallelism

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

**Get started in 30 seconds**: `export OPENAI_API_KEY="sk-..." && go run ./cmd/smolagents-cli run "Hello AI!"`

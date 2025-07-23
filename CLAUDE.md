# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Installation and Setup

```bash
# Install the library
go get github.com/rizome-dev/go-smolagents

# Clone from source
git clone https://github.com/rizome-dev/go-smolagents
cd go-smolagents
go mod download
```

## Build and Development Commands

```bash
# Build the module
go build ./...

# Run tests
go test ./...

# Run tests with verbose output  
go test -v ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests for specific package
go test ./pkg/memory
go test ./pkg/agents -v

# Run linting and code quality checks
go vet ./...

# Format code
go fmt ./...

# Check for security vulnerabilities
go list -json -deps ./... | nancy sleuth

# Update dependencies (check for newer versions)
go list -u -m all
go get -u ./...
go mod tidy

# Run examples
cd examples/react && go run main.go  # ReAct code agent with Thought/Code/Observation cycles

# Build example binaries
cd examples/react && go build -o react_agent
```

## Architecture Overview

This is a Go implementation of an AI agent framework inspired by smolagents. The codebase follows a modular architecture:

**Core Package Structure:**
- `pkg/smolagents.go` - Main package exports and re-exports from sub-packages
- `pkg/agents/` - Agent implementations (MultiStepAgent, ReactCodeAgent, ManagedAgent)
- `pkg/prompts/` - YAML-based prompt templates for ReAct reasoning
- `pkg/parser/` - Dynamic LLM response parser for Thought/Code/Observation extraction
- `pkg/models/` - LLM integration layer with multiple backend support
- `pkg/tools/` - Tool interface and implementations (web search, calculator, etc.)
- `pkg/memory/` - Agent memory management (conversation history, steps)
- `pkg/utils/` - Shared utilities and error types
- `pkg/default_tools/` - Built-in tool implementations
- `pkg/executors/` - Code execution engines (Go executor, remote executor)
- `pkg/monitoring/` - Performance monitoring and logging
- `pkg/agent_types/` - Core agent type definitions
- `pkg/display/` - Beautiful CLI output formatting with color support

**Key Agent Types:**
- `BaseMultiStepAgent` - Foundation for step-by-step execution agents
- `ReactCodeAgent` - ReAct reasoning agent with code execution, implementing Thought/Code/Observation cycles
- `ManagedAgent` - Wrapper for agent delegation in multi-agent systems

**LLM Integration:**
- Abstracted through `ModelFunc` interface for any LLM backend
- Supported backends:
  - HuggingFace Inference API (`inference_client.go`)
  - OpenAI API and compatible servers (`openai_server.go`)
  - AWS Bedrock (`bedrock_model.go`)
  - LiteLLM proxy (`litellm_model.go`)
- Messages follow standardized `Message`/`ChatMessage` types
- Multimodal support for vision models (`multimodal.go`)
- Structured generation support for constrained outputs

**Concurrency Support:**
- Agents designed to work with Go's goroutines and channels
- Examples show concurrent agent execution patterns
- Memory implementations are not thread-safe by default

## Environment Requirements

- Go 1.23.4+ (per go.mod)
- At least one model API key:
  - `OPENAI_API_KEY` for OpenAI models (recommended)
  - `HF_TOKEN` for HuggingFace models
  - AWS credentials for Bedrock models
- Optional API keys for enhanced functionality:
  - `SERP_API_KEY` for Google search via SerpAPI
  - `SERPER_API_KEY` for alternative Google search

## Key Files to Understand

- `pkg/smolagents.go` - Package entry point with all exports
- `pkg/agents/agents.go` - Core agent execution logic  
- `pkg/agents/react_code_agent.go` - ReAct implementation with code execution
- `pkg/prompts/prompts.go` - YAML-based prompt management system
- `pkg/parser/parser.go` - Dynamic LLM response parser for extracting thoughts, code, and observations
- `pkg/executors/go_executor.go` - Sandboxed Go code execution
- `pkg/models/models.go` - Model interface and factory functions
- `pkg/models/inference_client.go` - HuggingFace API client implementation
- `pkg/tools/tools.go` - Tool interface definition
- `pkg/memory/memory.go` - Memory abstraction interfaces
- `pkg/display/display.go` - CLI output formatting utilities

## Testing Patterns

Tests use standard Go testing framework. Test files exist for:
- `pkg/agents/react_code_agent_test.go` - Agent behavior tests
- `pkg/executors/go_executor_test.go` - Code execution tests
- `pkg/memory/memory_test.go` - Memory implementation tests
- `pkg/parser/parser_test.go` - Response parsing tests
- `pkg/prompts/prompts_test.go` - Prompt template tests
- `pkg/tools/tools_test.go` - Tool functionality tests
- `pkg/models/structured_generation_test.go` - Structured output tests

When adding tests, follow existing patterns and aim for comprehensive coverage of edge cases.

## Code Quality and Security Monitoring

**IMPORTANT: Claude should proactively monitor and fix the following on every session:**

### Go Report Card
- Check https://goreportcard.com/report/github.com/rizome-dev/go-smolagents for code quality issues
- Address any reported issues: gofmt, go vet, gocyclo, ineffassign, misspell, golint
- Run `go fmt ./...` and `go vet ./...` regularly to maintain code quality
- Target 100% Go Report Card score

### Dependabot Security Updates
- Monitor GitHub Dependabot alerts for security vulnerabilities
- Update vulnerable dependencies promptly using `go get -u [dependency]`
- Run `go mod tidy` after dependency updates
- Test all functionality after security updates

### Dependencies to Monitor
- `golang.org/x/net` - Network libraries (security-critical)
- `golang.org/x/sys` - System interfaces (security-critical) 
- `golang.org/x/text` - Text processing (security-critical)
- `golang.org/x/crypto` - Cryptographic libraries (security-critical)
- Third-party dependencies with known CVEs

### Environment Variables
- API keys should be stored in `.env` files (git-ignored)
- Never commit `.env` files or expose API keys in code

### Automation Checklist
When working on this codebase, Claude should:
1. Check Go Report Card status first
2. Review any open Dependabot alerts
3. Run `go vet ./...` and `go fmt ./...` before commits
4. Update dependencies if security alerts exist
5. Run full test suite after any dependency updates
6. Maintain backwards compatibility unless explicitly requested otherwise

## Development Best Practices

### Code Style
- Follow standard Go formatting (`go fmt`)
- Use meaningful variable and function names
- Add comments for exported functions and complex logic
- Keep functions focused and under 50 lines when possible
- Avoid cyclomatic complexity > 10

### Security
- Never commit API keys or secrets
- Validate all external inputs
- Use secure defaults for all configurations
- Follow principle of least privilege

### Performance
- Use context.Context for cancellation
- Implement proper error handling and timeouts
- Avoid memory leaks in long-running agents
- Use goroutines judiciously for concurrent operations

## Usage Examples

### Basic Agent Creation
```go
// Using HuggingFace model
model := models.NewInferenceClientModel("Qwen/Qwen2.5-Coder-32B-Instruct", token)
agent, _ := agents.NewReactCodeAgent(model, nil, "", nil)

// Using OpenAI model
model, _ := models.CreateModel(models.ModelTypeOpenAIServer, "gpt-4", map[string]interface{}{
    "api_key": "your-api-key",
})
agent, _ := agents.NewReactCodeAgent(model, nil, "You are a helpful assistant.", nil)
```

### Agent Configuration
```go
options := &agents.ReactCodeAgentOptions{
    AuthorizedPackages: []string{"fmt", "strings", "math", "json"},
    EnablePlanning:     true,
    PlanningInterval:   3,
    MaxSteps:          15,
    Verbose:           true,
}
```

## Parser Usage

The parser extracts structured information from LLM responses:
- **Thought extraction**: Identifies reasoning steps in ReAct format
- **Code block parsing**: Extracts code between `<code>` tags
- **Action parsing**: Identifies tool calls and parameters
- **Error detection**: Captures and formats parsing errors

## Notes

- Agent creation requires a `ModelFunc` - see examples for different backend patterns
- Tools implement a standardized JSON schema interface for LLM interaction
- Error handling uses custom error types defined in `utils/` package
- The parser dynamically adapts to different LLM response formats
- Display package provides consistent CLI formatting across all agent outputs

## Documentation

Full API documentation is available at: https://pkg.go.dev/github.com/rizome-dev/go-smolagents

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details
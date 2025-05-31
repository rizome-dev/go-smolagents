# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build the module
go build ./...

# Run tests
go test ./...

# Run tests with verbose output  
go test -v ./...

# Run tests for specific package
go test ./memory

# Run linting and code quality checks
go vet ./...

# Format code
go fmt ./...

# Check for security vulnerabilities
go list -json -deps ./... | nancy sleuth

# Update dependencies (check for newer versions)
go list -u -m all
go get -u ./...

# Run examples
cd internal/examples/calculator && go run main.go  # Requires HF_API_TOKEN env var
cd internal/examples/websearch && go run main.go  # Requires HF_API_TOKEN env var
cd internal/examples/research_agent && go run main.go  # Basic research agent
cd internal/examples/enhanced_research_agent && go run main.go  # Advanced agentic research system
```

## Architecture Overview

This is a Go port of the Python smolagents library that provides a framework for building AI agents. The codebase follows a modular architecture:

**Core Package Structure:**
- `smolagentsgo.go` - Main package exports and re-exports from sub-packages
- `agents/` - Agent implementations (MultiStepAgent, ToolCallingAgent, CodeAgent, ManagedAgent)
- `models/` - LLM integration layer with HuggingFace support
- `tools/` - Tool interface and implementations (web search, calculator, etc.)
- `memory/` - Agent memory management (conversation history, steps)
- `utils/` - Shared utilities and error types

**Key Agent Types:**
- `BaseMultiStepAgent` - Foundation for step-by-step execution agents
- `ToolCallingAgent` - Agents that can use external tools 
- `CodeAgent` - Specialized for code execution with Go sandbox support
- `ManagedAgent` - Wrapper for agent delegation in multi-agent systems

**LLM Integration:**
- Abstracted through `ModelFunc` interface for any LLM backend
- Currently implements HuggingFace Inference API client
- Messages follow standardized `Message`/`ChatMessage` types

**Concurrency Support:**
- Agents designed to work with Go's goroutines and channels
- Examples show concurrent agent execution patterns
- Memory implementations are not thread-safe by default

## Environment Requirements

- `HF_API_TOKEN` environment variable required for HuggingFace model usage in examples
- Go 1.23.4+ (per go.mod)

## Key Files to Understand

- `smolagentsgo.go` - Package entry point with all exports
- `agents/multi_step_agent.go` - Core agent execution logic  
- `agents/tool_calling_agent.go` - Tool integration patterns
- `models/inference_client.go` - LLM API client implementation
- `tools/tools.go` - Tool interface definition
- `memory/memory.go` - Memory abstraction interfaces
- `AGENTIC_WORKFLOW_BEST_PRACTICES.md` - Comprehensive guide to advanced agentic design patterns

## Testing Patterns

Tests use standard Go testing framework. Current test coverage is minimal with only memory package having tests. When adding tests, follow the pattern in `memory/simple_memory_test.go`.

## Code Quality and Security Monitoring

**IMPORTANT: Claude should proactively monitor and fix the following on every session:**

### Go Report Card
- Check https://goreportcard.com/report/github.com/rizome-dev/smolagentsgo for code quality issues
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

## Notes

- The `smolagents/` subdirectory contains the original Python implementation and is separate from the Go port
- Agent creation requires a `ModelFunc` - see examples for HuggingFace integration patterns
- Tools implement a standardized JSON schema interface for LLM interaction
- Error handling uses custom error types defined in `utils/` package
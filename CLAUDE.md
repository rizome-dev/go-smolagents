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

# Run linting (if available)
go vet ./...

# Format code
go fmt ./...

# Run examples
cd examples/calculator && go run main.go  # Requires HF_API_TOKEN env var
cd examples/web_search && go run main.go  # Requires HF_API_TOKEN env var
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
- `models/huggingface_model.go` - LLM API client implementation
- `tools/tools.go` - Tool interface definition
- `memory/memory.go` - Memory abstraction interfaces

## Testing Patterns

Tests use standard Go testing framework. Current test coverage is minimal with only memory package having tests. When adding tests, follow the pattern in `memory/simple_memory_test.go`.

## Notes

- The `smolagents/` subdirectory contains the original Python implementation and is separate from the Go port
- Agent creation requires a `ModelFunc` - see examples for HuggingFace integration patterns
- Tools implement a standardized JSON schema interface for LLM interaction
- Error handling uses custom error types defined in `utils/` package
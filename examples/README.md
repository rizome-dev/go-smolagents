# Go-Smolagents Examples

This directory contains examples demonstrating how to use the go-smolagents library with the new ReAct implementation.

## Examples

### 1. ReactCodeAgent Example (`react_code_agent/`)

Demonstrates the full ReAct (Reasoning + Acting) implementation for code execution:

```bash
cd react_code_agent
export HF_API_TOKEN=your_token_here
go run main.go
```

Features demonstrated:
- Thought → Code → Observation cycles
- Dynamic code execution with sandboxing
- Planning system for complex tasks
- Custom prompt templates
- Final answer detection

### 2. Custom Configuration Example

Shows how to customize the ReactCodeAgent:

```go
model := models.NewInferenceClientModel("Qwen/Qwen2.5-Coder-32B-Instruct", token)
options := &agents.ReactCodeAgentOptions{
    AuthorizedPackages: []string{"fmt", "strings", "math"},
    EnablePlanning:     true,
    PlanningInterval:   3,
    MaxSteps:          10,
    Verbose:           true,
}
agent, err := agents.NewReactCodeAgent(model, nil, "", options)
```

### 3. With Tools Example

Demonstrates ReactCodeAgent with additional tools:

```go
// Tools will be available when default_tools package is complete
tools := []tools.Tool{
    // Add custom tools here
}
agent, err := agents.NewReactCodeAgent(model, tools, "You are a helpful assistant.", nil)
```

## Key Features of the ReAct Implementation

### 1. Prompt Templates

The system uses YAML-based prompt templates that can be customized:
- `code_agent.yaml` - Standard code execution prompts
- `toolcalling_agent.yaml` - Tool-based agent prompts
- `structured_code_agent.yaml` - Structured JSON response format

### 2. Dynamic Response Parsing

The parser can handle multiple response formats:
- Thought/Code/Observation patterns
- JSON-based tool calls
- Structured responses
- Error recovery

### 3. Code Execution Sandbox

Safe Go code execution with:
- Package restrictions
- Resource limits
- State persistence between executions
- Security validation

### 4. Planning System

Agents can plan their approach:
- Configurable planning intervals
- Step-by-step reasoning
- Progress tracking

## Environment Variables

- `HF_API_TOKEN`: Required for HuggingFace model access
- `GOPROXY`: Set to `off` for security in sandboxed execution
- `GOSUMDB`: Set to `off` for security in sandboxed execution

## Best Practices

1. **Model Selection**: Use code-optimized models like Qwen2.5-Coder for best results
2. **Task Formatting**: Be specific and clear in task descriptions
3. **Error Handling**: Always check the result state and handle errors appropriately
4. **Resource Limits**: Configure appropriate timeouts and memory limits for your use case
5. **Security**: Review authorized packages and ensure they match your security requirements

## Troubleshooting

If you encounter issues:

1. Ensure Go 1.23.4+ is installed
2. Run `go mod tidy` to update dependencies
3. Check that your HF_API_TOKEN is valid
4. For execution errors, check the executor's stderr output
5. Enable verbose mode for detailed logging

## Contributing

When adding new examples:
1. Create a new directory with a descriptive name
2. Include a main.go file with clear comments
3. Add error handling and logging
4. Update this README with your example
5. Test with different models and edge cases
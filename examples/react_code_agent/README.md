# ReactCodeAgent Example

This example demonstrates the ReactCodeAgent implementation with full ReAct (Reasoning + Acting) cycles.

## Running the Example

```bash
export HF_API_TOKEN="your_token_here"
go run main.go
```

## Important Notes

- **API Latency**: Each step takes 5-10 seconds due to HuggingFace Inference API latency
- **Total Time**: Simple tasks typically complete in 20-30 seconds (3-4 API calls)
- **Progress**: You'll see step numbers printed as the agent progresses
- **Provider**: The example uses the provider you specify (or auto-selects)

## Expected Output

The agent will:
1. Plan the approach (if planning is enabled)
2. Write and execute Go code step by step
3. Observe the results
4. Provide a final answer

Each step involves an API call to the LLM, so be patient while it processes.

## Customization

See `main_with_provider.go.example` for how to:
- Specify different providers
- Adjust timeouts
- Configure agent options
- Use different models
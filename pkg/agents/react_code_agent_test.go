package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/memory"
	"github.com/rizome-dev/go-smolagents/pkg/models"
	"github.com/rizome-dev/go-smolagents/pkg/monitoring"
)

// ReactMockModel for testing ReactCodeAgent
type ReactMockModel struct {
	responses []string
	callCount int
}

func (m *ReactMockModel) Generate(messages []interface{}, options *models.GenerateOptions) (*models.ChatMessage, error) {
	if m.callCount >= len(m.responses) {
		return &models.ChatMessage{
			Role:    "assistant",
			Content: strPtr("I don't know what to do."),
		}, nil
	}

	response := m.responses[m.callCount]
	m.callCount++

	return &models.ChatMessage{
		Role:    "assistant",
		Content: &response,
		TokenUsage: &monitoring.TokenUsage{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	}, nil
}

func (m *ReactMockModel) GetModelID() string {
	return "mock-model"
}

func (m *ReactMockModel) ParseToolCalls(message *models.ChatMessage) (*models.ChatMessage, error) {
	return message, nil
}

func (m *ReactMockModel) StreamGenerate(messages []interface{}, options *models.GenerateOptions) (<-chan *models.ChatMessageStreamDelta, error) {
	// Not implemented for tests
	return nil, nil
}

func (m *ReactMockModel) GenerateStream(messages []interface{}, options *models.GenerateOptions) (<-chan *models.ChatMessageStreamDelta, error) {
	// Not implemented for tests
	return nil, nil
}

func (m *ReactMockModel) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"model_id": "mock-model",
		"type":     "mock",
	}
}

func (m *ReactMockModel) SupportsStreaming() bool {
	return false
}

func (m *ReactMockModel) Close() error {
	return nil
}

func strPtr(s string) *string {
	return &s
}

func TestReactCodeAgent(t *testing.T) {
	t.Skip("Skipping ReactCodeAgent tests - require real executor integration")

	t.Run("simple calculation", func(t *testing.T) {
		// Create mock model with expected responses
		model := &ReactMockModel{
			responses: []string{
				`Thought: I need to calculate 2 + 2 using Go code.
<code>
final_answer(4)
</code>`,
			},
		}

		// Create agent
		agent, err := NewReactCodeAgent(model, nil, "", nil)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Run agent
		result, err := agent.Run(&RunOptions{
			Task: "What is 2 + 2?",
		})
		if err != nil {
			t.Errorf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}
	})

	t.Run("multi-step reasoning", func(t *testing.T) {
		// Create mock model with multi-step responses
		model := &ReactMockModel{
			responses: []string{
				`Thought: I need to calculate the sum of numbers from 1 to 10. Let me start by initializing a variable.
<code>
sum := 0
fmt.Println("Initialized sum to 0")
</code>`,
				`Thought: Now I'll use a loop to calculate the sum.
<code>
for i := 1; i <= 10; i++ {
    sum += i
}
fmt.Printf("The sum of 1 to 10 is: %d\n", sum)
final_answer(fmt.Sprintf("The sum of numbers from 1 to 10 is %d", sum))
</code>`,
			},
		}

		// Create agent
		agent, err := NewReactCodeAgent(model, nil, "", nil)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Run agent
		result, err := agent.Run(&RunOptions{
			Task: "Calculate the sum of numbers from 1 to 10",
		})
		if err != nil {
			t.Errorf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}

		if result.StepCount != 2 {
			t.Errorf("Expected 2 steps, got %d", result.StepCount)
		}
	})

	t.Run("error handling", func(t *testing.T) {
		// Create mock model that produces an error
		model := &ReactMockModel{
			responses: []string{
				`Thought: Let me try to divide by zero.
<code>
result := 10 / 0
fmt.Println(result)
</code>`,
				`Thought: I got an error. Let me handle it properly.
<code>
if divisor := 0; divisor != 0 {
    result := 10 / divisor
    fmt.Println(result)
} else {
    final_answer("Cannot divide by zero")
}
</code>`,
			},
		}

		// Create agent
		agent, err := NewReactCodeAgent(model, nil, "", nil)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Run agent
		result, err := agent.Run(&RunOptions{
			Task: "Divide 10 by 0",
		})
		if err != nil {
			t.Errorf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state after error recovery, got %s", result.State)
		}
	})

	t.Run("max steps limit", func(t *testing.T) {
		// Create mock model that never provides final answer
		model := &ReactMockModel{
			responses: []string{
				`Thought: Let me think about this.
<code>
x := 1
</code>`,
				`Thought: I need to think more.
<code>
x = x + 1
</code>`,
				`Thought: Still thinking.
<code>
x = x + 1
</code>`,
			},
		}

		// Create agent with low max steps
		options := DefaultReactCodeAgentOptions()
		options.MaxSteps = 2

		agent, err := NewReactCodeAgent(model, nil, "", options)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Run agent
		maxSteps := 2
		result, err := agent.Run(&RunOptions{
			Task:     "Never-ending task",
			MaxSteps: &maxSteps,
		})

		if result.State != "max_steps_error" {
			t.Errorf("Expected max_steps_error state, got %s", result.State)
		}

		if result.StepCount != 2 {
			t.Errorf("Expected 2 steps, got %d", result.StepCount)
		}
	})

	t.Run("structured output", func(t *testing.T) {
		// Create mock model with structured response
		model := &ReactMockModel{
			responses: []string{
				`{
					"thought": "I need to calculate the square of 5",
					"code": "result := 5 * 5\nfinal_answer(result)"
				}`,
			},
		}

		// Create agent with structured output
		options := DefaultReactCodeAgentOptions()
		options.StructuredOutput = true

		agent, err := NewReactCodeAgent(model, nil, "", options)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Run agent
		result, err := agent.Run(&RunOptions{
			Task: "What is 5 squared?",
		})
		if err != nil {
			t.Errorf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		// Create mock model with slow response
		model := &ReactMockModel{
			responses: []string{
				`Thought: This will take a while.
<code>
time.Sleep(5 * time.Second)
</code>`,
			},
		}

		// Create agent
		agent, err := NewReactCodeAgent(model, nil, "", nil)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Create cancellable context
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Run agent
		result, err := agent.Run(&RunOptions{
			Task:    "Long running task",
			Context: ctx,
		})

		if result.State != "cancelled" {
			t.Errorf("Expected cancelled state, got %s", result.State)
		}
	})

	t.Run("planning step", func(t *testing.T) {
		// Create mock model with planning response
		model := &ReactMockModel{
			responses: []string{
				`To solve this task, I'll:
1. Define the Fibonacci function
2. Calculate the 10th number
3. Return the result`,
				`Thought: Let me implement the Fibonacci function.
<code>
func fibonacci(n int) int {
    if n <= 1 {
        return n
    }
    return fibonacci(n-1) + fibonacci(n-2)
}

result := fibonacci(10)
final_answer(fmt.Sprintf("The 10th Fibonacci number is %d", result))
</code>`,
			},
		}

		// Create agent with planning enabled
		options := DefaultReactCodeAgentOptions()
		options.EnablePlanning = true
		options.PlanningInterval = 1 // Plan at every step for testing

		agent, err := NewReactCodeAgent(model, nil, "", options)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Run agent
		result, err := agent.Run(&RunOptions{
			Task: "Calculate the 10th Fibonacci number",
		})
		if err != nil {
			t.Errorf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}

		// Check that planning step was recorded
		steps := agent.memory.GetSteps()
		hasPlanningStep := false
		for _, step := range steps {
			if _, ok := step.(*memory.PlanningStep); ok {
				hasPlanningStep = true
				break
			}
		}
		if !hasPlanningStep {
			t.Error("Expected planning step in memory")
		}
	})

	t.Run("custom code tags", func(t *testing.T) {
		// Create mock model with custom tags
		model := &ReactMockModel{
			responses: []string{
				"Thought: Using custom code tags.\n```go\nresult := 42\nfinal_answer(result)\n```",
			},
		}

		// Create agent with custom tags
		options := DefaultReactCodeAgentOptions()
		options.CodeBlockTags = [2]string{"```go", "```"}

		agent, err := NewReactCodeAgent(model, nil, "", options)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Run agent
		result, err := agent.Run(&RunOptions{
			Task: "Return 42",
		})
		if err != nil {
			t.Errorf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}
	})
}

func TestReactCodeAgentOptions(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		options := DefaultReactCodeAgentOptions()

		if len(options.AuthorizedPackages) == 0 {
			t.Error("Expected default authorized packages")
		}

		if options.CodeBlockTags[0] != "<code>" || options.CodeBlockTags[1] != "</code>" {
			t.Errorf("Unexpected default code tags: %v", options.CodeBlockTags)
		}

		if !options.StreamOutputs {
			t.Error("Expected streaming to be enabled by default")
		}

		if options.MaxSteps != 15 {
			t.Errorf("Expected default max steps of 15, got %d", options.MaxSteps)
		}
	})

	t.Run("custom options", func(t *testing.T) {
		customPackages := []string{"fmt", "strings"}
		options := &ReactCodeAgentOptions{
			AuthorizedPackages: customPackages,
			CodeBlockTags:      [2]string{"[code]", "[/code]"},
			MaxCodeLength:      5000,
			MaxSteps:           10,
			Verbose:            true,
		}

		model := &ReactMockModel{responses: []string{}}
		agent, err := NewReactCodeAgent(model, nil, "", options)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		if len(agent.authorizedPackages) != len(customPackages) {
			t.Errorf("Expected %d authorized packages, got %d", len(customPackages), len(agent.authorizedPackages))
		}

		if agent.maxCodeLength != 5000 {
			t.Errorf("Expected max code length 5000, got %d", agent.maxCodeLength)
		}

		if agent.maxSteps != 10 {
			t.Errorf("Expected max steps 10, got %d", agent.maxSteps)
		}
	})
}

func TestReactCodeAgentIntegration(t *testing.T) {
	t.Skip("Skipping integration test - requires actual Go executor")

	// This test would require a real model and executor
	// It's here as a template for integration testing

	t.Run("real execution", func(t *testing.T) {
		// Would need a real model implementation
		// model := models.NewLocalModel(...)

		// agent, err := NewReactCodeAgent(model, nil, "", nil)
		// if err != nil {
		// 	t.Fatalf("Failed to create agent: %v", err)
		// }
		// defer agent.Close()

		// result, err := agent.Run(&RunOptions{
		// 	Task: "Calculate factorial of 5",
		// })
		// if err != nil {
		// 	t.Errorf("Agent run failed: %v", err)
		// }

		// outputStr := fmt.Sprintf("%v", result.Output)
		// if !strings.Contains(outputStr, "120") {
		// 	t.Errorf("Expected factorial of 5 (120) in output, got: %s", outputStr)
		// }
	})
}

func TestReactCodeAgentMemory(t *testing.T) {
	t.Run("memory accumulation", func(t *testing.T) {
		// Create mock model with responses that build on each other
		model := &ReactMockModel{
			responses: []string{
				`Thought: Let me set a variable x.
<code>
x := 10
fmt.Printf("Set x to %d\n", x)
</code>`,
				`Thought: Now let me use x in a calculation.
<code>
y := x * 2
fmt.Printf("y = %d\n", y)
</code>`,
				`Thought: Let me calculate the final result.
<code>
result := x + y
final_answer(fmt.Sprintf("x + y = %d", result))
</code>`,
			},
		}

		// Create agent
		agent, err := NewReactCodeAgent(model, nil, "", nil)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Run agent
		result, err := agent.Run(&RunOptions{
			Task: "Set x=10, calculate y=x*2, then return x+y",
		})
		if err != nil {
			t.Errorf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}

		// Check memory has all steps
		steps := agent.memory.GetSteps()
		if len(steps) < 4 { // Task + 3 action steps
			t.Errorf("Expected at least 4 steps in memory, got %d", len(steps))
		}

		// Verify conversation flow is preserved
		memoryStr := agent.getMemoryString()
		if !strings.Contains(memoryStr, "Set x to") {
			t.Error("Memory should contain first step output")
		}
	})
}

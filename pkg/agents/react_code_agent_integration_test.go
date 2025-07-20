package agents

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/executors"
	"github.com/rizome-dev/go-smolagents/pkg/models"
	"github.com/rizome-dev/go-smolagents/pkg/monitoring"
)

// TestReactCodeAgentIntegrationWithRealExecutor tests the full integration with real Go code execution
func TestReactCodeAgentIntegrationWithRealExecutor(t *testing.T) {
	// Create a mock model that provides realistic ReAct-style responses
	model := &IntegrationMockModel{
		responses: make(map[string]string),
	}

	// Set up responses for different tasks
	model.responses["What is 2 + 2?"] = `Thought: I need to calculate 2 + 2 using Go code.
<code>
result := 2 + 2
final_answer(result)
</code>`

	model.responses["Calculate the factorial of 5"] = `Thought: I need to implement a factorial function to calculate 5!.
<code>
factorial := func(n int) int {
    if n <= 1 {
        return 1
    }
    return n * factorial(n-1)
}

result := factorial(5)
final_answer(result)
</code>`

	model.responses["Write a Go function to check if a number is prime and test it with the number 17"] = `Thought: I'll create a function to check if a number is prime and test it with 17.
<code>
isPrime := func(n int) bool {
    if n < 2 {
        return false
    }
    if n == 2 {
        return true
    }
    if n%2 == 0 {
        return false
    }
    for i := 3; i*i <= n; i += 2 {
        if n%i == 0 {
            return false
        }
    }
    return true
}

result := isPrime(17)
fmt.Printf("Is 17 prime? %v\n", result)
final_answer(result)
</code>`

	// Test basic calculation
	t.Run("simple calculation with real executor", func(t *testing.T) {
		agent, err := NewReactCodeAgent(model, nil, "", nil)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := agent.Run(&RunOptions{
			Task:    "What is 2 + 2?",
			Context: ctx,
		})

		if err != nil {
			t.Fatalf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}

		// Check the actual result
		// Handle both int and float64 types due to JSON unmarshaling
		switch v := result.Output.(type) {
		case int:
			if v != 4 {
				t.Errorf("Expected output 4, got %v", v)
			}
		case float64:
			if int(v) != 4 {
				t.Errorf("Expected output 4, got %v", int(v))
			}
		default:
			t.Errorf("Expected output 4, got %v (type: %T)", result.Output, result.Output)
		}
	})

	// Test factorial calculation
	t.Run("factorial calculation with real executor", func(t *testing.T) {
		agent, err := NewReactCodeAgent(model, nil, "", nil)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := agent.Run(&RunOptions{
			Task:    "Calculate the factorial of 5",
			Context: ctx,
			Reset:   true,
		})

		if err != nil {
			t.Fatalf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}

		// Check the actual result
		// Handle both int and float64 types due to JSON unmarshaling
		switch v := result.Output.(type) {
		case int:
			if v != 120 {
				t.Errorf("Expected output 120 (5!), got %v", v)
			}
		case float64:
			if int(v) != 120 {
				t.Errorf("Expected output 120 (5!), got %v", int(v))
			}
		default:
			t.Errorf("Expected output 120 (5!), got %v (type: %T)", result.Output, result.Output)
		}
	})

	// Test prime number check
	t.Run("prime number check with real executor", func(t *testing.T) {
		options := &ReactCodeAgentOptions{
			AuthorizedPackages: []string{"fmt", "math", "strings"},
			MaxCodeLength:      100000, // Support large code blocks
			MaxSteps:           10,
			Verbose:            false, // Turn off verbose to avoid display issues
			CodeBlockTags:      [2]string{"<code>", "</code>"}, // Must set code block tags!
		}

		agent, err := NewReactCodeAgent(model, nil, "", options)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := agent.Run(&RunOptions{
			Task:    "Write a Go function to check if a number is prime and test it with the number 17",
			Context: ctx,
			Reset:   true,
		})

		if err != nil {
			t.Fatalf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}

		// Check the actual result (17 is prime)
		if result.Output != true {
			t.Errorf("Expected output true (17 is prime), got %v", result.Output)
		}
	})
}

// TestCodeLengthLimits tests that code length limits are properly enforced
func TestCodeLengthLimits(t *testing.T) {
	t.Run("code within limit is accepted", func(t *testing.T) {
		// Create a code block that's large but within limit
		var codeLines []string
		for i := 0; i < 100; i++ {
			codeLines = append(codeLines, fmt.Sprintf("var line%d = %d", i, i))
		}
		codeLines = append(codeLines, "final_answer(42)")
		largeCode := strings.Join(codeLines, "\n")

		model := &IntegrationMockModel{
			responses: map[string]string{
				"test": fmt.Sprintf(`Thought: Testing large code block.
<code>
%s
</code>`, largeCode),
			},
		}

		options := &ReactCodeAgentOptions{
			AuthorizedPackages: executors.DefaultAuthorizedPackages(),
			MaxCodeLength:      100000, // 100k chars
			MaxSteps:           5,
			CodeBlockTags:      [2]string{"<code>", "</code>"},
		}

		agent, err := NewReactCodeAgent(model, nil, "", options)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		result, err := agent.Run(&RunOptions{
			Task: "test",
		})

		if err != nil {
			t.Fatalf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}
	})

	t.Run("code exceeding limit is rejected", func(t *testing.T) {
		// Create a code block that exceeds the limit
		var codeLines []string
		for i := 0; i < 200; i++ {
			codeLines = append(codeLines, fmt.Sprintf("var veryLongVariableName%d = \"This is a very long string to make the code exceed the character limit quickly\"", i))
		}
		veryLargeCode := strings.Join(codeLines, "\n")

		model := &IntegrationMockModel{
			responses: map[string]string{
				"test": fmt.Sprintf(`Thought: Testing very large code block.
<code>
%s
</code>`, veryLargeCode),
			},
		}

		options := &ReactCodeAgentOptions{
			AuthorizedPackages: executors.DefaultAuthorizedPackages(),
			MaxCodeLength:      1000, // Small limit for testing
			MaxSteps:           5,
			CodeBlockTags:      [2]string{"<code>", "</code>"},
		}

		agent, err := NewReactCodeAgent(model, nil, "", options)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		result, err := agent.Run(&RunOptions{
			Task: "test",
		})

		// The agent should handle the error gracefully
		if result.State == "success" {
			t.Error("Expected non-success state due to code length limit")
		}
	})

	t.Run("zero MaxCodeLength uses default", func(t *testing.T) {
		model := &IntegrationMockModel{
			responses: map[string]string{
				"test": `Thought: Testing default code length.
<code>
result := 42
final_answer(result)
</code>`,
			},
		}

		// Create options with MaxCodeLength = 0
		options := &ReactCodeAgentOptions{
			AuthorizedPackages: executors.DefaultAuthorizedPackages(),
			MaxCodeLength:      0, // Should use default
			MaxSteps:           5,
			CodeBlockTags:      [2]string{"<code>", "</code>"},
		}

		agent, err := NewReactCodeAgent(model, nil, "", options)
		if err != nil {
			t.Fatalf("Failed to create agent: %v", err)
		}
		defer agent.Close()

		// Check that maxCodeLength was set to default
		if agent.maxCodeLength != 100000 {
			t.Errorf("Expected maxCodeLength to be 100000, got %d", agent.maxCodeLength)
		}

		result, err := agent.Run(&RunOptions{
			Task: "test",
		})

		if err != nil {
			t.Fatalf("Agent run failed: %v", err)
		}

		if result.State != "success" {
			t.Errorf("Expected success state, got %s", result.State)
		}
	})
}

// IntegrationMockModel is a mock model for integration testing
type IntegrationMockModel struct {
	responses map[string]string
}

func (m *IntegrationMockModel) Generate(messages []interface{}, options *models.GenerateOptions) (*models.ChatMessage, error) {
	// Extract the task from messages
	var task string
	for _, msg := range messages {
		if msgMap, ok := msg.(map[string]interface{}); ok {
			if role, ok := msgMap["role"].(string); ok && role == "user" {
				if content, ok := msgMap["content"].(string); ok {
					if strings.Contains(content, "Task:") {
						// Extract task from content
						lines := strings.Split(content, "\n")
						for _, line := range lines {
							if strings.HasPrefix(line, "Task:") {
								task = strings.TrimSpace(strings.TrimPrefix(line, "Task:"))
								break
							}
						}
					}
				}
			}
		}
	}

	// Return appropriate response based on task
	response, exists := m.responses[task]
	if !exists {
		response = "I don't know how to handle this task."
	}

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

func (m *IntegrationMockModel) GetModelID() string {
	return "integration-mock-model"
}

func (m *IntegrationMockModel) ParseToolCalls(message *models.ChatMessage) (*models.ChatMessage, error) {
	return message, nil
}

func (m *IntegrationMockModel) StreamGenerate(messages []interface{}, options *models.GenerateOptions) (<-chan *models.ChatMessageStreamDelta, error) {
	return nil, fmt.Errorf("streaming not supported")
}

func (m *IntegrationMockModel) GenerateStream(messages []interface{}, options *models.GenerateOptions) (<-chan *models.ChatMessageStreamDelta, error) {
	return nil, fmt.Errorf("streaming not supported")
}

func (m *IntegrationMockModel) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"model_id": "integration-mock-model",
		"type":     "mock",
	}
}

func (m *IntegrationMockModel) SupportsStreaming() bool {
	return false
}

func (m *IntegrationMockModel) Close() error {
	return nil
}
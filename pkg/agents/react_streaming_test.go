package agents

import (
	"testing"
	"time"
	
	"github.com/rizome-dev/go-smolagents/pkg/models"
	"github.com/rizome-dev/go-smolagents/pkg/tools"
)

// MockStreamingModel implements a mock model that supports streaming
type MockStreamingModel struct {
	*models.BaseModel
}

func NewMockStreamingModel() *MockStreamingModel {
	return &MockStreamingModel{
		BaseModel: &models.BaseModel{ModelID: "mock-streaming"},
	}
}

func (m *MockStreamingModel) Generate(messages []interface{}, options *models.GenerateOptions) (*models.ChatMessage, error) {
	content := "Thought: I need to calculate something\n<code>\nresult = 42\nprint(result)\n</code>"
	return models.NewChatMessage(string(models.RoleAssistant), content), nil
}

func (m *MockStreamingModel) GenerateStream(messages []interface{}, options *models.GenerateOptions) (<-chan *models.ChatMessageStreamDelta, error) {
	streamChan := make(chan *models.ChatMessageStreamDelta, 10)
	
	go func() {
		defer close(streamChan)
		
		// Simulate streaming response
		chunks := []string{
			"Thought: ",
			"I need to ",
			"calculate something\n",
			"<code>\n",
			"result = 42\n",
			"print(result)\n",
			"</code>",
		}
		
		for _, chunk := range chunks {
			streamChan <- &models.ChatMessageStreamDelta{
				Content: &chunk,
			}
			time.Sleep(10 * time.Millisecond) // Simulate network delay
		}
	}()
	
	return streamChan, nil
}

func (m *MockStreamingModel) SupportsStreaming() bool {
	return true
}

func TestReactCodeAgent_RunStream(t *testing.T) {
	// Create a mock streaming model
	model := NewMockStreamingModel()
	
	// Create agent with streaming enabled
	options := DefaultReactCodeAgentOptions()
	options.StreamOutputs = true
	options.MaxSteps = 1
	
	agent, err := NewReactCodeAgent(model, []tools.Tool{}, "", options)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	
	// Run with streaming
	runOptions := &RunOptions{
		Task: "Calculate the answer to everything",
		Reset: true,
	}
	
	streamChan, err := agent.RunStream(runOptions)
	if err != nil {
		t.Fatalf("Failed to start streaming: %v", err)
	}
	
	// Collect stream events
	var events []*StreamStepResult
	for event := range streamChan {
		events = append(events, event)
	}
	
	// Verify we got events
	if len(events) == 0 {
		t.Error("Expected stream events but got none")
	}
	
	// Check for different event types
	hasStepStart := false
	hasStreamDelta := false
	hasThought := false
	hasCode := false
	hasObservation := false
	hasFinal := false
	
	for _, event := range events {
		switch event.StepType {
		case "step_start":
			hasStepStart = true
		case "stream_delta":
			hasStreamDelta = true
		case "thought":
			hasThought = true
		case "code":
			hasCode = true
		case "observation":
			hasObservation = true
		case "final":
			hasFinal = true
		}
	}
	
	if !hasStepStart {
		t.Error("Expected step_start event")
	}
	if !hasStreamDelta {
		t.Error("Expected stream_delta events")
	}
	if !hasThought {
		t.Error("Expected thought event")
	}
	if !hasCode {
		t.Error("Expected code event")
	}
	if !hasObservation {
		t.Error("Expected observation event")
	}
	if !hasFinal {
		t.Error("Expected final event")
	}
	
	// Verify final event
	var finalEvent *StreamStepResult
	for _, event := range events {
		if event.StepType == "final" {
			finalEvent = event
			break
		}
	}
	
	if finalEvent == nil {
		t.Fatal("No final event found")
	}
	
	if !finalEvent.IsComplete {
		t.Error("Final event should have IsComplete=true")
	}
}

func TestReactCodeAgent_RunStream_NonStreamingModel(t *testing.T) {
	// Create a non-streaming model
	model := &ReactMockModel{
		responses: []string{
			"Thought: Simple calculation\n<code>\nresult = 2 + 2\nprint(result)\n</code>",
		},
	}
	
	// Create agent
	options := DefaultReactCodeAgentOptions()
	options.MaxSteps = 1
	
	agent, err := NewReactCodeAgent(model, []tools.Tool{}, "", options)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	
	// Run with streaming (should fall back to non-streaming)
	runOptions := &RunOptions{
		Task: "Calculate 2+2",
		Reset: true,
	}
	
	streamChan, err := agent.RunStream(runOptions)
	if err != nil {
		t.Fatalf("Failed to start streaming: %v", err)
	}
	
	// Collect events
	var events []*StreamStepResult
	for event := range streamChan {
		events = append(events, event)
	}
	
	// Should still get events even without streaming support
	if len(events) == 0 {
		t.Error("Expected events even for non-streaming model")
	}
	
	// Check for model_output event (non-streaming path)
	hasModelOutput := false
	for _, event := range events {
		if event.StepType == "model_output" {
			hasModelOutput = true
			break
		}
	}
	
	if !hasModelOutput {
		t.Error("Expected model_output event for non-streaming model")
	}
}
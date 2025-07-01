package agents

import (
	"testing"

	"github.com/rizome-dev/go-smolagents/pkg/models"
	"github.com/rizome-dev/go-smolagents/pkg/tools"
)

// MockModel for testing
type MockModel struct{}

func (m *MockModel) Generate(messages []interface{}, options *models.GenerateOptions) (*models.ChatMessage, error) {
	return &models.ChatMessage{
		Role:    "assistant",
		Content: func() *string { s := "Test response"; return &s }(),
	}, nil
}

func (m *MockModel) GenerateStream(messages []interface{}, options *models.GenerateOptions) (<-chan *models.ChatMessageStreamDelta, error) {
	ch := make(chan *models.ChatMessageStreamDelta, 1)
	close(ch)
	return ch, nil
}

func (m *MockModel) GetModelID() string {
	return "mock-model"
}

func (m *MockModel) SupportsToolCalling() bool {
	return true
}

func (m *MockModel) Close() error {
	return nil
}

func (m *MockModel) ParseToolCalls(message *models.ChatMessage) (*models.ChatMessage, error) {
	return message, nil
}

func (m *MockModel) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"model_id": "mock-model",
		"type":     "mock",
	}
}

func (m *MockModel) SupportsStreaming() bool {
	return true
}

func (m *MockModel) GetMaxTokens() int {
	return 4096
}

func (m *MockModel) SetMaxTokens(maxTokens int) {}

func TestNewCodeAgent(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}

	agent, err := NewCodeAgent(model, tools, "", nil)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected non-nil agent")
	}

	if agent.goInterpreter == nil {
		t.Error("Expected Go interpreter to be set")
	}

	if len(agent.authorizedPackages) == 0 {
		t.Error("Expected default authorized packages")
	}
}

func TestCodeAgentWithOptions(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}
	options := map[string]interface{}{
		"authorized_packages": []string{"fmt", "math"},
		"max_code_length":     5000,
		"stream_outputs":      false,
		"structured_output":   true,
	}

	agent, err := NewCodeAgent(model, tools, "", options)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent with options: %v", err)
	}

	if len(agent.authorizedPackages) != 2 {
		t.Errorf("Expected 2 authorized packages, got %d", len(agent.authorizedPackages))
	}

	if agent.maxCodeLength != 5000 {
		t.Errorf("Expected max code length 5000, got %d", agent.maxCodeLength)
	}

	if agent.streamOutputs != false {
		t.Error("Expected stream outputs to be false")
	}

	if agent.structuredOutput != true {
		t.Error("Expected structured output to be true")
	}
}

func TestNewCodeAgentSimple(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}

	agent, err := NewCodeAgentSimple(tools, model)
	if err != nil {
		t.Fatalf("Failed to create simple CodeAgent: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected non-nil agent")
	}
}

func TestCodeAgentSystemPrompt(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}
	customPrompt := "Custom system prompt"

	agent, err := NewCodeAgent(model, tools, customPrompt, nil)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	if agent.GetSystemPrompt() != customPrompt {
		t.Error("Custom system prompt not set correctly")
	}
}

func TestCodeAgentAuthorizedPackages(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}

	agent, err := NewCodeAgent(model, tools, "", nil)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	// Test getter
	packages := agent.GetAuthorizedPackages()
	if len(packages) == 0 {
		t.Error("Expected non-empty authorized packages")
	}

	// Test setter
	newPackages := []string{"fmt", "math", "strings"}
	agent.SetAuthorizedPackages(newPackages)

	if len(agent.GetAuthorizedPackages()) != 3 {
		t.Error("Authorized packages not updated correctly")
	}
}

func TestCodeAgentStreamOutputs(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}

	agent, err := NewCodeAgent(model, tools, "", nil)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	// Test default value
	if !agent.GetStreamOutputs() {
		t.Error("Expected default stream outputs to be true")
	}

	// Test setter
	agent.SetStreamOutputs(false)
	if agent.GetStreamOutputs() {
		t.Error("Stream outputs not updated correctly")
	}
}

func TestCodeAgentStructuredOutput(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}

	agent, err := NewCodeAgent(model, tools, "", nil)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	// Test default value
	if agent.GetStructuredOutput() {
		t.Error("Expected default structured output to be false")
	}

	// Test setter
	agent.SetStructuredOutput(true)
	if !agent.GetStructuredOutput() {
		t.Error("Structured output not updated correctly")
	}
}

func TestCodeAgentMaxCodeLength(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}

	agent, err := NewCodeAgent(model, tools, "", nil)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	// Test default value
	if agent.GetMaxCodeLength() != 10000 {
		t.Errorf("Expected default max code length 10000, got %d", agent.GetMaxCodeLength())
	}

	// Test setter
	agent.SetMaxCodeLength(5000)
	if agent.GetMaxCodeLength() != 5000 {
		t.Error("Max code length not updated correctly")
	}
}

func TestCodeAgentToDict(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}

	agent, err := NewCodeAgent(model, tools, "", nil)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	dict := agent.ToDict()

	if dict["agent_type"] != "code" {
		t.Error("Agent type not set correctly in ToDict")
	}

	if dict["authorized_packages"] == nil {
		t.Error("Authorized packages not included in ToDict")
	}

	if dict["stream_outputs"] == nil {
		t.Error("Stream outputs not included in ToDict")
	}

	if dict["structured_output"] == nil {
		t.Error("Structured output not included in ToDict")
	}

	if dict["max_code_length"] == nil {
		t.Error("Max code length not included in ToDict")
	}
}

func TestCodeAgentExecuteCode(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}

	agent, err := NewCodeAgent(model, tools, "", nil)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	// Test code execution
	result, err := agent.ExecuteCode("result := 2 + 3")
	if err != nil {
		t.Errorf("Failed to execute code: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestCodeAgentCodeTooLong(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}
	options := map[string]interface{}{
		"max_code_length": 10,
	}

	agent, err := NewCodeAgent(model, tools, "", options)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	// Test code that's too long
	longCode := "This is a very long code snippet that exceeds the maximum length"
	_, err = agent.ExecuteCode(longCode)
	if err == nil {
		t.Error("Expected error for code that's too long")
	}
}

func TestCodeAgentClone(t *testing.T) {
	model := &MockModel{}
	tools := []tools.Tool{}

	original, err := NewCodeAgent(model, tools, "", nil)
	if err != nil {
		t.Fatalf("Failed to create CodeAgent: %v", err)
	}

	clone, err := original.Clone()
	if err != nil {
		t.Fatalf("Failed to clone CodeAgent: %v", err)
	}

	if clone == nil {
		t.Fatal("Expected non-nil clone")
	}

	// Verify that clone has same configuration
	if len(clone.GetAuthorizedPackages()) != len(original.GetAuthorizedPackages()) {
		t.Error("Clone doesn't have same authorized packages")
	}

	if clone.GetMaxCodeLength() != original.GetMaxCodeLength() {
		t.Error("Clone doesn't have same max code length")
	}
}

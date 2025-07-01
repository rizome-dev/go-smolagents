package agents

import (
	"testing"

	"github.com/rizome-dev/go-smolagents/pkg/tools"
)

func TestNewToolCallingAgent(t *testing.T) {
	model := &MockModel{}
	toolList := []tools.Tool{}

	agent, err := NewToolCallingAgent(model, toolList, "", nil)
	if err != nil {
		t.Fatalf("Failed to create ToolCallingAgent: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected non-nil agent")
	}

	if agent.finalAnswerTool == nil {
		t.Error("Expected final answer tool to be set")
	}
}

func TestNewToolCallingAgentSimple(t *testing.T) {
	model := &MockModel{}
	toolList := []tools.Tool{}

	agent, err := NewToolCallingAgentSimple(toolList, model)
	if err != nil {
		t.Fatalf("Failed to create simple ToolCallingAgent: %v", err)
	}

	if agent == nil {
		t.Fatal("Expected non-nil agent")
	}
}

func TestToolCallingAgentSystemPrompt(t *testing.T) {
	model := &MockModel{}
	toolList := []tools.Tool{}
	customPrompt := "Custom tool calling prompt"

	agent, err := NewToolCallingAgent(model, toolList, customPrompt, nil)
	if err != nil {
		t.Fatalf("Failed to create ToolCallingAgent: %v", err)
	}

	if agent.GetSystemPrompt() != customPrompt {
		t.Error("Custom system prompt not set correctly")
	}
}

func TestToolCallingAgentToDict(t *testing.T) {
	model := &MockModel{}
	toolList := []tools.Tool{}

	agent, err := NewToolCallingAgent(model, toolList, "", nil)
	if err != nil {
		t.Fatalf("Failed to create ToolCallingAgent: %v", err)
	}

	dict := agent.ToDict()

	if dict["agent_type"] != "tool_calling" {
		t.Error("Agent type not set correctly in ToDict")
	}
}

func TestToolCallingAgentGetAvailableTools(t *testing.T) {
	model := &MockModel{}
	toolList := []tools.Tool{}

	agent, err := NewToolCallingAgent(model, toolList, "", nil)
	if err != nil {
		t.Fatalf("Failed to create ToolCallingAgent: %v", err)
	}

	availableTools := agent.GetAvailableTools()

	// Should at least have the final answer tool
	if len(availableTools) == 0 {
		t.Error("Expected at least one available tool")
	}

	// Check for final answer tool
	found := false
	for _, toolName := range availableTools {
		if toolName == "final_answer" {
			found = true
			break
		}
	}

	if !found {
		t.Error("final_answer tool not found in available tools")
	}
}

func TestToolCallingAgentClone(t *testing.T) {
	model := &MockModel{}
	toolList := []tools.Tool{}

	original, err := NewToolCallingAgent(model, toolList, "", nil)
	if err != nil {
		t.Fatalf("Failed to create ToolCallingAgent: %v", err)
	}

	clone, err := original.Clone()
	if err != nil {
		t.Fatalf("Failed to clone ToolCallingAgent: %v", err)
	}

	if clone == nil {
		t.Fatal("Expected non-nil clone")
	}

	// Verify that clone has same number of tools
	if len(clone.GetTools()) != len(original.GetTools()) {
		t.Error("Clone doesn't have same number of tools")
	}
}

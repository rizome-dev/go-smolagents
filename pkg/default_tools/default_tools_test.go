package default_tools

import (
	"testing"

	"github.com/rizome-dev/go-smolagents/pkg/agent_types"
)

func TestNewGoInterpreterTool(t *testing.T) {
	tool := NewGoInterpreterTool()

	if tool == nil {
		t.Fatal("Expected non-nil tool")
	}

	if tool.GetName() != "go_interpreter" {
		t.Errorf("Expected name 'go_interpreter', got '%s'", tool.GetName())
	}

	if len(tool.AuthorizedPackages) == 0 {
		t.Error("Expected default authorized packages")
	}
}

func TestGoInterpreterToolWithCustomPackages(t *testing.T) {
	customPackages := []string{"fmt", "math"}
	tool := NewGoInterpreterTool(customPackages)

	if len(tool.AuthorizedPackages) != 2 {
		t.Errorf("Expected 2 packages, got %d", len(tool.AuthorizedPackages))
	}

	if tool.AuthorizedPackages[0] != "fmt" || tool.AuthorizedPackages[1] != "math" {
		t.Error("Custom packages not set correctly")
	}
}

func TestGoInterpreterToolExecution(t *testing.T) {
	tool := NewGoInterpreterTool()

	// Test simple code execution
	result, err := tool.Call("result := 2 + 3")
	if err != nil {
		t.Errorf("Failed to execute code: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestGoInterpreterToolValidation(t *testing.T) {
	tool := NewGoInterpreterTool()

	err := tool.Validate()
	if err != nil {
		t.Errorf("Tool validation failed: %v", err)
	}
}

func TestFinalAnswerTool(t *testing.T) {
	tool := NewFinalAnswerTool()

	if tool.GetName() != "final_answer" {
		t.Errorf("Expected name 'final_answer', got '%s'", tool.GetName())
	}

	// Test forward function
	result, err := tool.Call("test answer")
	if err != nil {
		t.Errorf("FinalAnswerTool failed: %v", err)
	}

	// Result should be an AgentText containing our answer
	if agentText, ok := result.(*agent_types.AgentText); ok {
		if agentText.ToString() != "test answer" {
			t.Errorf("Expected 'test answer', got '%v'", agentText.ToString())
		}
	} else {
		t.Errorf("Expected result to be AgentText, got %T: %v", result, result)
	}
}

func TestWebSearchTool(t *testing.T) {
	tool := NewWebSearchTool()

	if tool.GetName() != "web_search" {
		t.Errorf("Expected name 'web_search', got '%s'", tool.GetName())
	}

	if tool.Engine != "duckduckgo" {
		t.Errorf("Expected default engine 'duckduckgo', got '%s'", tool.Engine)
	}
}

func TestVisitWebpageTool(t *testing.T) {
	tool := NewVisitWebpageTool()

	if tool.GetName() != "visit_webpage" {
		t.Errorf("Expected name 'visit_webpage', got '%s'", tool.GetName())
	}

	if tool.MaxOutputLength != 40000 {
		t.Errorf("Expected default max output length 40000, got %d", tool.MaxOutputLength)
	}
}

func TestWikipediaSearchTool(t *testing.T) {
	tool := NewWikipediaSearchTool()

	if tool.GetName() != "wikipedia_search" {
		t.Errorf("Expected name 'wikipedia_search', got '%s'", tool.GetName())
	}

	if tool.Language != "en" {
		t.Errorf("Expected default language 'en', got '%s'", tool.Language)
	}
}

func TestUserInputTool(t *testing.T) {
	tool := NewUserInputTool()

	if tool.GetName() != "user_input" {
		t.Errorf("Expected name 'user_input', got '%s'", tool.GetName())
	}
}

func TestToolMapping(t *testing.T) {
	expectedTools := []string{
		"go_interpreter",
		"final_answer",
		"user_input",
		"web_search",
		"visit_webpage",
		"wikipedia_search",
	}

	for _, toolName := range expectedTools {
		if _, exists := ToolMapping[toolName]; !exists {
			t.Errorf("Tool '%s' not found in ToolMapping", toolName)
		}
	}
}

func TestGetToolByName(t *testing.T) {
	tool, err := GetToolByName("go_interpreter")
	if err != nil {
		t.Errorf("Failed to get tool by name: %v", err)
	}

	if tool.GetName() != "go_interpreter" {
		t.Errorf("Expected 'go_interpreter', got '%s'", tool.GetName())
	}

	// Test unknown tool
	_, err = GetToolByName("unknown_tool")
	if err == nil {
		t.Error("Expected error for unknown tool")
	}
}

func TestListAvailableTools(t *testing.T) {
	tools := ListAvailableTools()

	if len(tools) == 0 {
		t.Error("Expected non-empty tool list")
	}

	// Check that go_interpreter is in the list
	found := false
	for _, tool := range tools {
		if tool == "go_interpreter" {
			found = true
			break
		}
	}

	if !found {
		t.Error("go_interpreter not found in available tools")
	}
}

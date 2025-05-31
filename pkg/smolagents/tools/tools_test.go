package tools

import (
	"testing"
	
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/agent_types"
)

// MockTool for testing
type MockTool struct {
	*BaseTool
	callCount int
}

func NewMockTool() *MockTool {
	inputs := map[string]*ToolInput{
		"message": NewToolInput("string", "A test message"),
	}
	
	baseTool := NewBaseTool(
		"mock_tool",
		"A mock tool for testing",
		inputs,
		"string",
	)
	
	tool := &MockTool{
		BaseTool:  baseTool,
		callCount: 0,
	}
	
	tool.ForwardFunc = tool.forward
	return tool
}

func (mt *MockTool) forward(args ...interface{}) (interface{}, error) {
	mt.callCount++
	if len(args) > 0 {
		return args[0], nil
	}
	return "mock result", nil
}

func TestNewBaseTool(t *testing.T) {
	inputs := map[string]*ToolInput{
		"test": NewToolInput("string", "Test input"),
	}
	
	tool := NewBaseTool("test_tool", "A test tool", inputs, "string")
	
	if tool.GetName() != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", tool.GetName())
	}
	
	if tool.GetDescription() != "A test tool" {
		t.Errorf("Expected description 'A test tool', got '%s'", tool.GetDescription())
	}
	
	if tool.GetOutputType() != "string" {
		t.Errorf("Expected output type 'string', got '%s'", tool.GetOutputType())
	}
}

func TestNewToolInput(t *testing.T) {
	input := NewToolInput("string", "Test description")
	
	if input.Type != "string" {
		t.Errorf("Expected type 'string', got '%s'", input.Type)
	}
	
	if input.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", input.Description)
	}
}

func TestToolInputToDict(t *testing.T) {
	input := NewToolInput("integer", "An integer value")
	input.Required = true
	input.Default = 42
	
	dict := input.ToDict()
	
	if dict["type"] != "integer" {
		t.Error("Type not converted correctly")
	}
	
	if dict["description"] != "An integer value" {
		t.Error("Description not converted correctly")
	}
	
	if dict["required"] != true {
		t.Error("Required not converted correctly")
	}
	
	if dict["default"] != 42 {
		t.Error("Default not converted correctly")
	}
}

func TestBaseToolValidate(t *testing.T) {
	tool := NewMockTool()
	
	err := tool.Validate()
	if err != nil {
		t.Errorf("Tool validation failed: %v", err)
	}
}

func TestBaseToolCall(t *testing.T) {
	tool := NewMockTool()
	
	result, err := tool.Call("test message")
	if err != nil {
		t.Errorf("Tool call failed: %v", err)
	}
	
	// Result should be an AgentText containing our message
	if agentText, ok := result.(*agent_types.AgentText); ok {
		if agentText.ToString() != "test message" {
			t.Errorf("Expected 'test message', got '%v'", agentText.ToString())
		}
	} else {
		t.Errorf("Expected result to be AgentText, got %T", result)
	}
	
	if tool.callCount != 1 {
		t.Errorf("Expected call count 1, got %d", tool.callCount)
	}
}

func TestBaseToolCallWithMapArgs(t *testing.T) {
	tool := NewMockTool()
	
	args := map[string]interface{}{
		"message": "map test",
	}
	
	result, err := tool.Call(args)
	if err != nil {
		t.Errorf("Tool call with map args failed: %v", err)
	}
	
	// Result should be an AgentText containing our message
	if agentText, ok := result.(*agent_types.AgentText); ok {
		if agentText.ToString() != "map test" {
			t.Errorf("Expected 'map test', got '%v'", agentText.ToString())
		}
	} else {
		t.Errorf("Expected result to be AgentText, got %T", result)
	}
}

func TestBaseToolForward(t *testing.T) {
	tool := NewMockTool()
	
	result, err := tool.Forward("forward test")
	if err != nil {
		t.Errorf("Tool forward failed: %v", err)
	}
	
	if result != "forward test" {
		t.Errorf("Expected 'forward test', got '%v'", result)
	}
}

func TestBaseToolToDict(t *testing.T) {
	tool := NewMockTool()
	
	dict := tool.ToDict()
	
	if dict["name"] != "mock_tool" {
		t.Error("Name not included in ToDict")
	}
	
	if dict["description"] != "A mock tool for testing" {
		t.Error("Description not included in ToDict")
	}
	
	if dict["inputs"] == nil {
		t.Error("Inputs not included in ToDict")
	}
	
	if dict["output_type"] != "string" {
		t.Error("Output type not included in ToDict")
	}
}

func TestBaseToolToOpenAITool(t *testing.T) {
	tool := NewMockTool()
	
	openAITool := tool.ToOpenAITool()
	
	if openAITool["type"] != "function" {
		t.Error("Type not set correctly for OpenAI tool")
	}
	
	if openAITool["function"] == nil {
		t.Error("Function not included in OpenAI tool")
	}
	
	function := openAITool["function"].(map[string]interface{})
	if function["name"] != "mock_tool" {
		t.Error("Function name not set correctly")
	}
}

func TestBaseToolGetInputs(t *testing.T) {
	tool := NewMockTool()
	
	inputs := tool.GetInputs()
	
	if len(inputs) != 1 {
		t.Errorf("Expected 1 input, got %d", len(inputs))
	}
	
	if inputs["message"] == nil {
		t.Error("Expected 'message' input not found")
	}
}

func TestBaseToolGetInput(t *testing.T) {
	tool := NewMockTool()
	
	input := tool.GetInput("message")
	if input == nil {
		t.Error("Expected non-nil input")
	}
	
	if input.Type != "string" {
		t.Error("Input type not correct")
	}
	
	// Test non-existent input
	nonExistent := tool.GetInput("nonexistent")
	if nonExistent != nil {
		t.Error("Expected nil for non-existent input")
	}
}

func TestBaseToolSetupAndTeardown(t *testing.T) {
	tool := NewMockTool()
	
	err := tool.Setup()
	if err != nil {
		t.Errorf("Tool setup failed: %v", err)
	}
	
	err = tool.Teardown()
	if err != nil {
		t.Errorf("Tool teardown failed: %v", err)
	}
}

func TestNewToolCollection(t *testing.T) {
	tools := []Tool{
		NewMockTool(),
	}
	
	collection := NewToolCollection(tools)
	
	if len(collection.Tools) != 1 {
		t.Errorf("Expected 1 tool in collection, got %d", len(collection.Tools))
	}
	
	if len(collection.ToolsMap) != 1 {
		t.Errorf("Expected 1 tool in map, got %d", len(collection.ToolsMap))
	}
}

func TestToolCollectionGetTool(t *testing.T) {
	tools := []Tool{
		NewMockTool(),
	}
	
	collection := NewToolCollection(tools)
	
	tool, exists := collection.GetTool("mock_tool")
	if !exists {
		t.Error("Expected tool to exist")
	}
	
	if tool.GetName() != "mock_tool" {
		t.Error("Wrong tool returned")
	}
	
	// Test non-existent tool
	_, exists = collection.GetTool("nonexistent")
	if exists {
		t.Error("Expected tool to not exist")
	}
}

func TestToolCollectionAddTool(t *testing.T) {
	collection := NewToolCollection([]Tool{})
	
	tool := NewMockTool()
	err := collection.AddTool(tool)
	if err != nil {
		t.Errorf("Failed to add tool: %v", err)
	}
	
	if len(collection.Tools) != 1 {
		t.Error("Tool not added to collection")
	}
	
	// Test adding duplicate tool
	err = collection.AddTool(tool)
	if err == nil {
		t.Error("Expected error when adding duplicate tool")
	}
}

func TestToolCollectionRemoveTool(t *testing.T) {
	tools := []Tool{
		NewMockTool(),
	}
	
	collection := NewToolCollection(tools)
	
	removed := collection.RemoveTool("mock_tool")
	if !removed {
		t.Error("Expected tool to be removed")
	}
	
	if len(collection.Tools) != 0 {
		t.Error("Tool not removed from collection")
	}
	
	// Test removing non-existent tool
	removed = collection.RemoveTool("nonexistent")
	if removed {
		t.Error("Expected false for non-existent tool")
	}
}

func TestToolCollectionListTools(t *testing.T) {
	tools := []Tool{
		NewMockTool(),
	}
	
	collection := NewToolCollection(tools)
	
	toolNames := collection.ListTools()
	if len(toolNames) != 1 {
		t.Errorf("Expected 1 tool name, got %d", len(toolNames))
	}
	
	if toolNames[0] != "mock_tool" {
		t.Error("Wrong tool name returned")
	}
}

func TestToolCollectionValidateAll(t *testing.T) {
	tools := []Tool{
		NewMockTool(),
	}
	
	collection := NewToolCollection(tools)
	
	err := collection.ValidateAll()
	if err != nil {
		t.Errorf("Tool collection validation failed: %v", err)
	}
}
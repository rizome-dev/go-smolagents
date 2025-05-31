package memory

import (
	"testing"
	"time"

	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/models"
)

func TestNewMessage(t *testing.T) {
	message := NewMessage(models.RoleUser, "Test content")
	
	if message.Role != models.RoleUser {
		t.Errorf("Expected role %s, got %s", models.RoleUser, message.Role)
	}
	
	if len(message.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(message.Content))
	}
	
	if message.Content[0]["text"] != "Test content" {
		t.Error("Message content not set correctly")
	}
}

func TestMessageToDict(t *testing.T) {
	message := NewMessage(models.RoleAssistant, "Test response")
	
	dict := message.ToDict()
	
	if dict["role"] != string(models.RoleAssistant) {
		t.Error("Role not converted correctly in ToDict")
	}
	
	if dict["content"] == nil {
		t.Error("Content not included in ToDict")
	}
}

func TestToolCall(t *testing.T) {
	toolCall := &ToolCall{
		ID:   "test-id",
		Name: "test-tool",
		Arguments: map[string]interface{}{
			"arg1": "value1",
			"arg2": 42,
		},
	}
	
	dict := toolCall.ToDict()
	
	if dict["id"] != "test-id" {
		t.Error("ID not converted correctly")
	}
	
	if dict["name"] != "test-tool" {
		t.Error("Name not converted correctly")
	}
	
	if dict["arguments"] == nil {
		t.Error("Arguments not included")
	}
}

func TestNewActionStep(t *testing.T) {
	startTime := time.Now()
	step := NewActionStep(1, startTime)
	
	if step.StepNumber != 1 {
		t.Errorf("Expected step number 1, got %d", step.StepNumber)
	}
	
	if step.Timing.StartTime != startTime {
		t.Error("Start time not set correctly")
	}
}

func TestActionStepToMessages(t *testing.T) {
	step := &ActionStep{
		StepNumber:   1,
		ModelOutput:  "Test output",
		Observations: "Test observation",
	}
	
	messages, err := step.ToMessages(false)
	if err != nil {
		t.Errorf("Failed to convert step to messages: %v", err)
	}
	
	if len(messages) != 2 { // One for model output, one for observations
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
}

func TestActionStepToDict(t *testing.T) {
	step := NewActionStep(1)
	step.ModelOutput = "Test output"
	
	dict, err := step.ToDict()
	if err != nil {
		t.Errorf("Failed to convert step to dict: %v", err)
	}
	
	if dict["type"] != "action" {
		t.Error("Type not set correctly")
	}
	
	if dict["step_number"] != 1 {
		t.Error("Step number not set correctly")
	}
}

func TestNewTaskStep(t *testing.T) {
	step := NewTaskStep("Test task", nil)
	
	if step.Task != "Test task" {
		t.Errorf("Expected task 'Test task', got '%s'", step.Task)
	}
	
	if step.GetType() != "task" {
		t.Errorf("Expected type 'task', got '%s'", step.GetType())
	}
}

func TestTaskStepToMessages(t *testing.T) {
	step := NewTaskStep("Complete this task", nil)
	
	messages, err := step.ToMessages(false)
	if err != nil {
		t.Errorf("Failed to convert task step to messages: %v", err)
	}
	
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	
	if messages[0].Role != models.RoleUser {
		t.Error("Task step should create user message")
	}
}

func TestNewSystemPromptStep(t *testing.T) {
	step := NewSystemPromptStep("System prompt")
	
	if step.SystemPrompt != "System prompt" {
		t.Error("System prompt not set correctly")
	}
	
	if step.GetType() != "system_prompt" {
		t.Errorf("Expected type 'system_prompt', got '%s'", step.GetType())
	}
}

func TestSystemPromptStepToMessages(t *testing.T) {
	step := NewSystemPromptStep("You are a helpful assistant")
	
	messages, err := step.ToMessages(false)
	if err != nil {
		t.Errorf("Failed to convert system prompt step to messages: %v", err)
	}
	
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	
	if messages[0].Role != models.RoleSystem {
		t.Error("System prompt step should create system message")
	}
}

func TestNewFinalAnswerStep(t *testing.T) {
	step := NewFinalAnswerStep("Final answer")
	
	if step.Output != "Final answer" {
		t.Error("Output not set correctly")
	}
	
	if step.GetType() != "final_answer" {
		t.Errorf("Expected type 'final_answer', got '%s'", step.GetType())
	}
}

func TestNewAgentMemory(t *testing.T) {
	memory := NewAgentMemory("System prompt")
	
	if memory.SystemPrompt == nil {
		t.Error("System prompt not set")
	}
	
	if memory.SystemPrompt.SystemPrompt != "System prompt" {
		t.Error("System prompt content not set correctly")
	}
	
	if len(memory.Steps) != 0 {
		t.Error("Expected empty steps initially")
	}
}

func TestAgentMemoryAddStep(t *testing.T) {
	memory := NewAgentMemory("System prompt")
	
	step := NewTaskStep("Test task", nil)
	memory.AddStep(step)
	
	if len(memory.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(memory.Steps))
	}
	
	if memory.Steps[0] != step {
		t.Error("Step not added correctly")
	}
}

func TestAgentMemorySetSystemPrompt(t *testing.T) {
	memory := NewAgentMemory("")
	
	memory.SetSystemPrompt("New system prompt")
	
	if memory.SystemPrompt.SystemPrompt != "New system prompt" {
		t.Error("System prompt not updated correctly")
	}
}

func TestAgentMemoryGetStepCount(t *testing.T) {
	memory := NewAgentMemory("System prompt")
	
	if memory.GetStepCount() != 0 {
		t.Error("Expected 0 steps initially")
	}
	
	memory.AddStep(NewTaskStep("Task 1", nil))
	memory.AddStep(NewTaskStep("Task 2", nil))
	
	if memory.GetStepCount() != 2 {
		t.Errorf("Expected 2 steps, got %d", memory.GetStepCount())
	}
}

func TestAgentMemoryGetLastStep(t *testing.T) {
	memory := NewAgentMemory("System prompt")
	
	// Test empty memory
	if memory.GetLastStep() != nil {
		t.Error("Expected nil for empty memory")
	}
	
	// Add steps
	step1 := NewTaskStep("Task 1", nil)
	step2 := NewTaskStep("Task 2", nil)
	
	memory.AddStep(step1)
	memory.AddStep(step2)
	
	lastStep := memory.GetLastStep()
	if lastStep != step2 {
		t.Error("GetLastStep didn't return the last step")
	}
}

func TestAgentMemoryReset(t *testing.T) {
	memory := NewAgentMemory("System prompt")
	
	// Add some steps
	memory.AddStep(NewTaskStep("Task 1", nil))
	memory.AddStep(NewTaskStep("Task 2", nil))
	
	memory.Reset()
	
	if len(memory.Steps) != 0 {
		t.Error("Expected empty steps after reset")
	}
	
	// System prompt should remain
	if memory.SystemPrompt == nil {
		t.Error("System prompt should not be cleared by reset")
	}
}

func TestAgentMemoryWriteMemoryToMessages(t *testing.T) {
	memory := NewAgentMemory("You are helpful")
	
	memory.AddStep(NewTaskStep("Test task", nil))
	
	messages, err := memory.WriteMemoryToMessages(false)
	if err != nil {
		t.Errorf("Failed to write memory to messages: %v", err)
	}
	
	// Should have system prompt + task
	if len(messages) < 2 {
		t.Errorf("Expected at least 2 messages, got %d", len(messages))
	}
	
	// First message should be system
	if messages[0].Role != models.RoleSystem {
		t.Error("First message should be system message")
	}
}

func TestAgentMemoryToDict(t *testing.T) {
	memory := NewAgentMemory("System prompt")
	memory.AddStep(NewTaskStep("Test task", nil))
	
	dict, err := memory.ToDict()
	if err != nil {
		t.Errorf("Failed to convert memory to dict: %v", err)
	}
	
	if dict["steps"] == nil {
		t.Error("Steps not included in dict")
	}
	
	if dict["system_prompt"] == nil {
		t.Error("System prompt not included in dict")
	}
}

func TestAgentMemoryGetFullSteps(t *testing.T) {
	memory := NewAgentMemory("System prompt")
	memory.AddStep(NewTaskStep("Test task", nil))
	
	steps, err := memory.GetFullSteps()
	if err != nil {
		t.Errorf("Failed to get full steps: %v", err)
	}
	
	if len(steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(steps))
	}
}

func TestAgentMemoryGetActionSteps(t *testing.T) {
	memory := NewAgentMemory("System prompt")
	
	taskStep := NewTaskStep("Test task", nil)
	actionStep := NewActionStep(1, time.Now())
	
	memory.AddStep(taskStep)
	memory.AddStep(actionStep)
	
	actionSteps := memory.GetActionSteps()
	
	if len(actionSteps) != 1 {
		t.Errorf("Expected 1 action step, got %d", len(actionSteps))
	}
	
	if actionSteps[0] != actionStep {
		t.Error("Wrong action step returned")
	}
}
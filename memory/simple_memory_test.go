package memory

import (
	"testing"
)

func TestSimpleMemory(t *testing.T) {
	// Type assertion to ensure SimpleMemory implements Memory interface
	var _ Memory = (*SimpleMemory)(nil)

	// Create a new memory instance
	mem := NewSimpleMemory()

	// Test AddMessage and GetMessages
	mem.AddMessage("user", "Hello")
	mem.AddMessage("assistant", "Hi there")

	messages := mem.GetMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	if messages[0].Role != "user" || messages[0].Content != "Hello" {
		t.Errorf("First message doesn't match expected values")
	}

	if messages[1].Role != "assistant" || messages[1].Content != "Hi there" {
		t.Errorf("Second message doesn't match expected values")
	}

	// Test Clone
	clone := mem.Clone()
	cloneMessages := clone.GetMessages()

	if len(cloneMessages) != 2 {
		t.Errorf("Expected 2 messages in clone, got %d", len(cloneMessages))
	}

	// Modify original, shouldn't affect clone
	mem.AddMessage("user", "New message")
	if len(clone.GetMessages()) != 2 {
		t.Errorf("Clone was affected by changes to original")
	}

	// Test Clear
	mem.Clear()
	if len(mem.GetMessages()) != 0 {
		t.Errorf("Memory should be empty after Clear")
	}
}

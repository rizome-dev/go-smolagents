package memory

// SimpleMemory implements Memory interface with a simple slice-based storage
type SimpleMemory struct {
	messages []Message
}

// NewSimpleMemory creates a new instance of SimpleMemory
func NewSimpleMemory() *SimpleMemory {
	return &SimpleMemory{
		messages: make([]Message, 0),
	}
}

// AddMessage adds a new message to the memory
func (sm *SimpleMemory) AddMessage(role, content string) {
	sm.messages = append(sm.messages, Message{
		Role:    role,
		Content: content,
	})
}

// GetMessages returns all messages in the memory
func (sm *SimpleMemory) GetMessages() []Message {
	return sm.messages
}

// Clear removes all messages from the memory
func (sm *SimpleMemory) Clear() {
	sm.messages = make([]Message, 0)
}

// Clone creates a deep copy of the memory
func (sm *SimpleMemory) Clone() Memory {
	clone := NewSimpleMemory()
	for _, msg := range sm.messages {
		clone.AddMessage(msg.Role, msg.Content)
	}
	return clone
}

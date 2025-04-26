// Package memory provides interfaces and implementations for agent memory systems.
package memory

import (
	"fmt"
	"image"
	"time"

	"github.com/rizome-dev/smolagentsgo/models"
	"github.com/rizome-dev/smolagentsgo/utils"
)

// ToolCall represents a call to a tool by the agent
type ToolCall struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
	ID        string      `json:"id"`
}

// Dict converts a ToolCall to a map representation
func (tc *ToolCall) Dict() map[string]interface{} {
	return map[string]interface{}{
		"id":   tc.ID,
		"type": "function",
		"function": map[string]interface{}{
			"name":      tc.Name,
			"arguments": utils.MakeJSONSerializable(tc.Arguments),
		},
	}
}

// MemoryStep is the base interface for all steps in agent memory
type MemoryStep interface {
	ToDict() map[string]interface{}
	ToMessages(options map[string]interface{}) []models.Message
}

// ActionStep represents an action taken by the agent
type ActionStep struct {
	ModelInputMessages []models.Message    `json:"model_input_messages,omitempty"`
	ToolCalls          []*ToolCall         `json:"tool_calls,omitempty"`
	StartTime          time.Time           `json:"start_time,omitempty"`
	EndTime            time.Time           `json:"end_time,omitempty"`
	StepNumber         int                 `json:"step_number,omitempty"`
	Error              error               `json:"error,omitempty"`
	Duration           float64             `json:"duration,omitempty"`
	ModelOutputMessage *models.ChatMessage `json:"model_output_message,omitempty"`
	ModelOutput        string              `json:"model_output,omitempty"`
	Observations       string              `json:"observations,omitempty"`
	ObservationsImages []image.Image       `json:"observations_images,omitempty"`
	ActionOutput       interface{}         `json:"action_output,omitempty"`
}

// ToDict converts ActionStep to a map representation
func (a *ActionStep) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"step_number": a.StepNumber,
		"start_time":  a.StartTime.Format(time.RFC3339),
		"end_time":    a.EndTime.Format(time.RFC3339),
		"duration":    a.Duration,
	}

	if a.ModelOutput != "" {
		result["model_output"] = a.ModelOutput
	}

	if a.Observations != "" {
		result["observations"] = a.Observations
	}

	if a.Error != nil {
		result["error"] = a.Error.Error()
	}

	if a.ActionOutput != nil {
		result["action_output"] = utils.MakeJSONSerializable(a.ActionOutput)
	}

	if len(a.ToolCalls) > 0 {
		toolCalls := make([]map[string]interface{}, len(a.ToolCalls))
		for i, tc := range a.ToolCalls {
			toolCalls[i] = tc.Dict()
		}
		result["tool_calls"] = toolCalls
	}

	return result
}

// ToMessages converts ActionStep to a slice of messages
func (a *ActionStep) ToMessages(options map[string]interface{}) []models.Message {
	var messages []models.Message

	// Add the model output message as assistant message
	if a.ModelOutput != "" {
		messages = append(messages, models.Message{
			Role: models.RoleAssistant,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: a.ModelOutput,
				},
			},
		})
	}

	// Add the tool calls as tool messages
	if a.Observations != "" {
		messages = append(messages, models.Message{
			Role: models.RoleTool,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: a.Observations,
				},
			},
		})

		// Add observation images if any
		for _, img := range a.ObservationsImages {
			messages = append(messages, models.Message{
				Role: models.RoleTool,
				Content: []models.MessageContent{
					{
						Type:      "image",
						ImageData: img,
					},
				},
			})
		}
	}

	return messages
}

// PlanningStep represents a planning step in the agent's memory
type PlanningStep struct {
	ModelInputMessages []models.Message    `json:"model_input_messages,omitempty"`
	StartTime          time.Time           `json:"start_time,omitempty"`
	EndTime            time.Time           `json:"end_time,omitempty"`
	StepNumber         int                 `json:"step_number,omitempty"`
	Error              error               `json:"error,omitempty"`
	Duration           float64             `json:"duration,omitempty"`
	ModelOutputMessage *models.ChatMessage `json:"model_output_message,omitempty"`
	ModelOutput        string              `json:"model_output,omitempty"`
	Plan               string              `json:"plan,omitempty"`
}

// ToDict converts PlanningStep to a map representation
func (p *PlanningStep) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"step_number": p.StepNumber,
		"start_time":  p.StartTime.Format(time.RFC3339),
		"end_time":    p.EndTime.Format(time.RFC3339),
		"duration":    p.Duration,
	}

	if p.ModelOutput != "" {
		result["model_output"] = p.ModelOutput
	}

	if p.Plan != "" {
		result["plan"] = p.Plan
	}

	if p.Error != nil {
		result["error"] = p.Error.Error()
	}

	return result
}

// ToMessages converts PlanningStep to a slice of messages
func (p *PlanningStep) ToMessages(options map[string]interface{}) []models.Message {
	var messages []models.Message

	// Check if summary mode is enabled
	summaryMode, ok := options["summary_mode"].(bool)
	if ok && summaryMode {
		return messages
	}

	// Add the plan as assistant message
	if p.Plan != "" {
		messages = append(messages, models.Message{
			Role: models.RoleAssistant,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: fmt.Sprintf("Plan: %s", p.Plan),
				},
			},
		})
	}

	return messages
}

// TaskStep represents a task given to the agent
type TaskStep struct {
	Task      string    `json:"task"`
	StartTime time.Time `json:"start_time"`
}

// ToDict converts TaskStep to a map representation
func (t *TaskStep) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"task":       t.Task,
		"start_time": t.StartTime.Format(time.RFC3339),
	}
}

// ToMessages converts TaskStep to a slice of messages
func (t *TaskStep) ToMessages(options map[string]interface{}) []models.Message {
	return []models.Message{
		{
			Role: models.RoleUser,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: t.Task,
				},
			},
		},
	}
}

// SystemPromptStep represents the system prompt
type SystemPromptStep struct {
	SystemPrompt string `json:"system_prompt"`
}

// ToDict converts SystemPromptStep to a map representation
func (s *SystemPromptStep) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"system_prompt": s.SystemPrompt,
	}
}

// ToMessages converts SystemPromptStep to a slice of messages
func (s *SystemPromptStep) ToMessages(options map[string]interface{}) []models.Message {
	// Check if summary mode is enabled
	summaryMode, ok := options["summary_mode"].(bool)
	if ok && summaryMode {
		return nil
	}

	return []models.Message{
		{
			Role: models.RoleSystem,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: s.SystemPrompt,
				},
			},
		},
	}
}

// FinalAnswerStep represents the final answer from the agent
type FinalAnswerStep struct {
	FinalAnswer interface{} `json:"final_answer"`
	StartTime   time.Time   `json:"start_time"`
}

// ToDict converts FinalAnswerStep to a map representation
func (f *FinalAnswerStep) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"final_answer": utils.MakeJSONSerializable(f.FinalAnswer),
		"start_time":   f.StartTime.Format(time.RFC3339),
	}
}

// ToMessages converts FinalAnswerStep to a slice of messages
func (f *FinalAnswerStep) ToMessages(options map[string]interface{}) []models.Message {
	return []models.Message{
		{
			Role: models.RoleAssistant,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: fmt.Sprintf("Final Answer: %v", f.FinalAnswer),
				},
			},
		},
	}
}

// AgentMemory holds the memory of an agent's execution
type AgentMemory struct {
	SystemPrompt *SystemPromptStep
	Steps        []MemoryStep
}

// NewAgentMemory creates a new AgentMemory with the given system prompt
func NewAgentMemory(systemPrompt string) *AgentMemory {
	return &AgentMemory{
		SystemPrompt: &SystemPromptStep{SystemPrompt: systemPrompt},
		Steps:        []MemoryStep{},
	}
}

// Reset clears all steps in memory
func (m *AgentMemory) Reset() {
	m.Steps = []MemoryStep{}
}

// GetSuccinctSteps returns a simplified version of steps
func (m *AgentMemory) GetSuccinctSteps() []map[string]interface{} {
	result := make([]map[string]interface{}, len(m.Steps))
	for i, step := range m.Steps {
		dict := step.ToDict()
		delete(dict, "model_input_messages")
		result[i] = dict
	}
	return result
}

// GetFullSteps returns all step details
func (m *AgentMemory) GetFullSteps() []map[string]interface{} {
	result := make([]map[string]interface{}, len(m.Steps))
	for i, step := range m.Steps {
		result[i] = step.ToDict()
	}
	return result
}

// Message represents a conversation message
type Message struct {
	Role    string
	Content string
}

// Memory defines the interface for storing and retrieving conversation history
type Memory interface {
	// AddMessage adds a new message to the memory
	AddMessage(role, content string)

	// GetMessages returns all messages in the memory
	GetMessages() []Message

	// Clear removes all messages from the memory
	Clear()

	// Clone creates a deep copy of the memory
	Clone() Memory
}

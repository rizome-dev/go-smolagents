// Package memory provides structures for agent's memory management
package memory

import (
	"encoding/json"
	"fmt"
	"image"
	"time"

	"github.com/rizome-dev/smolagentsgo/models"
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
			"arguments": makeJSONSerializable(tc.Arguments),
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
func (s *ActionStep) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"model_input_messages": s.ModelInputMessages,
		"tool_calls":           make([]map[string]interface{}, 0),
		"start_time":           s.StartTime,
		"end_time":             s.EndTime,
		"step":                 s.StepNumber,
		"error":                nil,
		"duration":             s.Duration,
		"model_output_message": s.ModelOutputMessage,
		"model_output":         s.ModelOutput,
		"observations":         s.Observations,
		"action_output":        makeJSONSerializable(s.ActionOutput),
	}

	if s.Error != nil {
		result["error"] = s.Error.Error()
	}

	if s.ToolCalls != nil {
		toolCalls := make([]map[string]interface{}, len(s.ToolCalls))
		for i, tc := range s.ToolCalls {
			toolCalls[i] = tc.Dict()
		}
		result["tool_calls"] = toolCalls
	}

	return result
}

// ToMessages converts ActionStep to a slice of messages
func (s *ActionStep) ToMessages(options map[string]interface{}) []models.Message {
	summaryMode := options["summary_mode"] == true
	showModelInputMessages := options["show_model_input_messages"] == true

	messages := []models.Message{}

	if s.ModelInputMessages != nil && showModelInputMessages {
		// Add model input messages
		for _, m := range s.ModelInputMessages {
			messages = append(messages, m)
		}
	}

	if s.ModelOutput != "" && !summaryMode {
		messages = append(messages, models.Message{
			Role: models.MessageRoleAssistant,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: s.ModelOutput,
				},
			},
		})
	}

	if s.ToolCalls != nil {
		toolCallStr := fmt.Sprintf("Calling tools:\n%v", s.ToolCalls)
		messages = append(messages, models.Message{
			Role: models.MessageRoleToolCall,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: toolCallStr,
				},
			},
		})
	}

	// Handle observation images
	if len(s.ObservationsImages) > 0 {
		msgContent := make([]models.MessageContent, len(s.ObservationsImages))
		for i, img := range s.ObservationsImages {
			msgContent[i] = models.MessageContent{
				Type:  "image",
				Image: img,
			}
		}
		messages = append(messages, models.Message{
			Role:    models.MessageRoleUser,
			Content: msgContent,
		})
	}

	if s.Observations != "" {
		messages = append(messages, models.Message{
			Role: models.MessageRoleToolResponse,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: fmt.Sprintf("Observation:\n%s", s.Observations),
				},
			},
		})
	}

	if s.Error != nil {
		errMsg := fmt.Sprintf("Error:\n%s\nNow let's retry: take care not to repeat previous errors! If you have retried several times, try a completely different approach.\n", s.Error.Error())
		msgContent := errMsg

		if s.ToolCalls != nil && len(s.ToolCalls) > 0 {
			msgContent = fmt.Sprintf("Call id: %s\n%s", s.ToolCalls[0].ID, errMsg)
		}

		messages = append(messages, models.Message{
			Role: models.MessageRoleToolResponse,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: msgContent,
				},
			},
		})
	}

	return messages
}

// PlanningStep represents a planning step taken by the agent
type PlanningStep struct {
	ModelInputMessages []models.Message    `json:"model_input_messages"`
	ModelOutputMessage *models.ChatMessage `json:"model_output_message"`
	Plan               string              `json:"plan"`
}

// ToDict converts PlanningStep to a map representation
func (s *PlanningStep) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"model_input_messages": s.ModelInputMessages,
		"model_output_message": s.ModelOutputMessage,
		"plan":                 s.Plan,
	}
}

// ToMessages converts PlanningStep to a slice of messages
func (s *PlanningStep) ToMessages(options map[string]interface{}) []models.Message {
	summaryMode := options["summary_mode"] == true

	if summaryMode {
		return []models.Message{}
	}

	return []models.Message{
		{
			Role: models.MessageRoleAssistant,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: s.Plan,
				},
			},
		},
		{
			Role: models.MessageRoleUser,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: "Now proceed and carry out this plan.",
				},
			},
		},
	}
}

// TaskStep represents a task given to the agent
type TaskStep struct {
	Task       string        `json:"task"`
	TaskImages []image.Image `json:"task_images,omitempty"`
}

// ToDict converts TaskStep to a map representation
func (s *TaskStep) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"task":        s.Task,
		"task_images": s.TaskImages,
	}
}

// ToMessages converts TaskStep to a slice of messages
func (s *TaskStep) ToMessages(options map[string]interface{}) []models.Message {
	content := []models.MessageContent{
		{
			Type: "text",
			Text: fmt.Sprintf("New task:\n%s", s.Task),
		},
	}

	if s.TaskImages != nil {
		for _, img := range s.TaskImages {
			content = append(content, models.MessageContent{
				Type:  "image",
				Image: img,
			})
		}
	}

	return []models.Message{
		{
			Role:    models.MessageRoleUser,
			Content: content,
		},
	}
}

// SystemPromptStep represents the system prompt given to the agent
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
	summaryMode := options["summary_mode"] == true

	if summaryMode {
		return []models.Message{}
	}

	return []models.Message{
		{
			Role: models.MessageRoleSystem,
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
}

// ToDict converts FinalAnswerStep to a map representation
func (s *FinalAnswerStep) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"final_answer": makeJSONSerializable(s.FinalAnswer),
	}
}

// ToMessages implements the MemoryStep interface but doesn't need to return messages
func (s *FinalAnswerStep) ToMessages(options map[string]interface{}) []models.Message {
	return []models.Message{}
}

// AgentMemory represents the memory of an agent, including all steps
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

// Utility functions

// makeJSONSerializable converts a value to a JSON serializable form
func makeJSONSerializable(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	// Try to marshal and unmarshal to check if it's serializable
	_, err := json.Marshal(v)
	if err == nil {
		return v
	}

	// If not serializable, convert to string
	return fmt.Sprintf("%v", v)
}

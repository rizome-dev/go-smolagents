// Package memory provides memory management for agent conversations and execution history.
//
// This includes step-by-step tracking of agent actions, tool calls, planning steps,
// and conversation history management.
//
// This is a 1-to-1 port of the Python smolagents.memory module.
package memory

import (
	"encoding/json"
	"fmt"
	"image"
	"time"

	"github.com/rizome-dev/smolagentsgo/pkg/models"
	"github.com/rizome-dev/smolagentsgo/pkg/monitoring"
)

// Message represents a single message in the conversation
type Message struct {
	Role       models.MessageRole       `json:"role"`
	Content    []map[string]interface{} `json:"content"`
	Name       string                   `json:"name,omitempty"`
	ToolCalls  []ToolCall               `json:"tool_calls,omitempty"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
	Images     []image.Image            `json:"images,omitempty"`
	Metadata   map[string]interface{}   `json:"metadata,omitempty"`
}

// NewMessage creates a new message with the specified role and content
func NewMessage(role models.MessageRole, content string) *Message {
	return &Message{
		Role: role,
		Content: []map[string]interface{}{
			{
				"type": "text",
				"text": content,
			},
		},
		Metadata: make(map[string]interface{}),
	}
}

// ToDict converts the message to a dictionary representation
func (m *Message) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"role":    string(m.Role),
		"content": m.Content,
	}

	if m.Name != "" {
		result["name"] = m.Name
	}

	if len(m.ToolCalls) > 0 {
		toolCalls := make([]map[string]interface{}, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			toolCalls[i] = tc.ToDict()
		}
		result["tool_calls"] = toolCalls
	}

	if m.ToolCallID != "" {
		result["tool_call_id"] = m.ToolCallID
	}

	if len(m.Metadata) > 0 {
		result["metadata"] = m.Metadata
	}

	return result
}

// ToolCall represents a tool call made by an agent
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToDict converts the tool call to a dictionary representation
func (tc *ToolCall) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"id":        tc.ID,
		"name":      tc.Name,
		"arguments": tc.Arguments,
	}
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Add combines this token usage with another
func (tu *TokenUsage) Add(other *TokenUsage) {
	if other != nil {
		tu.InputTokens += other.InputTokens
		tu.OutputTokens += other.OutputTokens
		tu.TotalTokens += other.TotalTokens
	}
}

// Timing represents timing information for operations
type Timing struct {
	StartTime time.Time      `json:"start_time"`
	EndTime   *time.Time     `json:"end_time,omitempty"`
	Duration  *time.Duration `json:"duration,omitempty"`
}

// NewTiming creates a new timing instance with start time set to now
func NewTiming() *Timing {
	return &Timing{
		StartTime: time.Now(),
	}
}

// End marks the end time and calculates duration
func (t *Timing) End() {
	now := time.Now()
	t.EndTime = &now
	duration := now.Sub(t.StartTime)
	t.Duration = &duration
}

// MemoryStep is the interface that all memory steps must implement
type MemoryStep interface {
	// ToMessages converts the step to a list of messages for model consumption
	ToMessages(summaryMode bool) ([]Message, error)

	// ToDict converts the step to a dictionary representation
	ToDict() (map[string]interface{}, error)

	// GetType returns the type identifier for this step
	GetType() string
}

// ActionStep represents an action taken by the agent (tool calls, model interactions)
type ActionStep struct {
	StepNumber         int                    `json:"step_number"`
	Timing             monitoring.Timing      `json:"timing"`
	ModelInputMessages []Message              `json:"model_input_messages,omitempty"`
	ToolCalls          []ToolCall             `json:"tool_calls,omitempty"`
	Error              error                  `json:"error,omitempty"`
	ModelOutputMessage *models.ChatMessage    `json:"model_output_message,omitempty"`
	ModelOutput        string                 `json:"model_output,omitempty"`
	Observations       string                 `json:"observations,omitempty"`
	ObservationImages  []interface{}          `json:"observation_images,omitempty"`
	ActionOutput       interface{}            `json:"action_output,omitempty"`
	TokenUsage         *monitoring.TokenUsage `json:"token_usage,omitempty"`
}

// NewActionStep creates a new action step
func NewActionStep(stepNumber int, startTime ...time.Time) *ActionStep {
	start := time.Now()
	if len(startTime) > 0 {
		start = startTime[0]
	}

	return &ActionStep{
		StepNumber: stepNumber,
		Timing:     monitoring.Timing{StartTime: start},
	}
}

// NewActionStepWithImages creates a new action step with images
func NewActionStepWithImages(stepNumber int, startTime time.Time, images []interface{}) *ActionStep {
	return &ActionStep{
		StepNumber:        stepNumber,
		Timing:            monitoring.Timing{StartTime: startTime},
		ObservationImages: images,
	}
}

// Getter and setter methods for ActionStep
func (as *ActionStep) GetStepNumber() int                         { return as.StepNumber }
func (as *ActionStep) GetTiming() monitoring.Timing               { return as.Timing }
func (as *ActionStep) GetModelInputMessages() []Message           { return as.ModelInputMessages }
func (as *ActionStep) GetToolCalls() []ToolCall                   { return as.ToolCalls }
func (as *ActionStep) GetError() error                            { return as.Error }
func (as *ActionStep) GetModelOutputMessage() *models.ChatMessage { return as.ModelOutputMessage }
func (as *ActionStep) GetModelOutput() string                     { return as.ModelOutput }
func (as *ActionStep) GetObservations() string                    { return as.Observations }
func (as *ActionStep) GetObservationImages() []interface{}        { return as.ObservationImages }
func (as *ActionStep) GetActionOutput() interface{}               { return as.ActionOutput }
func (as *ActionStep) GetTokenUsage() *monitoring.TokenUsage      { return as.TokenUsage }

func (as *ActionStep) SetStepNumber(n int)                           { as.StepNumber = n }
func (as *ActionStep) SetTiming(t monitoring.Timing)                 { as.Timing = t }
func (as *ActionStep) SetModelInputMessages(msgs []Message)          { as.ModelInputMessages = msgs }
func (as *ActionStep) SetToolCalls(calls []ToolCall)                 { as.ToolCalls = calls }
func (as *ActionStep) SetError(err error)                            { as.Error = err }
func (as *ActionStep) SetModelOutputMessage(msg *models.ChatMessage) { as.ModelOutputMessage = msg }
func (as *ActionStep) SetModelOutput(output string)                  { as.ModelOutput = output }
func (as *ActionStep) SetObservations(obs string)                    { as.Observations = obs }
func (as *ActionStep) SetObservationImages(images []interface{})     { as.ObservationImages = images }
func (as *ActionStep) SetActionOutput(output interface{})            { as.ActionOutput = output }
func (as *ActionStep) SetTokenUsage(usage *monitoring.TokenUsage)    { as.TokenUsage = usage }

// GetType implements MemoryStep
func (as *ActionStep) GetType() string {
	return "action"
}

// ToMessages implements MemoryStep
func (as *ActionStep) ToMessages(summaryMode bool) ([]Message, error) {
	var messages []Message

	// Add model output if available and not in summary mode
	if as.ModelOutput != "" && !summaryMode {
		messages = append(messages, *NewMessage(models.RoleAssistant, as.ModelOutput))
	}

	// Add tool calls if any
	if len(as.ToolCalls) > 0 {
		// For tool calling agents, create a tool call message
		toolCallContent := "Calling tools:\n"
		for _, toolCall := range as.ToolCalls {
			toolCallContent += fmt.Sprintf("- %s: %v\n", toolCall.Name, toolCall.Arguments)
		}
		messages = append(messages, *NewMessage(models.RoleToolCall, toolCallContent))
	}

	// Add observation images if any
	if len(as.ObservationImages) > 0 {
		// Handle images - simplified for now
		imageMessage := NewMessage(models.RoleUser, "Observation images:")
		// TODO: Set images properly when image handling is implemented
		messages = append(messages, *imageMessage)
	}

	// Add observations if any
	if as.Observations != "" {
		messages = append(messages, *NewMessage(models.RoleToolResponse, "Observation:\n"+as.Observations))
	}

	// Add error if any
	if as.Error != nil {
		errorText := "Error:\n" + as.Error.Error() +
			"\nNow let's retry: take care not to repeat previous errors! " +
			"If you have retried several times, try a completely different approach.\n"

		messageContent := ""
		if len(as.ToolCalls) > 0 {
			messageContent = fmt.Sprintf("Call id: %s\n", as.ToolCalls[0].ID)
		}
		messageContent += errorText

		messages = append(messages, *NewMessage(models.RoleToolResponse, messageContent))
	}

	return messages, nil
}

// ToDict implements MemoryStep
func (as *ActionStep) ToDict() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"type":        as.GetType(),
		"step_number": as.StepNumber,
	}

	// Add timing info
	result["timing"] = map[string]interface{}{
		"start_time": as.Timing.StartTime,
	}
	if as.Timing.EndTime != nil && !as.Timing.EndTime.IsZero() {
		result["timing"].(map[string]interface{})["end_time"] = *as.Timing.EndTime
	}

	if len(as.ToolCalls) > 0 {
		toolCalls := make([]map[string]interface{}, len(as.ToolCalls))
		for i, tc := range as.ToolCalls {
			toolCalls[i] = tc.ToDict()
		}
		result["tool_calls"] = toolCalls
	}

	if len(as.ModelInputMessages) > 0 {
		result["model_input_messages"] = as.ModelInputMessages
	}

	if as.ModelOutput != "" {
		result["model_output"] = as.ModelOutput
	}

	if as.ModelOutputMessage != nil {
		msgDict := as.ModelOutputMessage.ToDict()
		result["model_output_message"] = msgDict
	}

	if as.Observations != "" {
		result["observations"] = as.Observations
	}

	if as.ActionOutput != nil {
		result["action_output"] = as.ActionOutput
	}

	if as.Error != nil {
		result["error"] = as.Error.Error()
	}

	if as.TokenUsage != nil {
		result["token_usage"] = map[string]interface{}{
			"input_tokens":  as.TokenUsage.InputTokens,
			"output_tokens": as.TokenUsage.OutputTokens,
		}
	}

	return result, nil
}

// PlanningStep represents a planning step by the agent
type PlanningStep struct {
	ModelInputMessages []Message              `json:"model_input_messages"`
	ModelOutputMessage models.ChatMessage     `json:"model_output_message"`
	Plan               string                 `json:"plan"`
	Timing             monitoring.Timing      `json:"timing"`
	TokenUsage         *monitoring.TokenUsage `json:"token_usage,omitempty"`
}

// NewPlanningStep creates a new planning step
func NewPlanningStep(inputMessages []Message, outputMessage models.ChatMessage, plan string, timing monitoring.Timing, tokenUsage *monitoring.TokenUsage) *PlanningStep {
	return &PlanningStep{
		ModelInputMessages: inputMessages,
		ModelOutputMessage: outputMessage,
		Plan:               plan,
		Timing:             timing,
		TokenUsage:         tokenUsage,
	}
}

// Getter methods for PlanningStep
func (ps *PlanningStep) GetModelInputMessages() []Message          { return ps.ModelInputMessages }
func (ps *PlanningStep) GetModelOutputMessage() models.ChatMessage { return ps.ModelOutputMessage }
func (ps *PlanningStep) GetPlan() string                           { return ps.Plan }
func (ps *PlanningStep) GetTiming() monitoring.Timing              { return ps.Timing }
func (ps *PlanningStep) GetTokenUsage() *monitoring.TokenUsage     { return ps.TokenUsage }

func (ps *PlanningStep) SetModelInputMessages(msgs []Message)         { ps.ModelInputMessages = msgs }
func (ps *PlanningStep) SetModelOutputMessage(msg models.ChatMessage) { ps.ModelOutputMessage = msg }
func (ps *PlanningStep) SetPlan(plan string)                          { ps.Plan = plan }
func (ps *PlanningStep) SetTiming(timing monitoring.Timing)           { ps.Timing = timing }
func (ps *PlanningStep) SetTokenUsage(usage *monitoring.TokenUsage)   { ps.TokenUsage = usage }

// GetType implements MemoryStep
func (ps *PlanningStep) GetType() string {
	return "planning"
}

// ToMessages implements MemoryStep
func (ps *PlanningStep) ToMessages(summaryMode bool) ([]Message, error) {
	if summaryMode {
		return []Message{}, nil
	}

	return []Message{
		*NewMessage(models.RoleAssistant, ps.Plan),
		*NewMessage(models.RoleUser, "Now proceed and carry out this plan."),
	}, nil
}

// ToDict implements MemoryStep
func (ps *PlanningStep) ToDict() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"type": ps.GetType(),
		"plan": ps.Plan,
	}

	result["timing"] = map[string]interface{}{
		"start_time": ps.Timing.StartTime,
	}
	if ps.Timing.EndTime != nil && !ps.Timing.EndTime.IsZero() {
		result["timing"].(map[string]interface{})["end_time"] = *ps.Timing.EndTime
	}

	if len(ps.ModelInputMessages) > 0 {
		result["model_input_messages"] = ps.ModelInputMessages
	}

	msgDict := ps.ModelOutputMessage.ToDict()
	result["model_output_message"] = msgDict

	if ps.TokenUsage != nil {
		result["token_usage"] = map[string]interface{}{
			"input_tokens":  ps.TokenUsage.InputTokens,
			"output_tokens": ps.TokenUsage.OutputTokens,
		}
	}

	return result, nil
}

// TaskStep represents the initial task given to the agent
type TaskStep struct {
	Task       string        `json:"task"`
	TaskImages []interface{} `json:"task_images,omitempty"`
}

// NewTaskStep creates a new task step
func NewTaskStep(task string, images ...[]interface{}) *TaskStep {
	var taskImages []interface{}
	if len(images) > 0 {
		taskImages = images[0]
	}

	return &TaskStep{
		Task:       task,
		TaskImages: taskImages,
	}
}

// GetType implements MemoryStep
func (ts *TaskStep) GetType() string {
	return "task"
}

// ToMessages implements MemoryStep
func (ts *TaskStep) ToMessages(summaryMode bool) ([]Message, error) {
	content := []map[string]interface{}{
		{
			"type": "text",
			"text": fmt.Sprintf("New task:\n%s", ts.Task),
		},
	}

	// Add images if present
	if len(ts.TaskImages) > 0 {
		for _, img := range ts.TaskImages {
			content = append(content, map[string]interface{}{
				"type":  "image",
				"image": img,
			})
		}
	}

	message := &Message{
		Role:    models.RoleUser,
		Content: content,
	}

	return []Message{*message}, nil
}

// ToDict implements MemoryStep
func (ts *TaskStep) ToDict() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"type": ts.GetType(),
		"task": ts.Task,
	}

	if len(ts.TaskImages) > 0 {
		result["task_images_count"] = len(ts.TaskImages)
	}

	return result, nil
}

// SystemPromptStep represents the system prompt
type SystemPromptStep struct {
	SystemPrompt string `json:"system_prompt"`
}

// NewSystemPromptStep creates a new system prompt step
func NewSystemPromptStep(systemPrompt string) *SystemPromptStep {
	return &SystemPromptStep{
		SystemPrompt: systemPrompt,
	}
}

// GetType implements MemoryStep
func (sps *SystemPromptStep) GetType() string {
	return "system_prompt"
}

// ToMessages implements MemoryStep
func (sps *SystemPromptStep) ToMessages(summaryMode bool) ([]Message, error) {
	if summaryMode {
		return []Message{}, nil
	}

	return []Message{*NewMessage(models.RoleSystem, sps.SystemPrompt)}, nil
}

// ToDict implements MemoryStep
func (sps *SystemPromptStep) ToDict() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type":          sps.GetType(),
		"system_prompt": sps.SystemPrompt,
	}, nil
}

// FinalAnswerStep represents the final answer from the agent
type FinalAnswerStep struct {
	Output interface{} `json:"output"`
}

// NewFinalAnswerStep creates a new final answer step
func NewFinalAnswerStep(output interface{}) *FinalAnswerStep {
	return &FinalAnswerStep{
		Output: output,
	}
}

// GetType implements MemoryStep
func (fas *FinalAnswerStep) GetType() string {
	return "final_answer"
}

// ToMessages implements MemoryStep
func (fas *FinalAnswerStep) ToMessages(summaryMode bool) ([]Message, error) {
	// Final answer doesn't contribute to message history
	return []Message{}, nil
}

// ToDict implements MemoryStep
func (fas *FinalAnswerStep) ToDict() (map[string]interface{}, error) {
	return map[string]interface{}{
		"type":   fas.GetType(),
		"output": fas.Output,
	}, nil
}

// Getter and setter methods for FinalAnswerStep
func (fas *FinalAnswerStep) GetOutput() interface{}       { return fas.Output }
func (fas *FinalAnswerStep) SetOutput(output interface{}) { fas.Output = output }

// AgentMemory represents the agent's conversation memory
type AgentMemory struct {
	SystemPrompt *SystemPromptStep `json:"system_prompt,omitempty"`
	Steps        []MemoryStep      `json:"steps"`
}

// NewAgentMemory creates a new agent memory instance
func NewAgentMemory(systemPrompt string) *AgentMemory {
	return &AgentMemory{
		SystemPrompt: NewSystemPromptStep(systemPrompt),
		Steps:        make([]MemoryStep, 0),
	}
}

// AddStep adds a memory step to the agent's memory
func (am *AgentMemory) AddStep(step MemoryStep) {
	am.Steps = append(am.Steps, step)
}

// SetSystemPrompt sets the system prompt for the agent
func (am *AgentMemory) SetSystemPrompt(prompt string) {
	am.SystemPrompt = NewSystemPromptStep(prompt)
}

// GetSystemPrompt returns the system prompt step
func (am *AgentMemory) GetSystemPrompt() *SystemPromptStep {
	return am.SystemPrompt
}

// GetSteps returns all memory steps
func (am *AgentMemory) GetSteps() []MemoryStep {
	return am.Steps
}

// WriteMemoryToMessages converts the agent's memory to a list of messages for model consumption
func (am *AgentMemory) WriteMemoryToMessages(summaryMode bool) ([]Message, error) {
	var messages []Message

	// Add system prompt if not in summary mode
	if am.SystemPrompt != nil {
		systemMessages, err := am.SystemPrompt.ToMessages(summaryMode)
		if err != nil {
			return nil, fmt.Errorf("failed to convert system prompt to messages: %w", err)
		}
		messages = append(messages, systemMessages...)
	}

	// Add all steps
	for _, step := range am.Steps {
		stepMessages, err := step.ToMessages(summaryMode)
		if err != nil {
			return nil, fmt.Errorf("failed to convert step to messages: %w", err)
		}
		messages = append(messages, stepMessages...)
	}

	return messages, nil
}

// ToDict converts the agent memory to a dictionary representation
func (am *AgentMemory) ToDict() (map[string]interface{}, error) {
	steps := make([]map[string]interface{}, len(am.Steps))
	for i, step := range am.Steps {
		stepDict, err := step.ToDict()
		if err != nil {
			return nil, fmt.Errorf("failed to convert step %d to dict: %w", i, err)
		}
		steps[i] = stepDict
	}

	result := map[string]interface{}{
		"steps": steps,
	}

	if am.SystemPrompt != nil {
		systemDict, err := am.SystemPrompt.ToDict()
		if err != nil {
			return nil, fmt.Errorf("failed to convert system prompt to dict: %w", err)
		}
		result["system_prompt"] = systemDict
	}

	return result, nil
}

// GetFullSteps returns all steps as dictionaries (matching Python API)
func (am *AgentMemory) GetFullSteps() ([]map[string]interface{}, error) {
	steps := make([]map[string]interface{}, len(am.Steps))
	for i, step := range am.Steps {
		stepDict, err := step.ToDict()
		if err != nil {
			return nil, fmt.Errorf("failed to convert step %d to dict: %w", i, err)
		}
		steps[i] = stepDict
	}
	return steps, nil
}

// Replay prints a replay of the agent's steps (simplified implementation)
func (am *AgentMemory) Replay(logger interface{}, detailed bool) error {
	// This would need proper implementation with the logger interface
	// For now, just a placeholder
	fmt.Println("Replaying agent steps...")
	if am.SystemPrompt != nil {
		fmt.Printf("System prompt: %s\n", am.SystemPrompt.SystemPrompt)
	}
	for i, step := range am.Steps {
		fmt.Printf("Step %d: %s\n", i+1, step.GetType())
	}
	return nil
}

// Reset clears all memory except system prompt
func (am *AgentMemory) Reset() {
	am.Steps = make([]MemoryStep, 0)
}

// GetStepCount returns the number of steps in memory
func (am *AgentMemory) GetStepCount() int {
	return len(am.Steps)
}

// GetLastStep returns the last step in memory, or nil if no steps exist
func (am *AgentMemory) GetLastStep() MemoryStep {
	if len(am.Steps) == 0 {
		return nil
	}
	return am.Steps[len(am.Steps)-1]
}

// GetActionSteps returns only the action steps from memory
func (am *AgentMemory) GetActionSteps() []*ActionStep {
	var actionSteps []*ActionStep
	for _, step := range am.Steps {
		if actionStep, ok := step.(*ActionStep); ok {
			actionSteps = append(actionSteps, actionStep)
		}
	}
	return actionSteps
}

// GetPlanningSteps returns only the planning steps from memory
func (am *AgentMemory) GetPlanningSteps() []*PlanningStep {
	var planningSteps []*PlanningStep
	for _, step := range am.Steps {
		if planningStep, ok := step.(*PlanningStep); ok {
			planningSteps = append(planningSteps, planningStep)
		}
	}
	return planningSteps
}

// MarshalJSON custom marshaling for AgentMemory
func (am *AgentMemory) MarshalJSON() ([]byte, error) {
	dict, err := am.ToDict()
	if err != nil {
		return nil, err
	}
	return json.Marshal(dict)
}

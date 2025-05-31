// Package agents - ToolCallingAgent implementation
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/default_tools"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/memory"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/models"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/monitoring"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/tools"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/utils"
)

// Default system prompt for ToolCallingAgent
const DefaultToolCallingSystemPrompt = `You are an expert assistant. You are given a user query or task, and you should complete it using the available tools.

You have access to the following tools:
{{tool_descriptions}}

Your output should directly call tools when appropriate. Always use the exact function names provided.

Be helpful, accurate, and efficient in your responses.`

// ToolCallingAgent implements an agent that can call tools using structured tool calls
type ToolCallingAgent struct {
	*BaseMultiStepAgent
	finalAnswerTool tools.Tool
}

// NewToolCallingAgent creates a new tool-calling agent
func NewToolCallingAgent(
	model models.Model,
	toolsArg []tools.Tool,
	systemPrompt string,
	options map[string]interface{},
) (*ToolCallingAgent, error) {
	
	// Create base agent
	baseAgent, err := NewBaseMultiStepAgent(model, toolsArg, systemPrompt, options)
	if err != nil {
		return nil, err
	}
	
	// Create final answer tool
	finalAnswerTool := default_tools.NewFinalAnswerTool()
	
	// Add final answer tool to the agent's tools
	err = baseAgent.AddTool(finalAnswerTool)
	if err != nil {
		return nil, fmt.Errorf("failed to add final answer tool: %w", err)
	}
	
	agent := &ToolCallingAgent{
		BaseMultiStepAgent: baseAgent,
		finalAnswerTool:    finalAnswerTool,
	}
	
	// Set default system prompt if none provided
	if systemPrompt == "" {
		agent.initializeSystemPrompt()
	}
	
	return agent, nil
}

// NewToolCallingAgentSimple creates a tool-calling agent with minimal configuration
func NewToolCallingAgentSimple(toolsArg []tools.Tool, model models.Model) (*ToolCallingAgent, error) {
	return NewToolCallingAgent(model, toolsArg, "", nil)
}

// Run implements MultiStepAgent for ToolCallingAgent
func (tca *ToolCallingAgent) Run(options *RunOptions) (*RunResult, error) {
	if options == nil {
		return nil, utils.NewAgentError("run options cannot be nil")
	}
	
	// Set running state
	tca.isRunning = true
	defer func() { tca.isRunning = false }()
	
	// Start timing
	result := NewRunResult()
	
	// Reset if requested
	if options.Reset {
		tca.Reset()
	}
	
	// Set up context
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Add task to memory
	taskStep := memory.NewTaskStep(options.Task)
	if len(options.Images) > 0 {
		taskStep.TaskImages = options.Images
	}
	tca.memory.AddStep(taskStep)
	
	// Determine max steps
	maxSteps := tca.maxSteps
	if options.MaxSteps != nil {
		maxSteps = *options.MaxSteps
	}
	
	// Execute agent steps
	for tca.stepCount < maxSteps {
		// Check for interruption
		if tca.interrupted {
			result.State = "interrupted"
			result.Error = utils.NewAgentExecutionError("agent execution was interrupted")
			break
		}
		
		// Check context cancellation
		select {
		case <-ctx.Done():
			result.State = "cancelled"
			result.Error = ctx.Err()
			result.Timing.End()
			return result, nil
		default:
		}
		
		tca.stepCount++
		
		// Execute step
		stepResult, err := tca.executeStep(ctx, tca.stepCount, options)
		if err != nil {
			result.State = "error"
			result.Error = err
			break
		}
		
		// Execute step callbacks
		if len(options.StepCallbacks) > 0 {
			latestStep := tca.memory.GetLastStep()
			if latestStep != nil {
				for _, callback := range options.StepCallbacks {
					if err := callback(latestStep); err != nil {
						result.State = "callback_error"
						result.Error = fmt.Errorf("step callback error: %w", err)
						result.Timing.End()
						return result, err
					}
				}
			}
		}
		
		// Check for final answer
		if stepResult.isFinalAnswer {
			result.State = "success"
			result.Output = stepResult.output
			result.StepCount = tca.stepCount
			result.TokenUsage = stepResult.tokenUsage
			break
		}
	}
	
	// Check if max steps reached
	if tca.stepCount >= maxSteps && result.State == "" {
		result.State = "max_steps_error"
		result.Error = utils.NewAgentMaxStepsError(fmt.Sprintf("reached maximum steps: %d", maxSteps))
	}
	
	// Finalize result
	result.StepCount = tca.stepCount
	result.Messages = tca.getMessagesForResult()
	result.Timing.End()
	
	return result, nil
}

// RunStream implements MultiStepAgent for ToolCallingAgent
func (tca *ToolCallingAgent) RunStream(options *RunOptions) (<-chan *StreamStepResult, error) {
	resultChan := make(chan *StreamStepResult, 100)
	
	go func() {
		defer close(resultChan)
		
		// Execute the agent and stream results
		result, err := tca.Run(options)
		if err != nil {
			resultChan <- &StreamStepResult{
				Error:      err,
				IsComplete: true,
			}
			return
		}
		
		// Send final result
		resultChan <- &StreamStepResult{
			StepNumber: tca.stepCount,
			StepType:   "final",
			Output:     result.Output,
			IsComplete: true,
			Metadata: map[string]interface{}{
				"state":       result.State,
				"step_count":  result.StepCount,
				"token_usage": result.TokenUsage,
			},
		}
	}()
	
	return resultChan, nil
}

// stepResult represents the result of executing one step
type stepResult struct {
	isFinalAnswer bool
	output        interface{}
	tokenUsage    *monitoring.TokenUsage
}

// executeStep executes a single agent step
func (tca *ToolCallingAgent) executeStep(ctx context.Context, stepNumber int, options *RunOptions) (*stepResult, error) {
	step := memory.NewActionStep(stepNumber)
	defer func() {
		step.Timing.End()
		tca.memory.AddStep(step)
	}()
	
	// Start monitoring
	if tca.monitor != nil {
		tca.monitor.StartStep(stepNumber, "tool_calling")
		defer tca.monitor.EndStep()
	}
	
	// Prepare messages for the model
	messages, err := tca.memory.WriteMemoryToMessages(false)
	if err != nil {
		step.Error = utils.NewAgentGenerationError("failed to write memory to messages: " + err.Error())
		return nil, step.Error
	}
	step.ModelInputMessages = messages
	
	// Convert messages to model format
	modelMessages := make([]interface{}, len(messages))
	for i, msg := range messages {
		modelMessages[i] = msg.ToDict()
	}
	
	// Build generation options
	genOptions := &models.GenerateOptions{
		ToolsToCallFrom: tca.tools,
		MaxTokens:       func() *int { v := 2048; return &v }(),
		Temperature:     func() *float64 { v := 0.7; return &v }(),
	}
	
	// Generate response
	response, err := tca.model.Generate(modelMessages, genOptions)
	if err != nil {
		step.Error = utils.NewAgentGenerationError("model generation failed: " + err.Error())
		return nil, step.Error
	}
	
	step.ModelOutputMessage = &models.ChatMessage{
		Role: response.Role,
		Content: response.Content,
	}
	
	if response.Content != nil {
		step.ModelOutput = *response.Content
	}
	step.TokenUsage = response.TokenUsage
	
	// Add token usage to monitoring
	if tca.monitor != nil {
		tca.monitor.AddTokenUsage(response.TokenUsage)
	}
	
	// Process tool calls
	if len(response.ToolCalls) > 0 {
		return tca.processToolCalls(ctx, step, response.ToolCalls)
	}
	
	// No tool calls - this might be a direct answer
	if response.Content != nil && *response.Content != "" {
		// Check if this looks like a final answer
		if tca.isFinalAnswerContent(*response.Content) {
			step.ActionOutput = *response.Content
			return &stepResult{
				isFinalAnswer: true,
				output:        *response.Content,
				tokenUsage:    response.TokenUsage,
			}, nil
		}
	}
	
	return &stepResult{
		isFinalAnswer: false,
		tokenUsage:    response.TokenUsage,
	}, nil
}

// processToolCalls processes tool calls from the model response
func (tca *ToolCallingAgent) processToolCalls(ctx context.Context, step *memory.ActionStep, toolCalls []models.ChatMessageToolCall) (*stepResult, error) {
	step.ToolCalls = make([]memory.ToolCall, len(toolCalls))
	var observations []string
	var lastOutput interface{}
	
	for i, tc := range toolCalls {
		// Convert to memory.ToolCall
		memoryTC := memory.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: make(map[string]interface{}),
		}
		
		// Parse arguments
		if args, ok := tc.Function.Arguments.(map[string]interface{}); ok {
			memoryTC.Arguments = args
		} else if argsStr, ok := tc.Function.Arguments.(string); ok {
			// Try to parse as JSON
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &parsed); err == nil {
				memoryTC.Arguments = parsed
			} else {
				memoryTC.Arguments = map[string]interface{}{"input": argsStr}
			}
		}
		
		step.ToolCalls[i] = memoryTC
		
		// Log tool call
		if tca.monitor != nil {
			tca.monitor.LogToolCall(tc.Function.Name, memoryTC.Arguments)
		}
		
		// Check for final answer
		if tc.Function.Name == "final_answer" {
			if answer, exists := memoryTC.Arguments["answer"]; exists {
				step.ActionOutput = answer
				
				// Log tool result
				if tca.monitor != nil {
					tca.monitor.LogToolResult("final_answer", answer, nil)
				}
				
				return &stepResult{
					isFinalAnswer: true,
					output:        answer,
					tokenUsage:    step.TokenUsage,
				}, nil
			}
		}
		
		// Execute the tool
		tool, exists := tca.GetTool(tc.Function.Name)
		if !exists {
			errMsg := fmt.Sprintf("Tool '%s' not found", tc.Function.Name)
			observations = append(observations, errMsg)
			if tca.monitor != nil {
				tca.monitor.LogToolResult(tc.Function.Name, nil, fmt.Errorf(errMsg))
			}
			continue
		}
		
		// Execute tool
		output, err := tool.Call(memoryTC.Arguments)
		if err != nil {
			errMsg := fmt.Sprintf("Error executing tool '%s': %s", tc.Function.Name, err.Error())
			observations = append(observations, errMsg)
			if tca.monitor != nil {
				tca.monitor.LogToolResult(tc.Function.Name, nil, err)
			}
		} else {
			observations = append(observations, fmt.Sprintf("Tool '%s' executed successfully", tc.Function.Name))
			lastOutput = output
			if tca.monitor != nil {
				tca.monitor.LogToolResult(tc.Function.Name, output, nil)
			}
		}
	}
	
	step.Observations = strings.Join(observations, "\n")
	if lastOutput != nil {
		step.ActionOutput = lastOutput
	}
	
	return &stepResult{
		isFinalAnswer: false,
		output:        lastOutput,
		tokenUsage:    step.TokenUsage,
	}, nil
}

// isFinalAnswerContent checks if content looks like a final answer
func (tca *ToolCallingAgent) isFinalAnswerContent(content string) bool {
	// Look for patterns that suggest this is a final answer
	finalAnswerPatterns := []string{
		"(?i)final answer",
		"(?i)in conclusion",
		"(?i)to summarize",
		"(?i)the answer is",
		"(?i)therefore",
	}
	
	for _, pattern := range finalAnswerPatterns {
		matched, _ := regexp.MatchString(pattern, content)
		if matched {
			return true
		}
	}
	
	return false
}

// initializeSystemPrompt sets up the default system prompt
func (tca *ToolCallingAgent) initializeSystemPrompt() {
	var toolDescriptions []string
	
	for _, tool := range tca.tools {
		desc := fmt.Sprintf("- %s: %s", tool.GetName(), tool.GetDescription())
		toolDescriptions = append(toolDescriptions, desc)
	}
	
	variables := map[string]interface{}{
		"tool_descriptions": strings.Join(toolDescriptions, "\n"),
	}
	
	systemPrompt := PopulateTemplate(DefaultToolCallingSystemPrompt, variables)
	tca.SetSystemPrompt(systemPrompt)
}

// getMessagesForResult converts memory to messages for the result
func (tca *ToolCallingAgent) getMessagesForResult() []map[string]interface{} {
	messages, err := tca.memory.WriteMemoryToMessages(false)
	if err != nil {
		// Return empty slice if error occurs
		return []map[string]interface{}{}
	}
	result := make([]map[string]interface{}, len(messages))
	
	for i, msg := range messages {
		result[i] = msg.ToDict()
	}
	
	return result
}

// ToDict implements MultiStepAgent for ToolCallingAgent
func (tca *ToolCallingAgent) ToDict() map[string]interface{} {
	result := tca.BaseMultiStepAgent.ToDict()
	result["agent_type"] = "tool_calling"
	return result
}

// Clone creates a copy of the ToolCallingAgent
func (tca *ToolCallingAgent) Clone() (*ToolCallingAgent, error) {
	// Create new agent with same configuration
	options := map[string]interface{}{
		"max_steps":         tca.maxSteps,
		"planning":          tca.planning,
		"planning_interval": tca.planningInterval,
		"prompt_templates":  tca.promptTemplates,
	}
	
	// Copy additional args
	for k, v := range tca.additionalArgs {
		options[k] = v
	}
	
	// Filter out the final_answer tool since NewToolCallingAgent will add it automatically
	toolsToClone := make([]tools.Tool, 0)
	for _, tool := range tca.tools {
		if tool.GetName() != "final_answer" {
			toolsToClone = append(toolsToClone, tool)
		}
	}
	
	clone, err := NewToolCallingAgent(tca.model, toolsToClone, tca.systemPrompt, options)
	if err != nil {
		return nil, err
	}
	
	// Copy managed agents
	for name, agent := range tca.managedAgents {
		clone.managedAgents[name] = agent
	}
	
	return clone, nil
}

// GetAvailableTools returns a list of available tool names
func (tca *ToolCallingAgent) GetAvailableTools() []string {
	names := make([]string, 0, len(tca.tools))
	for _, tool := range tca.tools {
		names = append(names, tool.GetName())
	}
	return names
}

// ValidateToolCall validates a tool call before execution
func (tca *ToolCallingAgent) ValidateToolCall(toolCall memory.ToolCall) error {
	// Check if tool exists
	tool, exists := tca.GetTool(toolCall.Name)
	if !exists {
		return utils.NewAgentToolCallError(fmt.Sprintf("tool '%s' not found", toolCall.Name))
	}
	
	// Validate tool arguments
	if err := tool.Validate(); err != nil {
		return utils.NewAgentToolCallError(fmt.Sprintf("tool validation failed: %v", err))
	}
	
	return nil
}
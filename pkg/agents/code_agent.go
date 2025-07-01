// Package agents - CodeAgent implementation
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/rizome-dev/go-smolagents/pkg/default_tools"
	"github.com/rizome-dev/go-smolagents/pkg/memory"
	"github.com/rizome-dev/go-smolagents/pkg/models"
	"github.com/rizome-dev/go-smolagents/pkg/tools"
	"github.com/rizome-dev/go-smolagents/pkg/utils"
)

// Default system prompt for CodeAgent
const DefaultCodeAgentSystemPrompt = `You are an expert assistant that can execute Go code to solve problems and answer questions.

You have access to a Go interpreter that can:
- Perform calculations and data analysis
- Execute Go code snippets
- Import allowed packages: {{authorized_packages}}
- Process data and perform computations

Your approach should be:
1. Analyze the problem or question
2. Write clean, well-commented Go code to solve it
3. Execute the code and examine the results
4. Provide a clear explanation of your findings

When writing code:
- Use descriptive variable names
- Add comments to explain complex logic
- Handle potential errors appropriately
- Use fmt.Print statements to show intermediate results when helpful
- Remember that variables persist between code executions

Use the go_interpreter tool to execute your code.`

// GoExecutor interface for executing Go code
type GoExecutor interface {
	Execute(code string, authorizedPackages []string) (interface{}, error)
	SendVariables(variables map[string]interface{}) error
	SendTools(tools map[string]tools.Tool) error
	GetState() map[string]interface{}
	Reset() error
}

// CodeAgent implements an agent specialized for code execution
type CodeAgent struct {
	*BaseMultiStepAgent
	goInterpreter      *default_tools.GoInterpreterTool
	goExecutor         GoExecutor
	authorizedPackages []string
	streamOutputs      bool
	structuredOutput   bool
	maxCodeLength      int
}

// NewCodeAgent creates a new code execution agent
func NewCodeAgent(
	model models.Model,
	toolsArg []tools.Tool,
	systemPrompt string,
	options map[string]interface{},
) (*CodeAgent, error) {

	// Set up authorized packages
	authorizedPackages := default_tools.BaseBuiltinPackages
	if options != nil {
		if packages, ok := options["authorized_packages"].([]string); ok {
			authorizedPackages = packages
		}
	}

	// Create Go interpreter tool
	goInterpreter := default_tools.NewGoInterpreterTool(authorizedPackages)

	// Combine tools with Go interpreter
	allTools := make([]tools.Tool, 0, len(toolsArg)+1)
	allTools = append(allTools, goInterpreter)
	if toolsArg != nil {
		allTools = append(allTools, toolsArg...)
	}

	// Create base agent
	baseAgent, err := NewBaseMultiStepAgent(model, allTools, systemPrompt, options)
	if err != nil {
		return nil, err
	}

	agent := &CodeAgent{
		BaseMultiStepAgent: baseAgent,
		goInterpreter:      goInterpreter,
		authorizedPackages: authorizedPackages,
		streamOutputs:      true,  // Default to streaming
		structuredOutput:   false, // Default to unstructured
		maxCodeLength:      10000, // Default max code length
	}

	// Apply CodeAgent-specific options
	if options != nil {
		if streamOutputs, ok := options["stream_outputs"].(bool); ok {
			agent.streamOutputs = streamOutputs
		}
		if structuredOutput, ok := options["structured_output"].(bool); ok {
			agent.structuredOutput = structuredOutput
		}
		if maxCodeLength, ok := options["max_code_length"].(int); ok {
			agent.maxCodeLength = maxCodeLength
		}
	}

	// Set default system prompt if none provided
	if systemPrompt == "" {
		agent.initializeSystemPrompt()
	}

	return agent, nil
}

// NewCodeAgentSimple creates a code agent with minimal configuration
func NewCodeAgentSimple(toolsArg []tools.Tool, model models.Model) (*CodeAgent, error) {
	return NewCodeAgent(model, toolsArg, "", nil)
}

// Run implements MultiStepAgent for CodeAgent
func (ca *CodeAgent) Run(options *RunOptions) (*RunResult, error) {
	if options == nil {
		return nil, utils.NewAgentError("run options cannot be nil")
	}

	// Set running state
	ca.isRunning = true
	defer func() { ca.isRunning = false }()

	// Start timing
	result := NewRunResult()

	// Reset if requested
	if options.Reset {
		ca.Reset()
	}

	// Set up context
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// Add task to memory
	taskStep := memory.NewTaskStep(options.Task, options.Images)
	ca.memory.AddStep(taskStep)

	// Determine max steps
	maxSteps := ca.maxSteps
	if options.MaxSteps != nil {
		maxSteps = *options.MaxSteps
	}

	// Execute agent steps
	for ca.stepCount < maxSteps {
		// Check for interruption
		if ca.interrupted {
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

		ca.stepCount++

		// Execute step
		stepResult, err := ca.executeCodeStep(ctx, ca.stepCount, options)
		if err != nil {
			result.State = "error"
			result.Error = err
			break
		}

		// Execute step callbacks
		if len(options.StepCallbacks) > 0 {
			latestStep := ca.memory.GetLastStep()
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
			result.StepCount = ca.stepCount
			result.TokenUsage = stepResult.tokenUsage
			break
		}
	}

	// Check if max steps reached
	if ca.stepCount >= maxSteps && result.State == "" {
		result.State = "max_steps_error"
		result.Error = utils.NewAgentMaxStepsError(fmt.Sprintf("reached maximum steps: %d", maxSteps))
	}

	// Finalize result
	result.StepCount = ca.stepCount
	result.Messages = ca.getMessagesForResult()
	result.Timing.End()

	return result, nil
}

// RunStream implements MultiStepAgent for CodeAgent
func (ca *CodeAgent) RunStream(options *RunOptions) (<-chan *StreamStepResult, error) {
	resultChan := make(chan *StreamStepResult, 100)

	go func() {
		defer close(resultChan)

		// Execute the agent and stream results
		result, err := ca.Run(options)
		if err != nil {
			resultChan <- &StreamStepResult{
				Error:      err,
				IsComplete: true,
			}
			return
		}

		// Send final result
		resultChan <- &StreamStepResult{
			StepNumber: ca.stepCount,
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

// executeCodeStep executes a single code agent step
func (ca *CodeAgent) executeCodeStep(ctx context.Context, stepNumber int, options *RunOptions) (*stepResult, error) {
	step := memory.NewActionStep(stepNumber)
	defer func() {
		step.Timing.End()
		ca.memory.AddStep(step)
	}()

	// Start monitoring
	if ca.monitor != nil {
		ca.monitor.StartStep(stepNumber, "code_execution")
		defer ca.monitor.EndStep()
	}

	// Prepare messages for the model
	messages, err := ca.memory.WriteMemoryToMessages(false)
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
		MaxTokens:   func() *int { v := 2048; return &v }(),
		Temperature: func() *float64 { v := 0.5; return &v }(), // Lower temperature for code
	}

	// Add structured output format if enabled
	if ca.structuredOutput {
		genOptions.ResponseFormat = models.CodeAgentResponseFormat
	}

	// Generate response
	response, err := ca.model.Generate(modelMessages, genOptions)
	if err != nil {
		step.Error = utils.NewAgentGenerationError("model generation failed: " + err.Error())
		return nil, step.Error
	}

	step.ModelOutputMessage = &models.ChatMessage{
		Role:    response.Role,
		Content: response.Content,
	}

	if response.Content != nil {
		step.ModelOutput = *response.Content
	}
	step.TokenUsage = response.TokenUsage

	// Add token usage to monitoring
	if ca.monitor != nil {
		ca.monitor.AddTokenUsage(response.TokenUsage)
	}

	// Process the response
	return ca.processCodeResponse(ctx, step, response)
}

// processCodeResponse processes the model response for code execution
func (ca *CodeAgent) processCodeResponse(ctx context.Context, step *memory.ActionStep, response *models.ChatMessage) (*stepResult, error) {
	if response.Content == nil {
		return &stepResult{isFinalAnswer: false, tokenUsage: response.TokenUsage}, nil
	}

	content := *response.Content

	// Check if this is structured output
	if ca.structuredOutput {
		return ca.processStructuredCodeResponse(ctx, step, content)
	}

	// Extract code from the response
	codeBlocks := utils.ParseCodeBlobs(content)

	var lastOutput interface{}
	var observations []string

	for _, block := range codeBlocks {
		// Validate code length
		if len(block.Code) > ca.maxCodeLength {
			errMsg := fmt.Sprintf("Code block too long: %d characters (max: %d)", len(block.Code), ca.maxCodeLength)
			observations = append(observations, errMsg)
			continue
		}

		// Log code execution
		if ca.monitor != nil {
			ca.monitor.LogToolCall("go_interpreter", map[string]interface{}{
				"code": block.Code,
			})
		}

		// Execute the code
		result, err := ca.goInterpreter.Call(block.Code)
		if err != nil {
			errMsg := fmt.Sprintf("Code execution error: %s", err.Error())
			observations = append(observations, errMsg)
			if ca.monitor != nil {
				ca.monitor.LogToolResult("go_interpreter", nil, err)
			}
		} else {
			observations = append(observations, fmt.Sprintf("Code executed successfully"))
			lastOutput = result
			if ca.monitor != nil {
				ca.monitor.LogToolResult("go_interpreter", result, nil)
			}

			// Check if the result looks like a final answer
			if ca.isFinalAnswerResult(result) {
				step.ActionOutput = result
				return &stepResult{
					isFinalAnswer: true,
					output:        result,
					tokenUsage:    response.TokenUsage,
				}, nil
			}
		}
	}

	// Store observations and output
	step.Observations = strings.Join(observations, "\n")
	if lastOutput != nil {
		step.ActionOutput = lastOutput
	}

	// Check if the content itself looks like a final answer
	if ca.isFinalAnswerContent(content) {
		return &stepResult{
			isFinalAnswer: true,
			output:        content,
			tokenUsage:    response.TokenUsage,
		}, nil
	}

	return &stepResult{
		isFinalAnswer: false,
		output:        lastOutput,
		tokenUsage:    response.TokenUsage,
	}, nil
}

// processStructuredCodeResponse processes structured JSON responses
func (ca *CodeAgent) processStructuredCodeResponse(ctx context.Context, step *memory.ActionStep, content string) (*stepResult, error) {
	// Parse the structured response
	var structured struct {
		Thought string `json:"thought"`
		Code    string `json:"code"`
	}

	if err := json.Unmarshal([]byte(content), &structured); err != nil {
		// Fall back to unstructured processing
		return ca.processCodeResponse(ctx, step, &models.ChatMessage{
			Content: &content,
		})
	}

	// Log the thought
	if structured.Thought != "" {
		step.ModelOutput = structured.Thought
	}

	// Execute the code
	if structured.Code != "" {
		// Validate code length
		if len(structured.Code) > ca.maxCodeLength {
			errMsg := fmt.Sprintf("Code too long: %d characters (max: %d)", len(structured.Code), ca.maxCodeLength)
			step.Observations = errMsg
			return &stepResult{isFinalAnswer: false}, nil
		}

		// Log code execution
		if ca.monitor != nil {
			ca.monitor.LogToolCall("go_interpreter", map[string]interface{}{
				"code": structured.Code,
			})
		}

		// Execute the code
		result, err := ca.goInterpreter.Call(structured.Code)
		if err != nil {
			step.Observations = fmt.Sprintf("Code execution error: %s", err.Error())
			if ca.monitor != nil {
				ca.monitor.LogToolResult("go_interpreter", nil, err)
			}
		} else {
			step.Observations = "Code executed successfully"
			step.ActionOutput = result
			if ca.monitor != nil {
				ca.monitor.LogToolResult("go_interpreter", result, nil)
			}

			// Check if the result looks like a final answer
			if ca.isFinalAnswerResult(result) {
				return &stepResult{
					isFinalAnswer: true,
					output:        result,
				}, nil
			}
		}
	}

	return &stepResult{isFinalAnswer: false}, nil
}

// isFinalAnswerContent checks if content looks like a final answer
func (ca *CodeAgent) isFinalAnswerContent(content string) bool {
	finalAnswerPatterns := []string{
		"(?i)final answer",
		"(?i)the answer is",
		"(?i)in conclusion",
		"(?i)to summarize",
		"(?i)the result is",
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

// isFinalAnswerResult checks if a code execution result looks like a final answer
func (ca *CodeAgent) isFinalAnswerResult(result interface{}) bool {
	if result == nil {
		return false
	}

	// Check if result contains final answer indicators
	resultStr := fmt.Sprintf("%v", result)
	return ca.isFinalAnswerContent(resultStr)
}

// initializeSystemPrompt sets up the default system prompt
func (ca *CodeAgent) initializeSystemPrompt() {
	variables := map[string]interface{}{
		"authorized_packages": strings.Join(ca.authorizedPackages, ", "),
	}

	systemPrompt := PopulateTemplate(DefaultCodeAgentSystemPrompt, variables)
	ca.SetSystemPrompt(systemPrompt)
}

// getMessagesForResult converts memory to messages for the result
func (ca *CodeAgent) getMessagesForResult() []map[string]interface{} {
	messages, err := ca.memory.WriteMemoryToMessages(false)
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

// ToDict implements MultiStepAgent for CodeAgent
func (ca *CodeAgent) ToDict() map[string]interface{} {
	result := ca.BaseMultiStepAgent.ToDict()
	result["agent_type"] = "code"
	result["authorized_packages"] = ca.authorizedPackages
	result["stream_outputs"] = ca.streamOutputs
	result["structured_output"] = ca.structuredOutput
	result["max_code_length"] = ca.maxCodeLength
	return result
}

// GetAuthorizedPackages returns the list of authorized packages
func (ca *CodeAgent) GetAuthorizedPackages() []string {
	return ca.authorizedPackages
}

// SetAuthorizedPackages sets the list of authorized packages
func (ca *CodeAgent) SetAuthorizedPackages(packages []string) {
	ca.authorizedPackages = packages

	// Update the Go interpreter tool
	ca.goInterpreter = default_tools.NewGoInterpreterTool(packages)

	// Update tools list
	for i, tool := range ca.tools {
		if tool.GetName() == "go_interpreter" {
			ca.tools[i] = ca.goInterpreter
			ca.toolsMap["go_interpreter"] = ca.goInterpreter
			break
		}
	}

	// Update system prompt
	if ca.systemPrompt != "" {
		ca.initializeSystemPrompt()
	}
}

// GetStreamOutputs returns whether output streaming is enabled
func (ca *CodeAgent) GetStreamOutputs() bool {
	return ca.streamOutputs
}

// SetStreamOutputs enables or disables output streaming
func (ca *CodeAgent) SetStreamOutputs(enabled bool) {
	ca.streamOutputs = enabled
}

// GetStructuredOutput returns whether structured output is enabled
func (ca *CodeAgent) GetStructuredOutput() bool {
	return ca.structuredOutput
}

// SetStructuredOutput enables or disables structured output
func (ca *CodeAgent) SetStructuredOutput(enabled bool) {
	ca.structuredOutput = enabled
}

// GetMaxCodeLength returns the maximum allowed code length
func (ca *CodeAgent) GetMaxCodeLength() int {
	return ca.maxCodeLength
}

// SetMaxCodeLength sets the maximum allowed code length
func (ca *CodeAgent) SetMaxCodeLength(length int) {
	ca.maxCodeLength = length
}

// ExecuteCode executes Go code directly
func (ca *CodeAgent) ExecuteCode(code string) (interface{}, error) {
	// Validate code length
	if len(code) > ca.maxCodeLength {
		return nil, utils.NewAgentError(fmt.Sprintf("code too long: %d characters (max: %d)", len(code), ca.maxCodeLength))
	}

	// Execute using the Go interpreter
	return ca.goInterpreter.Call(code)
}

// Close cleans up the CodeAgent resources
func (ca *CodeAgent) Close() error {
	if closer, ok := ca.goExecutor.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// Clone creates a copy of the CodeAgent
func (ca *CodeAgent) Clone() (*CodeAgent, error) {
	options := map[string]interface{}{
		"max_steps":           ca.maxSteps,
		"planning":            ca.planning,
		"planning_interval":   ca.planningInterval,
		"prompt_templates":    ca.promptTemplates,
		"authorized_packages": ca.authorizedPackages,
		"stream_outputs":      ca.streamOutputs,
		"structured_output":   ca.structuredOutput,
		"max_code_length":     ca.maxCodeLength,
	}

	// Copy additional args
	for k, v := range ca.additionalArgs {
		options[k] = v
	}

	clone, err := NewCodeAgent(ca.model, ca.tools[1:], ca.systemPrompt, options) // Skip go interpreter in tools
	if err != nil {
		return nil, err
	}

	// Copy managed agents
	for name, agent := range ca.managedAgents {
		clone.managedAgents[name] = agent
	}

	return clone, nil
}

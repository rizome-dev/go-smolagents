// Copyright 2025 Rizome Labs, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/display"
	"github.com/rizome-dev/go-smolagents/pkg/executors"
	"github.com/rizome-dev/go-smolagents/pkg/memory"
	"github.com/rizome-dev/go-smolagents/pkg/models"
	"github.com/rizome-dev/go-smolagents/pkg/monitoring"
	"github.com/rizome-dev/go-smolagents/pkg/parser"
	"github.com/rizome-dev/go-smolagents/pkg/prompts"
	"github.com/rizome-dev/go-smolagents/pkg/tools"
	"github.com/rizome-dev/go-smolagents/pkg/utils"
)

// ReactCodeAgent implements a full ReAct (Reasoning + Acting) agent for code execution
type ReactCodeAgent struct {
	*BaseMultiStepAgent
	promptManager      *prompts.PromptManager
	promptTemplate     *prompts.PromptTemplate
	responseParser     *parser.Parser
	goExecutor         *executors.GoExecutor
	authorizedPackages []string
	codeBlockTags      [2]string
	streamOutputs      bool
	structuredOutput   bool
	maxCodeLength      int
	enablePlanning     bool
	planningInterval   int
	verbose            bool
	display            *display.CharmDisplay
}

// ReactCodeAgentOptions configures the ReactCodeAgent
type ReactCodeAgentOptions struct {
	AuthorizedPackages []string
	CodeBlockTags      [2]string
	StreamOutputs      bool
	StructuredOutput   bool
	MaxCodeLength      int
	EnablePlanning     bool
	PlanningInterval   int
	MaxSteps           int
	Verbose            bool
}

// DefaultReactCodeAgentOptions returns default options for ReactCodeAgent
func DefaultReactCodeAgentOptions() *ReactCodeAgentOptions {
	return &ReactCodeAgentOptions{
		AuthorizedPackages: executors.DefaultAuthorizedPackages(),
		CodeBlockTags:      [2]string{"<code>", "</code>"},
		StreamOutputs:      true,
		StructuredOutput:   false,
		MaxCodeLength:      100000, // ~1000 lines * 100 chars/line
		EnablePlanning:     true,
		PlanningInterval:   5,
		MaxSteps:           15,
		Verbose:            false,
	}
}

// stepResult represents the result of a single ReAct step
type stepResult struct {
	isFinalAnswer bool
	output        interface{}
	tokenUsage    *monitoring.TokenUsage
}

// NewReactCodeAgent creates a new ReAct code execution agent
func NewReactCodeAgent(
	model models.Model,
	toolsArg []tools.Tool,
	systemPrompt string,
	options *ReactCodeAgentOptions,
) (*ReactCodeAgent, error) {
	if options == nil {
		options = DefaultReactCodeAgentOptions()
	}

	// Create prompt manager
	promptManager, err := prompts.NewPromptManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt manager: %w", err)
	}

	// Get the appropriate prompt template
	templateName := "code_agent"
	if options.StructuredOutput {
		templateName = "structured_code_agent"
	}

	promptTemplate, err := promptManager.GetTemplate(templateName)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt template: %w", err)
	}

	// Create response parser
	responseParser := parser.NewParserWithTags(options.CodeBlockTags[0], options.CodeBlockTags[1])

	// Create Go executor
	execOptions := map[string]interface{}{
		"authorized_packages": options.AuthorizedPackages,
	}
	goExecutor, err := executors.NewGoExecutor(execOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create Go executor: %w", err)
	}

	// Send tools to executor
	if len(toolsArg) > 0 {
		toolsMap := make(map[string]tools.Tool)
		for _, tool := range toolsArg {
			toolsMap[tool.GetName()] = tool
		}
		if err := goExecutor.SendTools(toolsMap); err != nil {
			return nil, fmt.Errorf("failed to send tools to executor: %w", err)
		}
	}

	// Create base agent options
	baseOptions := map[string]interface{}{
		"max_steps":         options.MaxSteps,
		"planning":          options.EnablePlanning,
		"planning_interval": options.PlanningInterval,
		"verbose":           options.Verbose,
	}

	// Create base agent (no tools needed as we use the executor directly)
	baseAgent, err := NewBaseMultiStepAgent(model, nil, systemPrompt, baseOptions)
	if err != nil {
		return nil, err
	}

	// Handle zero values for maxCodeLength
	maxCodeLength := options.MaxCodeLength
	if maxCodeLength == 0 {
		// Default to 100k chars (approximately 1000 lines * 100 chars/line)
		maxCodeLength = 100000
	}

	agent := &ReactCodeAgent{
		BaseMultiStepAgent: baseAgent,
		promptManager:      promptManager,
		promptTemplate:     promptTemplate,
		responseParser:     responseParser,
		goExecutor:         goExecutor,
		authorizedPackages: options.AuthorizedPackages,
		codeBlockTags:      options.CodeBlockTags,
		streamOutputs:      options.StreamOutputs,
		structuredOutput:   options.StructuredOutput,
		maxCodeLength:      maxCodeLength,
		enablePlanning:     options.EnablePlanning,
		planningInterval:   options.PlanningInterval,
		verbose:            options.Verbose,
		display:            display.NewCharmDisplay(options.Verbose),
	}

	// Initialize system prompt if not provided
	if systemPrompt == "" {
		agent.initializeSystemPrompt()
	}

	return agent, nil
}

// Run implements the ReAct reasoning loop for code execution
// prepareRunContext sets up the initial context and state for execution
func (rca *ReactCodeAgent) prepareRunContext(options *RunOptions) (context.Context, *RunResult, error) {
	if options == nil {
		return nil, nil, utils.NewAgentError("run options cannot be nil")
	}

	// Set running state
	rca.isRunning = true

	// Start timing
	result := NewRunResult()

	// Reset if requested
	if options.Reset {
		rca.Reset()
		if err := rca.goExecutor.Reset(); err != nil {
			return nil, nil, fmt.Errorf("failed to reset executor: %w", err)
		}
	}

	// Set up context
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}

	return ctx, result, nil
}

// processTaskImages converts and loads images from options
func (rca *ReactCodeAgent) processTaskImages(images []interface{}) []*models.MediaContent {
	var taskImages []*models.MediaContent
	for _, img := range images {
		if imgStr, ok := img.(string); ok {
			mediaContent, _ := models.LoadImageURL(imgStr, "auto")
			if mediaContent != nil {
				taskImages = append(taskImages, mediaContent)
			}
		}
	}
	return taskImages
}

// checkStepConditions checks interruption and context cancellation
func (rca *ReactCodeAgent) checkStepConditions(ctx context.Context, result *RunResult) bool {
	// Check for interruption
	if rca.interrupted {
		result.State = "interrupted"
		result.Error = utils.NewAgentExecutionError("agent execution was interrupted")
		return false
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		result.State = "cancelled"
		result.Error = ctx.Err()
		return false
	default:
		return true
	}
}

// executeStepCallbacks runs step callbacks if provided
func (rca *ReactCodeAgent) executeStepCallbacks(options *RunOptions, result *RunResult) error {
	if len(options.StepCallbacks) == 0 {
		return nil
	}

	latestStep := rca.memory.GetLastStep()
	if latestStep == nil {
		return nil
	}

	for _, callback := range options.StepCallbacks {
		if err := callback(latestStep); err != nil {
			result.State = "callback_error"
			result.Error = fmt.Errorf("step callback error: %w", err)
			return err
		}
	}

	return nil
}

// displayMaxStepsError shows helpful error message when max steps reached
func (rca *ReactCodeAgent) displayMaxStepsError() {
	rca.display.Rule("Max Steps Reached")
	rca.display.Error(utils.NewAgentMaxStepsError("Maximum step limit reached"))
	rca.display.Info("💡 Tip: The agent was unable to complete the task within the step limit. Consider:")
	rca.display.Info("   - Simplifying the task")
	rca.display.Info("   - Increasing max_steps")
	rca.display.Info("   - Providing more specific instructions")
}

// displayFinalSummary shows the final execution summary
func (rca *ReactCodeAgent) displayFinalSummary(result *RunResult) {
	duration := time.Since(result.Timing.StartTime)
	if result.State == "success" {
		rca.display.Success(fmt.Sprintf("Task completed successfully in %d steps (%.2fs)",
			result.StepCount, duration.Seconds()))
	} else if result.State != "" {
		rca.display.Info(fmt.Sprintf("Task ended with state: %s after %d steps (%.2fs)",
			result.State, result.StepCount, duration.Seconds()))
	}
}

func (rca *ReactCodeAgent) Run(options *RunOptions) (*RunResult, error) {
	ctx, result, err := rca.prepareRunContext(options)
	if err != nil {
		return nil, err
	}
	defer func() { rca.isRunning = false }()

	// Process and add task to memory
	taskImages := rca.processTaskImages(options.Images)
	taskStep := memory.NewTaskStep(options.Task, taskImages)
	rca.memory.AddStep(taskStep)

	// Determine max steps
	maxSteps := rca.maxSteps
	if options.MaxSteps != nil {
		maxSteps = *options.MaxSteps
	}

	// Display the task prominently
	rca.display.Task(options.Task, fmt.Sprintf("Max steps: %d | Agent: ReactCodeAgent", maxSteps))

	// Execute ReAct loop
	for rca.stepCount < maxSteps {
		if !rca.checkStepConditions(ctx, result) {
			if result.State == "cancelled" {
				result.Timing.End()
				return result, nil
			}
			break
		}

		rca.stepCount++

		// Check if planning is needed
		if rca.enablePlanning && rca.stepCount%rca.planningInterval == 1 {
			if err := rca.executePlanningStep(ctx); err != nil {
				result.State = "planning_error"
				result.Error = err
				break
			}
		}

		// Execute ReAct step
		stepResult, err := rca.executeReactStep(ctx, rca.stepCount, options)
		if err != nil {
			result.State = "error"
			result.Error = err
			rca.display.Error(err)
			break
		}

		// Execute step callbacks
		if err := rca.executeStepCallbacks(options, result); err != nil {
			result.Timing.End()
			return result, err
		}

		// Check for final answer
		if stepResult.isFinalAnswer {
			result.State = "success"
			result.Output = stepResult.output
			result.StepCount = rca.stepCount
			result.TokenUsage = stepResult.tokenUsage
			break
		}
	}

	// Check if max steps reached
	if rca.stepCount >= maxSteps && result.State == "" {
		result.State = "max_steps_error"
		result.Error = utils.NewAgentMaxStepsError(fmt.Sprintf("reached maximum steps: %d", maxSteps))
		rca.displayMaxStepsError()
	}

	// Finalize result
	result.StepCount = rca.stepCount
	result.Messages = rca.getMessagesForResult()
	result.Timing.End()

	// Display final summary
	rca.displayFinalSummary(result)

	return result, nil
}

// executeReactStep executes a single ReAct step with retry logic
// Helper functions to reduce cyclomatic complexity

// preparePrompts builds the system and task prompts
func (rca *ReactCodeAgent) preparePrompts(options *RunOptions) (string, string, []string, error) {
	promptBuilder := prompts.NewPromptBuilder(rca.promptTemplate).
		WithVariable("task", options.Task).
		WithVariable("tool_descriptions", rca.getToolDescriptions()).
		WithVariable("memory", rca.getMemoryString()).
		WithVariable("code_block_opening_tag", rca.codeBlockTags[0]).
		WithVariable("code_block_closing_tag", rca.codeBlockTags[1])

	systemPrompt, err := promptBuilder.BuildSystemPrompt()
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to build system prompt: %w", err)
	}

	taskPrompt, err := promptBuilder.BuildTaskPrompt()
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to build task prompt: %w", err)
	}

	stopSequences, err := promptBuilder.GetStopSequences()
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to get stop sequences: %w", err)
	}

	return systemPrompt, taskPrompt, stopSequences, nil
}

// shouldSkipMessage determines if a message should be skipped
func (rca *ReactCodeAgent) shouldSkipMessage(msg *memory.Message) bool {
	if msg.Role == "system" {
		return true
	}

	if len(msg.Content) == 0 {
		return true
	}

	if textContent, ok := msg.Content[0]["text"].(string); ok {
		if textContent == "" || strings.HasPrefix(textContent, "Task:") || strings.HasPrefix(textContent, "New task:") {
			return true
		}
	}

	return false
}

// hasValidContent checks if a message has valid content
func (rca *ReactCodeAgent) hasValidContent(msgDict map[string]interface{}) bool {
	if content, ok := msgDict["content"].([]map[string]interface{}); ok && len(content) > 0 {
		for _, item := range content {
			if text, ok := item["text"].(string); ok && text != "" {
				return true
			}
		}
	}

	if toolCalls, ok := msgDict["tool_calls"].([]map[string]interface{}); ok && len(toolCalls) > 0 {
		return true
	}

	return false
}

// prepareMessages builds the message list for the model
func (rca *ReactCodeAgent) prepareMessages(systemPrompt, taskPrompt string) ([]interface{}, []memory.Message, error) {
	messages := []interface{}{
		map[string]interface{}{
			"role":    "system",
			"content": systemPrompt,
		},
		map[string]interface{}{
			"role":    "user",
			"content": taskPrompt,
		},
	}

	memoryMessages, err := rca.memory.WriteMemoryToMessages(false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write memory to messages: %w", err)
	}

	for _, msg := range memoryMessages {
		if rca.shouldSkipMessage(&msg) {
			continue
		}

		msgDict := msg.ToDict()
		if rca.hasValidContent(msgDict) {
			messages = append(messages, msgDict)
		}
	}

	return messages, memoryMessages, nil
}

// buildGenerationOptions creates model generation options
func (rca *ReactCodeAgent) buildGenerationOptions(stopSequences []string) *models.GenerateOptions {
	genOptions := &models.GenerateOptions{
		MaxTokens:   func() *int { v := 2048; return &v }(),
		Temperature: func() *float64 { v := 0.3; return &v }(),
	}

	if models.SupportsStopParameter(rca.model.GetModelID()) {
		genOptions.StopSequences = stopSequences
	}

	if rca.structuredOutput && rca.promptTemplate.ResponseSchema != nil {
		genOptions.ResponseFormat = &models.ResponseFormat{
			Type: "json_object",
			JSONSchema: &models.JSONSchema{
				Name:        "code_agent_response",
				Description: "Structured response for code agent",
				Schema:      rca.promptTemplate.ResponseSchema,
				Strict:      true,
			},
		}
	}

	return genOptions
}

// validateResponse checks if a response is valid
func (rca *ReactCodeAgent) validateResponse(response *models.ChatMessage) error {
	if response == nil {
		return fmt.Errorf("received nil response from model")
	}
	if response.Content == nil || *response.Content == "" {
		return fmt.Errorf("received empty content from model")
	}
	return nil
}

// executeWithRetries performs model generation with retry logic
func (rca *ReactCodeAgent) executeWithRetries(messages []interface{}, genOptions *models.GenerateOptions) (*models.ChatMessage, error) {
	maxRetries := 3
	var response *models.ChatMessage
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			waitTime := time.Duration(2<<(attempt-1)) * time.Second
			if rca.verbose {
				rca.display.Info(fmt.Sprintf("Retrying after %v (attempt %d/%d)...", waitTime, attempt+1, maxRetries))
			}
			time.Sleep(waitTime)
		}

		if rca.verbose && attempt > 0 {
			rca.display.Info(fmt.Sprintf("Calling model.Generate() - attempt %d/%d", attempt+1, maxRetries))
		}

		response, err = rca.model.Generate(messages, genOptions)
		if err == nil {
			if validErr := rca.validateResponse(response); validErr != nil {
				err = validErr
				if rca.verbose {
					rca.display.Error(err)
				}
				continue
			}
			break
		}

		if rca.verbose {
			rca.display.Error(fmt.Errorf("Model generation attempt %d failed: %v", attempt+1, err))
		}
	}

	if err != nil {
		return nil, fmt.Errorf("model generation failed after %d attempts: %v", maxRetries, err)
	}

	return response, nil
}

// cleanModelOutput removes system-generated content
func (rca *ReactCodeAgent) cleanModelOutput(output string) string {
	if idx := strings.Index(output, "Observation:"); idx >= 0 {
		if rca.verbose {
			rca.display.Info("Cleaned model output by removing generated 'Observation:' content")
		}
		return strings.TrimSpace(output[:idx])
	}
	return output
}

// updateMetrics updates monitoring and display metrics
func (rca *ReactCodeAgent) updateMetrics(step *memory.ActionStep, response *models.ChatMessage) {
	if rca.monitor != nil {
		rca.monitor.AddTokenUsage(response.TokenUsage)
	}

	if rca.verbose && response.TokenUsage != nil {
		duration := time.Since(step.Timing.StartTime)
		rca.display.Metrics(step.StepNumber, duration, response.TokenUsage.InputTokens, response.TokenUsage.OutputTokens)
	}
}

func (rca *ReactCodeAgent) executeReactStep(ctx context.Context, stepNumber int, options *RunOptions) (*stepResult, error) {
	step := memory.NewActionStep(stepNumber)
	defer func() {
		step.Timing.End()
		rca.memory.AddStep(step)
	}()

	// Display step header
	rca.display.Rule(fmt.Sprintf("Step %d", stepNumber))
	rca.display.Progress("Generating response...")

	// Start monitoring
	if rca.monitor != nil {
		rca.monitor.StartStep(stepNumber, "react_code_step")
		defer rca.monitor.EndStep()
	}

	// Prepare prompts
	systemPrompt, taskPrompt, stopSequences, err := rca.preparePrompts(options)
	if err != nil {
		return nil, err
	}

	// Prepare messages
	messages, memoryMessages, err := rca.prepareMessages(systemPrompt, taskPrompt)
	if err != nil {
		return nil, err
	}
	step.ModelInputMessages = memoryMessages

	// Build generation options
	genOptions := rca.buildGenerationOptions(stopSequences)

	// Execute with retries
	response, err := rca.executeWithRetries(messages, genOptions)
	if err != nil {
		step.Error = utils.NewAgentGenerationError(err.Error())
		return nil, step.Error
	}

	// Process response
	if response.Content != nil {
		step.ModelOutput = *response.Content
		cleanedOutput := rca.cleanModelOutput(step.ModelOutput)
		if cleanedOutput != step.ModelOutput {
			step.ModelOutput = cleanedOutput
			response.Content = &cleanedOutput
		}
	}

	step.ModelOutputMessage = &models.ChatMessage{
		Role:    response.Role,
		Content: response.Content,
	}
	step.TokenUsage = response.TokenUsage

	// Update metrics
	rca.updateMetrics(step, response)

	// Parse and execute the response
	return rca.processReactResponse(ctx, step, response)
}

// Helper function to create error results
func (rca *ReactCodeAgent) createErrorResult(step *memory.ActionStep, errMsg string, tokenUsage *monitoring.TokenUsage) *stepResult {
	if rca.verbose {
		rca.display.Error(fmt.Errorf(errMsg))
	}
	step.Error = utils.NewAgentError(errMsg)
	step.Observations = "Error: " + errMsg
	return &stepResult{isFinalAnswer: false, tokenUsage: tokenUsage}
}

// validateAndExtractContent validates response and extracts content
func (rca *ReactCodeAgent) validateAndExtractContent(response *models.ChatMessage, step *memory.ActionStep) (string, *stepResult) {
	if response == nil {
		return "", rca.createErrorResult(step, "received nil response from processReactResponse", nil)
	}

	if response.Content == nil {
		return "", rca.createErrorResult(step, "received nil content in response", response.TokenUsage)
	}

	content := *response.Content
	if content == "" {
		return "", rca.createErrorResult(step, "received empty content string", response.TokenUsage)
	}

	return content, nil
}

// handleRawParseResult processes raw parse results
func (rca *ReactCodeAgent) handleRawParseResult(ctx context.Context, step *memory.ActionStep, parseResult *parser.ParseResult, content string, tokenUsage *monitoring.TokenUsage) (*stepResult, error) {
	// Check if there's code or final_answer hidden in the content
	if strings.Contains(content, "final_answer(") || strings.Contains(content, "final_answer (") {
		if rca.verbose {
			rca.display.Info("Detected final_answer in raw content, attempting extraction")
		}
		codeResult := &parser.ParseResult{
			Type:    "code",
			Content: extractCodeFromRaw(content),
			Thought: parseResult.Thought,
		}
		return rca.handleCodeExecution(ctx, step, codeResult)
	}

	// Check for code blocks that might have been missed
	if strings.Contains(content, rca.codeBlockTags[0]) {
		if rca.verbose {
			rca.display.Info("Found code tags in raw content, re-attempting parse")
		}
		if code := extractCodeBetweenTags(content, rca.codeBlockTags[0], rca.codeBlockTags[1]); code != "" {
			codeResult := &parser.ParseResult{
				Type:    "code",
				Content: code,
				Thought: parseResult.Thought,
			}
			return rca.handleCodeExecution(ctx, step, codeResult)
		}
	}

	// Otherwise treat as observation/thinking
	if rca.verbose {
		rca.display.Info("Processing as raw thought/observation")
	}
	step.Observations = content
	rca.display.ModelOutput(content)
	return &stepResult{isFinalAnswer: false, tokenUsage: tokenUsage}, nil
}

// processReactResponse processes the model response following ReAct pattern
func (rca *ReactCodeAgent) processReactResponse(ctx context.Context, step *memory.ActionStep, response *models.ChatMessage) (*stepResult, error) {
	// Validate and extract content
	content, errorResult := rca.validateAndExtractContent(response, step)
	if errorResult != nil {
		return errorResult, nil
	}

	// Log the raw content if verbose
	if rca.verbose {
		rca.display.Info(fmt.Sprintf("Raw model output (length=%d): %s", len(content), truncateForLogging(content, 200)))
	}

	// Parse the response with error recovery
	parseResult := rca.responseParser.Parse(content)
	if parseResult == nil {
		if rca.verbose {
			rca.display.Error(fmt.Errorf("parser returned nil result"))
		}
		parseResult = &parser.ParseResult{
			Type:    "raw",
			Content: content,
			Thought: extractThoughtFromRaw(content),
		}
	}

	// Always display the raw model output first
	if rca.verbose && content != "" {
		rca.display.ModelOutput(content)
	}

	// Display the thought if present
	if parseResult.Thought != "" {
		rca.display.Thought(parseResult.Thought)
		step.ModelOutput = parseResult.Thought
	}

	// Debug logging for parse result
	if rca.verbose {
		rca.display.Info(fmt.Sprintf("Parse result type: %s, has_content: %v, has_error: %v",
			parseResult.Type, parseResult.Content != "", parseResult.Error != nil))
		if parseResult.Error != nil {
			rca.display.Error(fmt.Errorf("Parse error: %v", parseResult.Error))
		}
	}

	// Handle different parse result types
	switch parseResult.Type {
	case "code", "structured":
		return rca.handleCodeExecution(ctx, step, parseResult)
	case "final_answer":
		return rca.handleFinalAnswer(step, parseResult)
	case "error":
		step.Error = utils.NewAgentError(parseResult.Content)
		step.Observations = fmt.Sprintf("Parse error: %s", parseResult.Content)
		rca.display.Error(step.Error)
		return &stepResult{isFinalAnswer: false, tokenUsage: response.TokenUsage}, nil
	case "raw":
		return rca.handleRawParseResult(ctx, step, parseResult, content, response.TokenUsage)
	default:
		// Unknown parse result type
		if rca.verbose {
			rca.display.Info(fmt.Sprintf("Unknown parse result type: %s, treating as raw", parseResult.Type))
		}
		step.Observations = content
		rca.display.ModelOutput(content)
		return &stepResult{isFinalAnswer: false, tokenUsage: response.TokenUsage}, nil
	}
}

// validateCodeLength checks if code exceeds maximum length
func (rca *ReactCodeAgent) validateCodeLength(code string, step *memory.ActionStep) *stepResult {
	if len(code) > rca.maxCodeLength {
		errMsg := fmt.Sprintf("Code block too long: %d characters (max: %d)", len(code), rca.maxCodeLength)
		step.Observations = errMsg
		rca.display.Error(fmt.Errorf(errMsg))
		return &stepResult{isFinalAnswer: false}
	}
	return nil
}

// logCodeExecution logs code execution to monitor
func (rca *ReactCodeAgent) logCodeExecution(code string) {
	if rca.monitor != nil {
		rca.monitor.LogToolCall("go_executor", map[string]interface{}{
			"code": code,
		})
	}
}

// handleExecutionError processes code execution errors
func (rca *ReactCodeAgent) handleExecutionError(err error, execResult *executors.ExecutionResult, step *memory.ActionStep) *stepResult {
	errMsg := fmt.Sprintf("Code execution error: %s", err.Error())
	step.Observations = errMsg
	
	if execResult != nil && execResult.Stderr != "" {
		step.Observations += fmt.Sprintf("\nStderr: %s", execResult.Stderr)
		rca.display.Error(fmt.Errorf("Stderr: %s", execResult.Stderr))
	}
	
	rca.display.Error(err)
	if rca.monitor != nil {
		rca.monitor.LogToolResult("go_executor", nil, err)
	}
	
	return &stepResult{isFinalAnswer: false}
}

// formatObservation creates observation string from execution result
func (rca *ReactCodeAgent) formatObservation(execResult *executors.ExecutionResult) string {
	var observationParts []string
	observationParts = append(observationParts, "=== Code Execution ===")

	// Add execution logs if present
	if execResult.Logs != "" {
		observationParts = append(observationParts, "Execution logs:")
		observationParts = append(observationParts, execResult.Logs)
	}

	// Add output if present
	if execResult.Output != nil {
		outputStr := fmt.Sprintf("%v", execResult.Output)
		if outputStr != "" && outputStr != "<nil>" {
			observationParts = append(observationParts, "\nLast output from code snippet:")
			if len(outputStr) > 500 {
				outputStr = outputStr[:500] + "...[truncated]"
			}
			observationParts = append(observationParts, outputStr)
		}
	}

	// If neither logs nor output, indicate success
	if execResult.Logs == "" && execResult.Output == nil {
		observationParts = append(observationParts, "Code executed successfully with no output")
	}

	return strings.Join(observationParts, "\n")
}

// handleCodeExecution executes parsed code
func (rca *ReactCodeAgent) handleCodeExecution(ctx context.Context, step *memory.ActionStep, parseResult *parser.ParseResult) (*stepResult, error) {
	code := parseResult.Content

	// Validate code length
	if result := rca.validateCodeLength(code, step); result != nil {
		return result, nil
	}

	// Display the code block
	rca.display.Rule("Code")
	rca.display.Code("Executing parsed code:", code)

	// Log code execution
	rca.logCodeExecution(code)

	// Execute the code
	startTime := time.Now()
	execResult, err := rca.goExecutor.ExecuteWithResult(code)
	executionDuration := time.Since(startTime)

	// Display execution duration
	if rca.verbose {
		rca.display.Info(fmt.Sprintf("Execution time: %.3fs", executionDuration.Seconds()))
	}

	// Handle execution error
	if err != nil {
		return rca.handleExecutionError(err, execResult, step), nil
	}

	// Check if it's a final answer
	if execResult.IsFinalAnswer {
		step.ActionOutput = execResult.FinalAnswer
		step.Observations = "Final answer provided"
		rca.display.FinalAnswer(execResult.FinalAnswer)
		return &stepResult{
			isFinalAnswer: true,
			output:        execResult.FinalAnswer,
		}, nil
	}

	// Format observation
	step.ActionOutput = execResult.Output
	step.Observations = rca.formatObservation(execResult)

	// Display the full observation
	rca.display.Observation(step.Observations)

	if rca.monitor != nil {
		rca.monitor.LogToolResult("go_executor", step.ActionOutput, nil)
	}

	return &stepResult{
		isFinalAnswer: false,
		output:        step.ActionOutput,
	}, nil
}

// handleFinalAnswer processes a final answer
func (rca *ReactCodeAgent) handleFinalAnswer(step *memory.ActionStep, parseResult *parser.ParseResult) (*stepResult, error) {
	var finalAnswer interface{}

	if parseResult.Type == "final_answer" {
		if parseResult.Action != nil && parseResult.Action["arguments"] != nil {
			finalAnswer = parseResult.Action["arguments"]
		} else {
			finalAnswer = parseResult.Content
		}
	}

	step.ActionOutput = finalAnswer
	step.Observations = "Final answer provided"

	// Display the final answer
	rca.display.FinalAnswer(finalAnswer)

	return &stepResult{
		isFinalAnswer: true,
		output:        finalAnswer,
	}, nil
}

// executePlanningStep executes a planning step
func (rca *ReactCodeAgent) executePlanningStep(ctx context.Context) error {
	// Display planning header
	rca.display.Rule("Planning")
	rca.display.Progress("Generating plan...")

	// Get task from memory
	task := ""
	steps := rca.memory.GetSteps()
	if len(steps) > 0 {
		if taskStep, ok := steps[0].(*memory.TaskStep); ok {
			task = taskStep.Task
		}
	}

	// Build planning prompt
	promptBuilder := prompts.NewPromptBuilder(rca.promptTemplate).
		WithVariable("task", task).
		WithVariable("memory", rca.getMemoryString())

	planningPrompt, err := promptBuilder.BuildPlanningPrompt()
	if err != nil {
		return fmt.Errorf("failed to build planning prompt: %w", err)
	}

	// Get current messages and add planning prompt
	messages, err := rca.memory.WriteMemoryToMessages(false)
	if err != nil {
		return fmt.Errorf("failed to write memory to messages: %w", err)
	}

	// Convert to model format
	modelMessages := make([]interface{}, len(messages))
	for i, msg := range messages {
		modelMessages[i] = msg.ToDict()
	}

	// Add planning prompt
	modelMessages = append(modelMessages, map[string]interface{}{
		"role":    "user",
		"content": planningPrompt,
	})

	// Generate planning response
	genOptions := &models.GenerateOptions{
		MaxTokens:   func() *int { v := 1024; return &v }(),
		Temperature: func() *float64 { v := 0.5; return &v }(),
	}

	// Only add stop sequences if the model supports them
	if models.SupportsStopParameter(rca.model.GetModelID()) {
		genOptions.StopSequences = []string{"<end_plan>"}
	}

	// Add debug logging
	if rca.verbose {
		rca.display.Info(fmt.Sprintf("Planning prompt length: %d characters", len(planningPrompt)))
		rca.display.Info(fmt.Sprintf("Number of messages being sent: %d", len(modelMessages)))
		// Log the planning prompt content for debugging
		if len(planningPrompt) > 200 {
			rca.display.Info(fmt.Sprintf("Planning prompt preview: %s...", planningPrompt[:200]))
		} else {
			rca.display.Info(fmt.Sprintf("Planning prompt: %s", planningPrompt))
		}
	}

	response, err := rca.model.Generate(modelMessages, genOptions)
	if err != nil {
		return fmt.Errorf("planning generation failed: %w", err)
	}

	// Debug logging
	if rca.verbose {
		if response != nil && response.Content != nil {
			rca.display.Info(fmt.Sprintf("Received planning response (length: %d)", len(*response.Content)))
		} else {
			rca.display.Info("Received nil or empty planning response")
		}
	}

	// Create planning step with proper arguments
	var planContent string
	if response.Content != nil {
		planContent = *response.Content
		// Clean the plan content by removing <end_plan> tag
		if idx := strings.Index(planContent, "<end_plan>"); idx >= 0 {
			planContent = strings.TrimSpace(planContent[:idx])
		}
		// Display the planning output
		rca.display.Planning(planContent)
	}

	step := memory.NewPlanningStep(
		messages,
		*response,
		planContent,
		monitoring.Timing{StartTime: time.Now()},
		response.TokenUsage,
	)
	step.Timing.End()

	rca.memory.AddStep(step)

	return nil
}

// initializeSystemPrompt sets up the default system prompt
func (rca *ReactCodeAgent) initializeSystemPrompt() {
	variables := map[string]interface{}{
		"authorized_packages":    strings.Join(rca.authorizedPackages, ", "),
		"tool_descriptions":      rca.getToolDescriptions(),
		"code_block_opening_tag": rca.codeBlockTags[0],
		"code_block_closing_tag": rca.codeBlockTags[1],
		"additional_prompting":   "", // Optional additional instructions
		"agent_description":      "I am a ReactCodeAgent that can execute Go code to solve problems step by step.",
	}

	// Build system prompt using the template
	promptBuilder := prompts.NewPromptBuilder(rca.promptTemplate).WithVariables(variables)
	systemPrompt, err := promptBuilder.BuildSystemPrompt()
	if err != nil {
		// Fall back to a simple default
		systemPrompt = "You are an expert AI assistant that can solve problems by writing and executing Go code using a ReAct reasoning approach."
	}

	rca.SetSystemPrompt(systemPrompt)
}

// getToolDescriptions returns formatted tool descriptions
func (rca *ReactCodeAgent) getToolDescriptions() string {
	descriptions := []string{
		"- go_executor: Execute Go code with persistent state between executions",
	}

	for _, tool := range rca.tools {
		descriptions = append(descriptions, fmt.Sprintf("- %s: %s", tool.GetName(), tool.GetDescription()))
	}

	// Add final_answer function description
	descriptions = append(descriptions, "- final_answer(answer): Provide the final answer to the task and complete execution")

	return strings.Join(descriptions, "\n")
}

// getMemoryString returns a formatted string of the conversation memory
func (rca *ReactCodeAgent) getMemoryString() string {
	steps := rca.memory.GetSteps()
	if len(steps) <= 1 { // Only task step
		return "No previous steps."
	}

	var parts []string
	for i, step := range steps {
		if i == 0 { // Skip the task step
			continue
		}

		switch s := step.(type) {
		case *memory.ActionStep:
			// Format like Python: include step number and all relevant info
			stepInfo := fmt.Sprintf("=== Step %d ===", s.StepNumber)
			parts = append(parts, stepInfo)

			if s.ModelOutput != "" {
				parts = append(parts, fmt.Sprintf("Thought: %s", s.ModelOutput))
			}

			// If we have observations that include code execution info, extract it
			if strings.Contains(s.Observations, "=== Code Execution ===") {
				// The code is already shown in the thought, so we don't need to duplicate it
			}

			if s.Observations != "" {
				parts = append(parts, fmt.Sprintf("Observation: %s", s.Observations))
			}

			if s.Error != nil {
				parts = append(parts, fmt.Sprintf("Error: %v", s.Error))
			}
		case *memory.PlanningStep:
			parts = append(parts, "=== Planning ===")
			if s.Plan != "" {
				parts = append(parts, s.Plan)
			}
		}
	}

	if len(parts) == 0 {
		return "No previous steps."
	}

	return strings.Join(parts, "\n")
}

// getMessagesForResult converts memory to messages for the result
func (rca *ReactCodeAgent) getMessagesForResult() []map[string]interface{} {
	messages, err := rca.memory.WriteMemoryToMessages(false)
	if err != nil {
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		result[i] = msg.ToDict()
	}

	return result
}

// RunStream implements streaming for ReactCodeAgent
func (rca *ReactCodeAgent) RunStream(options *RunOptions) (<-chan *StreamStepResult, error) {
	resultChan := make(chan *StreamStepResult, 100)

	// Apply defaults
	if options == nil {
		options = &RunOptions{}
	}
	if options.Task == "" {
		close(resultChan)
		return resultChan, fmt.Errorf("task is required")
	}

	go func() {
		defer close(resultChan)

		// Initialize agent state
		if options.Reset {
			rca.Reset()
		}

		// Add task to memory
		// Convert images if provided
		var taskImages []*models.MediaContent
		if len(options.Images) > 0 {
			for _, img := range options.Images {
				// Assume images are provided as URLs or base64 strings
				if imgStr, ok := img.(string); ok {
					mediaContent, _ := models.LoadImageURL(imgStr, "auto")
					if mediaContent != nil {
						taskImages = append(taskImages, mediaContent)
					}
				}
			}
		}
		taskStep := memory.NewTaskStep(options.Task, taskImages)
		rca.memory.AddStep(taskStep)

		// Determine max steps
		maxSteps := rca.maxSteps
		if options.MaxSteps != nil {
			maxSteps = *options.MaxSteps
		}

		// Main execution loop
		stepNumber := 0
		finalAnswer := ""
		for stepNumber < maxSteps {
			stepNumber++
			rca.stepCount++

			// Send step start event
			resultChan <- &StreamStepResult{
				StepNumber: stepNumber,
				StepType:   "step_start",
				Metadata: map[string]interface{}{
					"step": stepNumber,
				},
			}

			// Execute streaming step
			result, err := rca.executeStreamingStep(stepNumber, options, resultChan)
			if err != nil {
				resultChan <- &StreamStepResult{
					StepNumber: stepNumber,
					StepType:   "error",
					Error:      err,
					IsComplete: true,
				}
				return
			}

			// Check if we have a final answer
			if result.isFinalAnswer {
				if outputStr, ok := result.output.(string); ok {
					finalAnswer = outputStr
				} else {
					finalAnswer = fmt.Sprintf("%v", result.output)
				}
				break
			}
		}

		// Send final result
		resultChan <- &StreamStepResult{
			StepNumber: stepNumber,
			StepType:   "final",
			Output:     finalAnswer,
			IsComplete: true,
			Metadata: map[string]interface{}{
				"state":       "completed",
				"step_count":  stepNumber,
				"token_usage": nil, // TODO: Aggregate token usage from steps
			},
		}
	}()

	return resultChan, nil
}

// executeStreamingStep executes a single step with streaming support
func (rca *ReactCodeAgent) executeStreamingStep(stepNumber int, options *RunOptions, resultChan chan<- *StreamStepResult) (*stepResult, error) {
	step := memory.NewActionStep(stepNumber)
	defer func() {
		step.Timing.End()
		rca.memory.AddStep(step)
	}()

	// Build messages
	memoryMessages, err := rca.memory.WriteMemoryToMessages(false)
	if err != nil {
		return nil, fmt.Errorf("failed to build memory messages: %w", err)
	}
	step.ModelInputMessages = memoryMessages

	// Prepare generation options
	maxTokens := 2048
	temperature := 0.7
	genOptions := &models.GenerateOptions{
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
	}

	// Only add stop sequences if the model supports them
	if models.SupportsStopParameter(rca.model.GetModelID()) {
		stopSequences := []string{"Observation:"}
		if rca.codeBlockTags[1] != "" && !strings.Contains(rca.codeBlockTags[0], rca.codeBlockTags[1]) {
			stopSequences = append(stopSequences, rca.codeBlockTags[1])
		}
		genOptions.StopSequences = stopSequences
	}

	// Check if model supports streaming
	if rca.model.SupportsStreaming() && rca.streamOutputs {
		// Convert messages to interface{} slice
		var messages []interface{}
		for _, msg := range memoryMessages {
			messages = append(messages, msg)
		}

		// Stream model output
		streamChan, err := rca.model.GenerateStream(messages, genOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to generate stream: %w", err)
		}

		// Accumulate streamed content
		var fullContent strings.Builder
		for delta := range streamChan {
			if delta.Content != nil {
				fullContent.WriteString(*delta.Content)
				// Send streaming content
				resultChan <- &StreamStepResult{
					StepNumber: stepNumber,
					StepType:   "stream_delta",
					Output:     *delta.Content,
					Metadata: map[string]interface{}{
						"delta_type": "content",
					},
				}
			}
		}

		modelOutput := fullContent.String()
		step.ModelOutput = modelOutput

		// Parse and execute the output
		parseResult := rca.responseParser.Parse(modelOutput)
		return rca.processParseResult(parseResult, step, resultChan)
	} else {
		// Convert messages to interface{} slice
		var messages []interface{}
		for _, msg := range memoryMessages {
			messages = append(messages, msg)
		}

		// Non-streaming execution
		chatMessage, err := rca.model.Generate(messages, genOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to generate response: %w", err)
		}

		step.ModelOutputMessage = chatMessage
		modelOutput := ""
		if chatMessage.Content != nil {
			modelOutput = *chatMessage.Content
		}
		step.ModelOutput = modelOutput

		// Send the complete model output
		resultChan <- &StreamStepResult{
			StepNumber: stepNumber,
			StepType:   "model_output",
			Output:     modelOutput,
		}

		// Parse and execute
		parseResult := rca.responseParser.Parse(modelOutput)
		return rca.processParseResult(parseResult, step, resultChan)
	}
}

// processParseResult processes the parsed result and sends appropriate streaming events
func (rca *ReactCodeAgent) processParseResult(parseResult *parser.ParseResult, step *memory.ActionStep, resultChan chan<- *StreamStepResult) (*stepResult, error) {
	result := &stepResult{
		isFinalAnswer: false,
	}

	// Send thought if present
	if parseResult.Thought != "" {
		resultChan <- &StreamStepResult{
			StepNumber: step.StepNumber,
			StepType:   "thought",
			Output:     parseResult.Thought,
		}
	}

	switch parseResult.Type {
	case "code":
		// Send code event
		resultChan <- &StreamStepResult{
			StepNumber: step.StepNumber,
			StepType:   "code",
			Output:     parseResult.Content,
		}

		// Execute code
		execResult, err := rca.goExecutor.ExecuteRaw(parseResult.Content, rca.authorizedPackages)
		if err != nil {
			step.Error = err
			step.Observations = fmt.Sprintf("Error: %v", err)
			// Send error observation
			resultChan <- &StreamStepResult{
				StepNumber: step.StepNumber,
				StepType:   "observation",
				Output:     step.Observations,
				Metadata: map[string]interface{}{
					"has_error": true,
				},
			}
		} else {
			// Convert output to string
			outputStr := fmt.Sprintf("%v", execResult.Output)
			step.Observations = outputStr

			// Check for errors based on exit code or stderr
			hasError := execResult.ExitCode != 0 || execResult.Stderr != ""
			if hasError {
				step.Error = fmt.Errorf("execution failed: %s", execResult.Stderr)
			}

			// Send observation
			resultChan <- &StreamStepResult{
				StepNumber: step.StepNumber,
				StepType:   "observation",
				Output:     outputStr,
				Metadata: map[string]interface{}{
					"has_error": hasError,
					"stdout":    execResult.Stdout,
					"stderr":    execResult.Stderr,
				},
			}

			if execResult.IsFinalAnswer {
				result.isFinalAnswer = true
				result.output = execResult.FinalAnswer
			}
		}

	case "final_answer":
		result.isFinalAnswer = true
		result.output = parseResult.Content
		resultChan <- &StreamStepResult{
			StepNumber: step.StepNumber,
			StepType:   "final_answer",
			Output:     parseResult.Content,
		}

	case "error":
		step.Error = fmt.Errorf("parsing error: %s", parseResult.Content)
		resultChan <- &StreamStepResult{
			StepNumber: step.StepNumber,
			StepType:   "error",
			Error:      step.Error,
		}
	}

	return result, nil
}

// ToDict exports the agent configuration
func (rca *ReactCodeAgent) ToDict() map[string]interface{} {
	result := rca.BaseMultiStepAgent.ToDict()
	result["agent_type"] = "react_code"
	result["authorized_packages"] = rca.authorizedPackages
	result["code_block_tags"] = rca.codeBlockTags
	result["stream_outputs"] = rca.streamOutputs
	result["structured_output"] = rca.structuredOutput
	result["max_code_length"] = rca.maxCodeLength
	result["enable_planning"] = rca.enablePlanning
	result["planning_interval"] = rca.planningInterval
	return result
}

// Close cleans up resources
func (rca *ReactCodeAgent) Close() error {
	if rca.goExecutor != nil {
		return rca.goExecutor.Close()
	}
	return nil
}

// Helper functions for error recovery and content extraction

// truncateForLogging truncates a string for logging purposes
func truncateForLogging(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractThoughtFromRaw attempts to extract thought content from raw text
func extractThoughtFromRaw(content string) string {
	// Look for common thought patterns
	patterns := []string{
		"Thought:", "Thinking:", "I need to", "I should", "Let me", "I'll", "I will",
		"First,", "Next,", "Now,", "The task", "To solve", "To answer",
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, pattern := range patterns {
			if strings.HasPrefix(trimmed, pattern) {
				return trimmed
			}
		}
	}

	// If no pattern found, return first non-empty line as thought
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "<") {
			return trimmed
		}
	}

	return ""
}

// extractCodeFromRaw attempts to extract code from raw content
func extractCodeFromRaw(content string) string {
	// Look for code patterns
	lines := strings.Split(content, "\n")
	var codeLines []string
	inCode := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for code indicators
		if strings.Contains(line, "final_answer(") ||
			strings.Contains(line, "result =") ||
			strings.Contains(line, "fmt.") ||
			strings.Contains(line, ":=") {
			inCode = true
		}

		if inCode {
			// Stop at obvious non-code lines
			if strings.HasPrefix(trimmed, "Thought:") ||
				strings.HasPrefix(trimmed, "Observation:") ||
				strings.HasPrefix(trimmed, "Error:") {
				break
			}
			codeLines = append(codeLines, line)
		}
	}

	return strings.Join(codeLines, "\n")
}

// extractCodeBetweenTags extracts code between specific tags
func extractCodeBetweenTags(content, openTag, closeTag string) string {
	startIdx := strings.Index(content, openTag)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(openTag)

	endIdx := strings.Index(content[startIdx:], closeTag)
	if endIdx == -1 {
		// No closing tag, take rest of content
		return strings.TrimSpace(content[startIdx:])
	}

	return strings.TrimSpace(content[startIdx : startIdx+endIdx])
}

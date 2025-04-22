// Package agents provides agent implementations for various types of AI agents
package agents

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/rizome-dev/smolagentsgo/agent_types"
	"github.com/rizome-dev/smolagentsgo/memory"
	"github.com/rizome-dev/smolagentsgo/models"
	"github.com/rizome-dev/smolagentsgo/tools"
	"github.com/rizome-dev/smolagentsgo/utils"
)

// PromptTemplate is a type for holding templates for generating prompts
type PromptTemplate string

// PromptTemplates holds the different prompt templates used by an agent
type PromptTemplates struct {
	SystemPrompt string
	Planning     struct {
		InitialPlan            string
		UpdatePlanPreMessages  string
		UpdatePlanPostMessages string
	}
	ManagedAgent struct {
		Task   string
		Report string
	}
	FinalAnswer struct {
		PreMessages  string
		PostMessages string
	}
}

// PopulateTemplate fills in the template with the given variables
func PopulateTemplate(template string, variables map[string]interface{}) (string, error) {
	result := template
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		str := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, str)
	}
	return result, nil
}

// EmptyPromptTemplates returns empty prompt templates
func EmptyPromptTemplates() PromptTemplates {
	return PromptTemplates{
		SystemPrompt: "",
		Planning: struct {
			InitialPlan            string
			UpdatePlanPreMessages  string
			UpdatePlanPostMessages string
		}{
			InitialPlan:            "",
			UpdatePlanPreMessages:  "",
			UpdatePlanPostMessages: "",
		},
		ManagedAgent: struct {
			Task   string
			Report string
		}{
			Task:   "",
			Report: "",
		},
		FinalAnswer: struct {
			PreMessages  string
			PostMessages string
		}{
			PreMessages:  "",
			PostMessages: "",
		},
	}
}

// ModelFunc is a function type for calling a language model
type ModelFunc func(messages []models.Message, stopSequences []string) (*models.ChatMessage, error)

// RunCallback is a function type for callbacks executed at each step
type RunCallback func(step memory.ActionStep)

// FinalAnswerCheck is a function type for checking the validity of a final answer
type FinalAnswerCheck func(finalAnswer interface{}, mem *memory.AgentMemory) bool

// MultiStepAgent is the base interface for all multi-step agents
type MultiStepAgent interface {
	// Run executes the agent with the given task
	Run(task string, stream bool, reset bool, images []image.Image, additionalArgs map[string]interface{}, maxSteps int) (interface{}, error)

	// InitializeSystemPrompt initializes the system prompt
	InitializeSystemPrompt() string

	// Step executes a single step
	Step(memoryStep *memory.ActionStep) (interface{}, error)
}

// BaseMultiStepAgent provides a base implementation for multi-step agents
type BaseMultiStepAgent struct {
	Tools             map[string]tools.Tool
	Model             ModelFunc
	PromptTemplates   PromptTemplates
	MaxSteps          int
	Grammar           map[string]string
	ManagedAgents     map[string]ManagedAgent
	StepCallbacks     []RunCallback
	PlanningInterval  int
	Name              string
	Description       string
	ProvideRunSummary bool
	FinalAnswerChecks []FinalAnswerCheck

	// Internal state
	AgentName       string
	State           map[string]interface{}
	SystemPrompt    string
	Task            string
	Memory          *memory.AgentMemory
	InterruptSwitch bool
	StepNumber      int
}

// NewBaseMultiStepAgent creates a new BaseMultiStepAgent
func NewBaseMultiStepAgent(
	toolsList []tools.Tool,
	model ModelFunc,
	promptTemplates PromptTemplates,
	maxSteps int,
	addBaseTools bool,
	grammar map[string]string,
	managedAgents []ManagedAgent,
	stepCallbacks []RunCallback,
	planningInterval int,
	name string,
	description string,
	provideRunSummary bool,
	finalAnswerChecks []FinalAnswerCheck,
) (*BaseMultiStepAgent, error) {
	// Validate agent name if provided
	if name != "" && !utils.IsValidName(name) {
		return nil, fmt.Errorf("agent name '%s' must be a valid identifier and not a reserved keyword", name)
	}

	// Set up tools
	toolMap := make(map[string]tools.Tool)
	for _, tool := range toolsList {
		toolMap[tool.Name()] = tool
	}

	// Add FinalAnswerTool if needed
	if _, exists := toolMap["final_answer"]; !exists {
		toolMap["final_answer"] = tools.NewFinalAnswerTool()
	}

	// Set up managed agents
	managedAgentMap := make(map[string]ManagedAgent)
	if managedAgents != nil {
		for _, agent := range managedAgents {
			if agent.GetName() == "" || agent.GetDescription() == "" {
				return nil, fmt.Errorf("all managed agents need both a name and a description")
			}
			managedAgentMap[agent.GetName()] = agent
		}
	}

	// Validate tool and managed agent names
	allNames := make([]string, 0, len(toolMap)+len(managedAgentMap)+1)
	for toolName := range toolMap {
		allNames = append(allNames, toolName)
	}
	for agentName := range managedAgentMap {
		allNames = append(allNames, agentName)
	}
	if name != "" {
		allNames = append(allNames, name)
	}

	// Check for duplicates
	nameSet := make(map[string]bool)
	for _, name := range allNames {
		if nameSet[name] {
			return nil, fmt.Errorf("each tool or managed_agent should have a unique name! Duplicate name: %s", name)
		}
		nameSet[name] = true
	}

	// Initialize the agent
	agent := &BaseMultiStepAgent{
		Tools:             toolMap,
		Model:             model,
		PromptTemplates:   promptTemplates,
		MaxSteps:          maxSteps,
		Grammar:           grammar,
		ManagedAgents:     managedAgentMap,
		StepCallbacks:     stepCallbacks,
		PlanningInterval:  planningInterval,
		Name:              name,
		Description:       description,
		ProvideRunSummary: provideRunSummary,
		FinalAnswerChecks: finalAnswerChecks,

		AgentName:       "BaseMultiStepAgent",
		State:           make(map[string]interface{}),
		SystemPrompt:    "",
		Task:            "",
		InterruptSwitch: false,
		StepNumber:      0,
	}

	// Initialize the system prompt
	agent.SystemPrompt = agent.InitializeSystemPrompt()
	agent.Memory = memory.NewAgentMemory(agent.SystemPrompt)

	return agent, nil
}

// Run executes the agent for the given task
func (a *BaseMultiStepAgent) Run(
	task string,
	stream bool,
	reset bool,
	images []image.Image,
	additionalArgs map[string]interface{},
	maxSteps int,
) (interface{}, error) {
	// If maxSteps not provided, use the agent's default
	if maxSteps <= 0 {
		maxSteps = a.MaxSteps
	}

	a.Task = task
	a.InterruptSwitch = false

	// Update state with additional args if provided
	if additionalArgs != nil {
		for k, v := range additionalArgs {
			a.State[k] = v
		}

		// Append additional args to task for clarity
		a.Task += fmt.Sprintf("\nYou have been provided with these additional arguments, that you can access using the keys as variables in your python code:\n%v", additionalArgs)
	}

	// Initialize system prompt
	a.SystemPrompt = a.InitializeSystemPrompt()
	a.Memory.SystemPrompt = &memory.SystemPromptStep{SystemPrompt: a.SystemPrompt}

	if reset {
		a.Memory.Reset()
	}

	// Add task to memory
	a.Memory.Steps = append(a.Memory.Steps, &memory.TaskStep{
		Task:       a.Task,
		TaskImages: images,
	})

	// Run the agent
	if stream {
		// For streaming, return a channel
		return a.runStreaming(a.Task, maxSteps, images)
	}

	// For non-streaming, execute all steps and return final answer
	return a.runBlocking(a.Task, maxSteps, images)
}

// runStreaming runs the agent in streaming mode, returning each step as it's executed
func (a *BaseMultiStepAgent) runStreaming(task string, maxSteps int, images []image.Image) (chan memory.MemoryStep, error) {
	resultChan := make(chan memory.MemoryStep)

	go func() {
		defer close(resultChan)

		finalAnswer := interface{}(nil)
		a.StepNumber = 1

		for finalAnswer == nil && a.StepNumber <= maxSteps {
			if a.InterruptSwitch {
				resultChan <- &memory.ActionStep{
					Error: &utils.AgentError{Message: "Agent interrupted"},
				}
				return
			}

			stepStartTime := time.Now()

			// Create planning step if needed
			if a.PlanningInterval > 0 && (a.StepNumber == 1 || (a.StepNumber-1)%a.PlanningInterval == 0) {
				planningStep := a.createPlanningStep(task, a.StepNumber == 1, a.StepNumber)
				a.Memory.Steps = append(a.Memory.Steps, planningStep)
				resultChan <- planningStep
			}

			// Create action step
			actionStep := &memory.ActionStep{
				StepNumber:         a.StepNumber,
				StartTime:          stepStartTime,
				ObservationsImages: images,
			}

			// Execute the step
			var err error
			finalAnswer, err = a.Step(actionStep)
			if err != nil {
				if genErr, ok := err.(*utils.AgentGenerationError); ok {
					// Agent generation errors are implementation errors, so raise them
					resultChan <- &memory.ActionStep{
						Error: &utils.AgentError{Message: genErr.Error()},
					}
					return
				}

				// Other agent errors are just logged and we continue
				actionStep.Error = err
			}

			// Finalize the step
			a.finalizeStep(actionStep, stepStartTime)
			a.Memory.Steps = append(a.Memory.Steps, actionStep)
			resultChan <- actionStep
			a.StepNumber++
		}

		// If max steps reached without final answer
		if finalAnswer == nil && a.StepNumber == maxSteps+1 {
			stepStartTime := time.Now()
			finalAnswer = a.provideFinalAnswer(task, images)
			finalMemoryStep := &memory.ActionStep{
				StepNumber: a.StepNumber,
				Error:      utils.NewAgentMaxStepsError("Reached max steps.", nil),
			}
			finalMemoryStep.ActionOutput = finalAnswer

			finalMemoryStep.EndTime = time.Now()
			finalMemoryStep.Duration = time.Since(stepStartTime).Seconds()

			a.Memory.Steps = append(a.Memory.Steps, finalMemoryStep)
			resultChan <- finalMemoryStep
		}

		// Send final answer
		resultChan <- &memory.FinalAnswerStep{
			FinalAnswer: agent_types.HandleAgentOutputTypes(finalAnswer, ""),
		}
	}()

	return resultChan, nil
}

// runBlocking runs the agent in blocking mode, executing all steps and returning the final answer
func (a *BaseMultiStepAgent) runBlocking(task string, maxSteps int, images []image.Image) (interface{}, error) {
	stepChan, err := a.runStreaming(task, maxSteps, images)
	if err != nil {
		return nil, err
	}

	var finalAnswer interface{}
	for step := range stepChan {
		if finalAnswerStep, ok := step.(*memory.FinalAnswerStep); ok {
			finalAnswer = finalAnswerStep.FinalAnswer
		}
	}

	return finalAnswer, nil
}

// createPlanningStep creates a planning step
func (a *BaseMultiStepAgent) createPlanningStep(task string, isFirstStep bool, step int) *memory.PlanningStep {
	var inputMessages []models.Message
	var planMessage *models.ChatMessage
	var plan string

	if isFirstStep {
		// Create initial plan
		variables := map[string]interface{}{
			"task":           task,
			"tools":          a.Tools,
			"managed_agents": a.ManagedAgents,
		}

		initialPlanPrompt, err := PopulateTemplate(a.PromptTemplates.Planning.InitialPlan, variables)
		if err != nil {
			initialPlanPrompt = fmt.Sprintf("Task: %s\n\nAvailable tools: %v\n\n", task, a.Tools)
		}

		inputMessages = []models.Message{
			{
				Role: models.MessageRoleUser,
				Content: []models.MessageContent{
					{
						Type: "text",
						Text: initialPlanPrompt,
					},
				},
			},
		}

		planMessage, err = a.Model(inputMessages, []string{"<end_plan>"})
		if err != nil {
			// Handle error gracefully
			planMessage = &models.ChatMessage{
				Role:    string(models.MessageRoleAssistant),
				Content: "Error creating initial plan: " + err.Error(),
			}
		}

		plan = fmt.Sprintf("Here are the facts I know and the plan of action that I will follow to solve the task:\n```\n%s\n```", planMessage.Content)
	} else {
		// Create plan update
		variables := map[string]interface{}{
			"task": task,
		}

		// Get memory messages in summary mode
		memoryMessages := a.writeMemoryToMessages(true)

		planUpdatePrePrompt, err := PopulateTemplate(a.PromptTemplates.Planning.UpdatePlanPreMessages, variables)
		if err != nil {
			planUpdatePrePrompt = fmt.Sprintf("Let's update our plan based on what we've learned so far about the task: %s", task)
		}

		variables["tools"] = a.Tools
		variables["managed_agents"] = a.ManagedAgents
		variables["remaining_steps"] = (a.MaxSteps - step)

		planUpdatePostPrompt, err := PopulateTemplate(a.PromptTemplates.Planning.UpdatePlanPostMessages, variables)
		if err != nil {
			planUpdatePostPrompt = fmt.Sprintf("Please provide an updated plan considering the remaining steps (%d) and the available tools.", a.MaxSteps-step)
		}

		inputMessages = append([]models.Message{
			{
				Role: models.MessageRoleSystem,
				Content: []models.MessageContent{
					{
						Type: "text",
						Text: planUpdatePrePrompt,
					},
				},
			},
		}, memoryMessages...)

		inputMessages = append(inputMessages, models.Message{
			Role: models.MessageRoleUser,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: planUpdatePostPrompt,
				},
			},
		})

		planMessage, err = a.Model(inputMessages, []string{"<end_plan>"})
		if err != nil {
			// Handle error gracefully
			planMessage = &models.ChatMessage{
				Role:    string(models.MessageRoleAssistant),
				Content: "Error updating plan: " + err.Error(),
			}
		}

		plan = fmt.Sprintf("I still need to solve the task I was given:\n```\n%s\n```\n\nHere are the facts I know and my new/updated plan of action to solve the task:\n```\n%s\n```", a.Task, planMessage.Content)
	}

	return &memory.PlanningStep{
		ModelInputMessages: inputMessages,
		Plan:               plan,
		ModelOutputMessage: planMessage,
	}
}

// finalizeStep completes a step with end time and duration, and calls callbacks
func (a *BaseMultiStepAgent) finalizeStep(memoryStep *memory.ActionStep, stepStartTime time.Time) {
	memoryStep.EndTime = time.Now()
	memoryStep.Duration = time.Since(stepStartTime).Seconds()

	// Execute callbacks
	for _, callback := range a.StepCallbacks {
		callback(*memoryStep)
	}
}

// writeMemoryToMessages converts memory to a slice of messages
func (a *BaseMultiStepAgent) writeMemoryToMessages(summaryMode bool) []models.Message {
	var messages []models.Message

	// Add system prompt if not in summary mode
	if !summaryMode {
		systemPromptMessages := a.Memory.SystemPrompt.ToMessages(map[string]interface{}{
			"summary_mode": summaryMode,
		})
		messages = append(messages, systemPromptMessages...)
	}

	// Add memory steps
	for _, step := range a.Memory.Steps {
		stepMessages := step.ToMessages(map[string]interface{}{
			"summary_mode": summaryMode,
		})
		messages = append(messages, stepMessages...)
	}

	return messages
}

// provideFinalAnswer attempts to generate a final answer when max steps are reached
func (a *BaseMultiStepAgent) provideFinalAnswer(task string, images []image.Image) interface{} {
	variables := map[string]interface{}{
		"task": task,
	}

	// Get memory messages
	memoryMessages := a.writeMemoryToMessages(false)

	preMessagesPrompt, err := PopulateTemplate(a.PromptTemplates.FinalAnswer.PreMessages, variables)
	if err != nil {
		preMessagesPrompt = "You've reached the maximum number of steps. Please provide your final answer based on what you've learned so far."
	}

	postMessagesPrompt, err := PopulateTemplate(a.PromptTemplates.FinalAnswer.PostMessages, variables)
	if err != nil {
		postMessagesPrompt = "Time to provide your final answer. Use the final_answer tool with your conclusion."
	}

	inputMessages := append([]models.Message{
		{
			Role: models.MessageRoleSystem,
			Content: []models.MessageContent{
				{
					Type: "text",
					Text: preMessagesPrompt,
				},
			},
		},
	}, memoryMessages...)

	inputMessages = append(inputMessages, models.Message{
		Role: models.MessageRoleUser,
		Content: []models.MessageContent{
			{
				Type: "text",
				Text: postMessagesPrompt,
			},
		},
	})

	// Add images if provided
	if len(images) > 0 {
		imgContents := make([]models.MessageContent, len(images))
		for i, img := range images {
			imgContents[i] = models.MessageContent{
				Type:  "image",
				Image: img,
			}
		}

		inputMessages = append(inputMessages, models.Message{
			Role:    models.MessageRoleUser,
			Content: imgContents,
		})
	}

	// Call model for final answer
	finalMessage, err := a.Model(inputMessages, nil)
	if err != nil {
		return "Failed to generate final answer: " + err.Error()
	}

	return finalMessage.Content
}

// InitializeSystemPrompt initializes the system prompt
func (a *BaseMultiStepAgent) InitializeSystemPrompt() string {
	return a.PromptTemplates.SystemPrompt
}

// Step executes a single step (to be implemented by child classes)
func (a *BaseMultiStepAgent) Step(memoryStep *memory.ActionStep) (interface{}, error) {
	return nil, fmt.Errorf("Step method must be implemented by child classes")
}

// Interrupt stops the agent execution
func (a *BaseMultiStepAgent) Interrupt() {
	a.InterruptSwitch = true
}

// extractAction extracts action and reasoning from model output
func (a *BaseMultiStepAgent) extractAction(modelOutput string, splitToken string) (string, string, error) {
	parts := strings.Split(modelOutput, splitToken)

	if len(parts) < 2 {
		return "", "", fmt.Errorf("could not find split token '%s' in model output", splitToken)
	}

	// First part is the thought/reasoning
	thought := strings.TrimSpace(parts[0])

	// Second part is the action
	action := strings.TrimSpace(parts[1])

	return thought, action, nil
}

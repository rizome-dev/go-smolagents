// Package agents provides the core agent implementations for smolagents.
//
// This includes MultiStepAgent, ReactCodeAgent, and supporting infrastructure
// for the ReAct framework with code execution. The ReactCodeAgent implements
// proper Thought/Code/Observation cycles with YAML-based prompts and sandboxed execution.
package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/rizome-dev/go-smolagents/pkg/memory"
	"github.com/rizome-dev/go-smolagents/pkg/models"
	"github.com/rizome-dev/go-smolagents/pkg/monitoring"
	"github.com/rizome-dev/go-smolagents/pkg/tools"
	"github.com/rizome-dev/go-smolagents/pkg/utils"
)

// RunOptions represents options for agent execution
type RunOptions struct {
	Task           string                 `json:"task"`
	Stream         bool                   `json:"stream,omitempty"`
	Reset          bool                   `json:"reset,omitempty"`
	Images         []interface{}          `json:"images,omitempty"`
	AdditionalArgs map[string]interface{} `json:"additional_args,omitempty"`
	MaxSteps       *int                   `json:"max_steps,omitempty"`
	Context        context.Context        `json:"-"`
	StepCallbacks  []StepCallback         `json:"-"`
}

// NewRunOptions creates run options with defaults
func NewRunOptions(task string) *RunOptions {
	return &RunOptions{
		Task:           task,
		Stream:         false,
		Reset:          true,
		AdditionalArgs: make(map[string]interface{}),
		Context:        context.Background(),
	}
}

// StepCallback is called after each agent step
type StepCallback func(step memory.MemoryStep) error

// RunResult represents the result of an agent run
type RunResult struct {
	Output     interface{}              `json:"output"`
	State      string                   `json:"state"` // "success", "max_steps_error"
	Messages   []map[string]interface{} `json:"messages"`
	TokenUsage *monitoring.TokenUsage   `json:"token_usage,omitempty"`
	Timing     *monitoring.Timing       `json:"timing"`
	StepCount  int                      `json:"step_count"`
	Metadata   map[string]interface{}   `json:"metadata,omitempty"`
	Error      error                    `json:"error,omitempty"`
}

// NewRunResult creates a new run result
func NewRunResult() *RunResult {
	return &RunResult{
		Timing:   monitoring.NewTiming(),
		Metadata: make(map[string]interface{}),
	}
}

// FinalOutput represents the final output from an agent
type FinalOutput struct {
	Output interface{} `json:"output"`
}

// PromptTemplates represents the prompt templates for an agent
type PromptTemplates struct {
	SystemPrompt string                     `json:"system_prompt"`
	Planning     PlanningPromptTemplate     `json:"planning"`
	ManagedAgent ManagedAgentPromptTemplate `json:"managed_agent"`
	FinalAnswer  FinalAnswerPromptTemplate  `json:"final_answer"`
}

// PlanningPromptTemplate represents planning prompt templates
type PlanningPromptTemplate struct {
	InitialPlan       string `json:"initial_plan"`
	UpdatePlanPreMsg  string `json:"update_plan_pre_messages"`
	UpdatePlanPostMsg string `json:"update_plan_post_messages"`
}

// ManagedAgentPromptTemplate represents managed agent prompt templates
type ManagedAgentPromptTemplate struct {
	Task   string `json:"task"`
	Report string `json:"report"`
}

// FinalAnswerPromptTemplate represents final answer prompt templates
type FinalAnswerPromptTemplate struct {
	PreMessages  string `json:"pre_messages"`
	PostMessages string `json:"post_messages"`
}

// EmptyPromptTemplates returns empty prompt templates
func EmptyPromptTemplates() *PromptTemplates {
	return &PromptTemplates{
		SystemPrompt: "",
		Planning: PlanningPromptTemplate{
			InitialPlan:       "",
			UpdatePlanPreMsg:  "",
			UpdatePlanPostMsg: "",
		},
		ManagedAgent: ManagedAgentPromptTemplate{
			Task:   "",
			Report: "",
		},
		FinalAnswer: FinalAnswerPromptTemplate{
			PreMessages:  "",
			PostMessages: "",
		},
	}
}

// StreamStepResult represents a single step result in streaming mode
type StreamStepResult struct {
	StepNumber int                    `json:"step_number"`
	StepType   string                 `json:"step_type"`
	Output     interface{}            `json:"output,omitempty"`
	Error      error                  `json:"error,omitempty"`
	IsComplete bool                   `json:"is_complete"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// MultiStepAgent represents the main interface for multi-step agents
type MultiStepAgent interface {
	// Core execution methods
	Run(options *RunOptions) (*RunResult, error)
	RunStream(options *RunOptions) (<-chan *StreamStepResult, error)

	// Agent management
	Reset() error
	Interrupt() error

	// Configuration
	GetModel() models.Model
	SetModel(model models.Model)
	GetTools() []tools.Tool
	SetTools(tools []tools.Tool)
	GetMemory() *memory.AgentMemory
	SetMemory(mem *memory.AgentMemory)
	GetSystemPrompt() string
	SetSystemPrompt(prompt string)
	GetMaxSteps() int
	SetMaxSteps(maxSteps int)

	// Serialization
	ToDict() map[string]interface{}
	Save(outputDir string, toolFileName string, makeGradioApp bool) error
	PushToHub(repoID string, commitMessage string, token string, private bool) error

	// Execution state
	IsRunning() bool
	GetStepCount() int
}

// BaseMultiStepAgent provides common functionality for all agent implementations
type BaseMultiStepAgent struct {
	model            models.Model
	tools            []tools.Tool
	toolsMap         map[string]tools.Tool
	memory           *memory.AgentMemory
	systemPrompt     string
	maxSteps         int
	planning         bool
	planningInterval int
	promptTemplates  *PromptTemplates
	monitor          *monitoring.Monitor
	logger           *monitoring.AgentLogger

	// State management
	isRunning   bool
	stepCount   int
	interrupted bool

	// Managed agents
	managedAgents map[string]MultiStepAgent

	// Additional configuration
	additionalArgs map[string]interface{}
}

// NewBaseMultiStepAgent creates a new base multi-step agent
func NewBaseMultiStepAgent(
	model models.Model,
	toolsArg []tools.Tool,
	systemPrompt string,
	options map[string]interface{},
) (*BaseMultiStepAgent, error) {

	if model == nil {
		return nil, utils.NewAgentError("model cannot be nil")
	}

	// Set up tools
	agentTools := make([]tools.Tool, 0)
	toolsMap := make(map[string]tools.Tool)

	if toolsArg != nil {
		for _, tool := range toolsArg {
			if tool == nil {
				continue
			}

			// Validate tool
			if err := tool.Validate(); err != nil {
				return nil, utils.NewAgentError(fmt.Sprintf("invalid tool '%s': %v", tool.GetName(), err))
			}

			// Check for name conflicts
			if _, exists := toolsMap[tool.GetName()]; exists {
				return nil, utils.NewAgentError(fmt.Sprintf("duplicate tool name: %s", tool.GetName()))
			}

			agentTools = append(agentTools, tool)
			toolsMap[tool.GetName()] = tool
		}
	}

	// Create agent memory
	agentMemory := memory.NewAgentMemory(systemPrompt)

	// Set up logging and monitoring
	logger := monitoring.NewAgentLogger(monitoring.LogLevelINFO)
	monitor := monitoring.NewMonitor(logger)

	agent := &BaseMultiStepAgent{
		model:            model,
		tools:            agentTools,
		toolsMap:         toolsMap,
		memory:           agentMemory,
		systemPrompt:     systemPrompt,
		maxSteps:         20, // Default
		planning:         false,
		planningInterval: 3,
		promptTemplates:  EmptyPromptTemplates(),
		monitor:          monitor,
		logger:           logger,
		managedAgents:    make(map[string]MultiStepAgent),
		additionalArgs:   make(map[string]interface{}),
	}

	// Apply options
	if options != nil {
		if maxSteps, ok := options["max_steps"].(int); ok {
			agent.maxSteps = maxSteps
		}
		if planning, ok := options["planning"].(bool); ok {
			agent.planning = planning
		}
		if planningInterval, ok := options["planning_interval"].(int); ok {
			agent.planningInterval = planningInterval
		}
		if templates, ok := options["prompt_templates"].(*PromptTemplates); ok {
			agent.promptTemplates = templates
		}

		// Store additional options
		for k, v := range options {
			switch k {
			case "max_steps", "planning", "planning_interval", "prompt_templates":
				// Already processed
			default:
				agent.additionalArgs[k] = v
			}
		}
	}

	return agent, nil
}

// GetModel implements MultiStepAgent
func (ba *BaseMultiStepAgent) GetModel() models.Model {
	return ba.model
}

// SetModel implements MultiStepAgent
func (ba *BaseMultiStepAgent) SetModel(model models.Model) {
	ba.model = model
}

// GetTools implements MultiStepAgent
func (ba *BaseMultiStepAgent) GetTools() []tools.Tool {
	return ba.tools
}

// SetTools implements MultiStepAgent
func (ba *BaseMultiStepAgent) SetTools(toolsArg []tools.Tool) {
	ba.tools = make([]tools.Tool, 0)
	ba.toolsMap = make(map[string]tools.Tool)

	if toolsArg != nil {
		for _, tool := range toolsArg {
			if tool != nil {
				ba.tools = append(ba.tools, tool)
				ba.toolsMap[tool.GetName()] = tool
			}
		}
	}
}

// GetMemory implements MultiStepAgent
func (ba *BaseMultiStepAgent) GetMemory() *memory.AgentMemory {
	return ba.memory
}

// SetMemory implements MultiStepAgent
func (ba *BaseMultiStepAgent) SetMemory(mem *memory.AgentMemory) {
	ba.memory = mem
}

// GetSystemPrompt implements MultiStepAgent
func (ba *BaseMultiStepAgent) GetSystemPrompt() string {
	return ba.systemPrompt
}

// SetSystemPrompt implements MultiStepAgent
func (ba *BaseMultiStepAgent) SetSystemPrompt(prompt string) {
	ba.systemPrompt = prompt
	if ba.memory != nil {
		ba.memory.SetSystemPrompt(prompt)
	}
}

// GetMaxSteps implements MultiStepAgent
func (ba *BaseMultiStepAgent) GetMaxSteps() int {
	return ba.maxSteps
}

// SetMaxSteps implements MultiStepAgent
func (ba *BaseMultiStepAgent) SetMaxSteps(maxSteps int) {
	ba.maxSteps = maxSteps
}

// Reset implements MultiStepAgent
func (ba *BaseMultiStepAgent) Reset() error {
	ba.memory.Reset()
	ba.stepCount = 0
	ba.isRunning = false
	ba.interrupted = false

	if ba.monitor != nil {
		ba.monitor.Reset()
	}

	return nil
}

// Interrupt implements MultiStepAgent
func (ba *BaseMultiStepAgent) Interrupt() error {
	ba.interrupted = true
	return nil
}

// IsRunning implements MultiStepAgent
func (ba *BaseMultiStepAgent) IsRunning() bool {
	return ba.isRunning
}

// GetStepCount implements MultiStepAgent
func (ba *BaseMultiStepAgent) GetStepCount() int {
	return ba.stepCount
}

// ToDict implements MultiStepAgent
func (ba *BaseMultiStepAgent) ToDict() map[string]interface{} {
	toolsData := make([]map[string]interface{}, len(ba.tools))
	for i, tool := range ba.tools {
		toolsData[i] = tool.ToDict()
	}

	result := map[string]interface{}{
		"model_id":          ba.model.GetModelID(),
		"tools":             toolsData,
		"system_prompt":     ba.systemPrompt,
		"max_steps":         ba.maxSteps,
		"planning":          ba.planning,
		"planning_interval": ba.planningInterval,
		"prompt_templates":  ba.promptTemplates,
		"step_count":        ba.stepCount,
	}

	// Add additional args
	for k, v := range ba.additionalArgs {
		result[k] = v
	}

	return result
}

// Save implements MultiStepAgent (placeholder implementation)
func (ba *BaseMultiStepAgent) Save(outputDir string, toolFileName string, makeGradioApp bool) error {
	// This would save the agent configuration to disk
	// For now, return a placeholder implementation
	ba.logger.Info("Save method not fully implemented yet")
	return nil
}

// PushToHub implements MultiStepAgent (placeholder implementation)
func (ba *BaseMultiStepAgent) PushToHub(repoID string, commitMessage string, token string, private bool) error {
	// This would push the agent to Hugging Face Hub
	// For now, return a placeholder implementation
	ba.logger.Info("PushToHub method not fully implemented yet")
	return nil
}

// GetTool retrieves a tool by name
func (ba *BaseMultiStepAgent) GetTool(name string) (tools.Tool, bool) {
	tool, exists := ba.toolsMap[name]
	return tool, exists
}

// AddTool adds a tool to the agent
func (ba *BaseMultiStepAgent) AddTool(tool tools.Tool) error {
	if tool == nil {
		return utils.NewAgentError("tool cannot be nil")
	}

	if err := tool.Validate(); err != nil {
		return utils.NewAgentError(fmt.Sprintf("invalid tool: %v", err))
	}

	name := tool.GetName()
	if _, exists := ba.toolsMap[name]; exists {
		return utils.NewAgentError(fmt.Sprintf("tool with name '%s' already exists", name))
	}

	ba.tools = append(ba.tools, tool)
	ba.toolsMap[name] = tool

	return nil
}

// RemoveTool removes a tool from the agent
func (ba *BaseMultiStepAgent) RemoveTool(name string) bool {
	if _, exists := ba.toolsMap[name]; !exists {
		return false
	}

	delete(ba.toolsMap, name)

	// Remove from slice
	for i, tool := range ba.tools {
		if tool.GetName() == name {
			ba.tools = append(ba.tools[:i], ba.tools[i+1:]...)
			break
		}
	}

	return true
}

// AddManagedAgent adds a managed agent
func (ba *BaseMultiStepAgent) AddManagedAgent(name string, agent MultiStepAgent) error {
	if agent == nil {
		return utils.NewAgentError("managed agent cannot be nil")
	}

	if _, exists := ba.managedAgents[name]; exists {
		return utils.NewAgentError(fmt.Sprintf("managed agent with name '%s' already exists", name))
	}

	ba.managedAgents[name] = agent
	return nil
}

// GetManagedAgent retrieves a managed agent by name
func (ba *BaseMultiStepAgent) GetManagedAgent(name string) (MultiStepAgent, bool) {
	agent, exists := ba.managedAgents[name]
	return agent, exists
}

// SetLogger sets the logger for the agent
func (ba *BaseMultiStepAgent) SetLogger(logger *monitoring.AgentLogger) {
	ba.logger = logger
	if ba.monitor != nil {
		ba.monitor = monitoring.NewMonitor(logger)
	}
}

// SetMonitor sets the monitor for the agent
func (ba *BaseMultiStepAgent) SetMonitor(monitor *monitoring.Monitor) {
	ba.monitor = monitor
}

// GetLogger returns the agent's logger
func (ba *BaseMultiStepAgent) GetLogger() *monitoring.AgentLogger {
	return ba.logger
}

// GetMonitor returns the agent's monitor
func (ba *BaseMultiStepAgent) GetMonitor() *monitoring.Monitor {
	return ba.monitor
}

// SetPlanning enables or disables planning
func (ba *BaseMultiStepAgent) SetPlanning(enabled bool) {
	ba.planning = enabled
}

// GetPlanning returns whether planning is enabled
func (ba *BaseMultiStepAgent) GetPlanning() bool {
	return ba.planning
}

// SetPlanningInterval sets the planning interval
func (ba *BaseMultiStepAgent) SetPlanningInterval(interval int) {
	ba.planningInterval = interval
}

// GetPlanningInterval returns the planning interval
func (ba *BaseMultiStepAgent) GetPlanningInterval() int {
	return ba.planningInterval
}

// SetPromptTemplates sets the prompt templates
func (ba *BaseMultiStepAgent) SetPromptTemplates(templates *PromptTemplates) {
	ba.promptTemplates = templates
}

// GetPromptTemplates returns the prompt templates
func (ba *BaseMultiStepAgent) GetPromptTemplates() *PromptTemplates {
	return ba.promptTemplates
}

// PopulateTemplate fills in template variables using the provided data
func PopulateTemplate(template string, variables map[string]interface{}) string {
	result := template

	// Simple template variable replacement using {{variable}} syntax
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		replacement := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, replacement)
	}

	return result
}

// GetVariableNames extracts variable names from a template string
func GetVariableNames(template string) []string {
	pattern := regexp.MustCompile(`\{\{([^{}]+)\}\}`)
	matches := pattern.FindAllStringSubmatch(template, -1)

	var variables []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			variable := strings.TrimSpace(match[1])
			if !seen[variable] {
				variables = append(variables, variable)
				seen[variable] = true
			}
		}
	}

	return variables
}

// Utility functions for agent creation

// AgentConfig represents configuration for creating agents
type AgentConfig struct {
	Model            models.Model            `json:"-"`
	Tools            []tools.Tool            `json:"-"`
	SystemPrompt     string                  `json:"system_prompt"`
	MaxSteps         int                     `json:"max_steps"`
	Planning         bool                    `json:"planning"`
	PlanningInterval int                     `json:"planning_interval"`
	PromptTemplates  *PromptTemplates        `json:"prompt_templates"`
	Logger           *monitoring.AgentLogger `json:"-"`
	Monitor          *monitoring.Monitor     `json:"-"`
	Additional       map[string]interface{}  `json:"additional"`
}

// DefaultAgentConfig returns a default agent configuration
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		MaxSteps:         20,
		Planning:         false,
		PlanningInterval: 3,
		PromptTemplates:  EmptyPromptTemplates(),
		Additional:       make(map[string]interface{}),
	}
}

// CreateAgent creates an agent based on the provided configuration
func CreateAgent(config *AgentConfig, agentType string) (MultiStepAgent, error) {
	if config == nil {
		return nil, utils.NewAgentError("agent config cannot be nil")
	}

	if config.Model == nil {
		return nil, utils.NewAgentError("model cannot be nil")
	}

	options := make(map[string]interface{})
	options["max_steps"] = config.MaxSteps
	options["planning"] = config.Planning
	options["planning_interval"] = config.PlanningInterval
	options["prompt_templates"] = config.PromptTemplates

	// Add additional options
	for k, v := range config.Additional {
		options[k] = v
	}

	switch agentType {
	case "react_code", "code":
		// Convert generic options to ReactCodeAgentOptions
		reactOptions := &ReactCodeAgentOptions{
			MaxSteps:         config.MaxSteps,
			EnablePlanning:   config.Planning,
			PlanningInterval: config.PlanningInterval,
			StreamOutputs:    true,
			Verbose:          false,
		}

		// Apply additional options
		if val, ok := config.Additional["authorized_packages"].([]string); ok {
			reactOptions.AuthorizedPackages = val
		}
		if val, ok := config.Additional["stream_outputs"].(bool); ok {
			reactOptions.StreamOutputs = val
		}
		if val, ok := config.Additional["verbose"].(bool); ok {
			reactOptions.Verbose = val
		}

		return NewReactCodeAgent(config.Model, config.Tools, config.SystemPrompt, reactOptions)
	default:
		return nil, utils.NewAgentError(fmt.Sprintf("unknown agent type: %s", agentType))
	}
}

// JSON marshaling support
func (rr *RunResult) MarshalJSON() ([]byte, error) {
	type Alias RunResult
	return json.Marshal(&struct {
		*Alias
		TimingDuration string `json:"timing_duration,omitempty"`
	}{
		Alias: (*Alias)(rr),
		TimingDuration: func() string {
			if rr.Timing != nil && rr.Timing.Duration != nil {
				return rr.Timing.Duration.String()
			}
			return ""
		}(),
	})
}

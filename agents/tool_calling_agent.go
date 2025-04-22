// Package agents provides agent implementations for various types of AI agents
package agents

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/rizome-dev/smolagentsgo/memory"
	"github.com/rizome-dev/smolagentsgo/tools"
	"github.com/rizome-dev/smolagentsgo/utils"
)

// DefaultToolCallingAgentSystemPrompt is the default system prompt for ToolCallingAgent
const DefaultToolCallingAgentSystemPrompt = `You are a helpful problem-solving assistant. You have access to tools that can help you solve problems.

To solve the problem, you'll go through multiple steps of reasoning and tool use. If you solve the problem, use the 'final_answer' tool with the solution to complete the task.

For each step:
1. Think about what you've learned and what you need to do next.
2. Choose a tool to use. You have access to the following tools:
{{ tools_description }}

When using a tool, use this format:
Thought: <your thinking about what to do next>
Action: <tool_name>(<param1>=<value1>, <param2>=<value2>, ...)
`

// DefaultToolCallingAgentSystemPromptWithManagedAgents extends the default prompt with managed agents
const DefaultToolCallingAgentSystemPromptWithManagedAgents = `You are a helpful problem-solving assistant. You have access to tools and managed agents that can help you solve problems.

To solve the problem, you'll go through multiple steps of reasoning and tool/agent use. If you solve the problem, use the 'final_answer' tool with the solution to complete the task.

For each step:
1. Think about what you've learned and what you need to do next.
2. Choose a tool or agent to use.

You have access to the following tools:
{{ tools_description }}

You have access to the following managed agents that specialize in particular tasks:
{{ managed_agents_description }}

When using a tool or agent, use this format:
Thought: <your thinking about what to do next>
Action: <tool_or_agent_name>(<param1>=<value1>, <param2>=<value2>, ...)
`

// ToolCallingAgent is an agent that uses tools to solve problems
type ToolCallingAgent struct {
	*BaseMultiStepAgent
}

// NewToolCallingAgent creates a new ToolCallingAgent
func NewToolCallingAgent(
	toolsList []tools.Tool,
	model ModelFunc,
	promptTemplates PromptTemplates,
	planningInterval int,
	maxSteps int,
	grammar map[string]string,
	managedAgents []ManagedAgent,
	stepCallbacks []RunCallback,
	name string,
	description string,
	provideRunSummary bool,
	finalAnswerChecks []FinalAnswerCheck,
) (*ToolCallingAgent, error) {

	// If no prompt templates provided, use default
	if promptTemplates.SystemPrompt == "" {
		promptTemplates = EmptyPromptTemplates()

		if managedAgents != nil && len(managedAgents) > 0 {
			promptTemplates.SystemPrompt = DefaultToolCallingAgentSystemPromptWithManagedAgents
		} else {
			promptTemplates.SystemPrompt = DefaultToolCallingAgentSystemPrompt
		}
	}

	baseAgent, err := NewBaseMultiStepAgent(
		toolsList,
		model,
		promptTemplates,
		maxSteps,
		true, // addBaseTools
		grammar,
		managedAgents,
		stepCallbacks,
		planningInterval,
		name,
		description,
		provideRunSummary,
		finalAnswerChecks,
	)

	if err != nil {
		return nil, err
	}

	agent := &ToolCallingAgent{
		BaseMultiStepAgent: baseAgent,
	}

	return agent, nil
}

// InitializeSystemPrompt initializes the system prompt with tool descriptions
func (a *ToolCallingAgent) InitializeSystemPrompt() string {
	// Get the base system prompt
	systemPrompt := a.PromptTemplates.SystemPrompt

	// Generate tool descriptions
	var toolsDescription strings.Builder
	for _, tool := range a.Tools {
		fmt.Fprintf(&toolsDescription, "- %s: %s\n", tool.Name(), tool.Description())

		// Add input parameters if any
		inputs := tool.Inputs()
		if len(inputs) > 0 {
			fmt.Fprintf(&toolsDescription, "  Parameters:\n")
			for name, param := range inputs {
				fmt.Fprintf(&toolsDescription, "  - %s: %s\n", name, param.Description)
			}
		}
		fmt.Fprintf(&toolsDescription, "\n")
	}

	// Generate managed agent descriptions
	var managedAgentsDescription strings.Builder
	for _, agent := range a.ManagedAgents {
		fmt.Fprintf(&managedAgentsDescription, "- %s: %s\n\n", agent.GetName(), agent.GetDescription())
	}

	// Substitute descriptions in the template
	result := systemPrompt
	result = strings.ReplaceAll(result, "{{ tools_description }}", toolsDescription.String())
	result = strings.ReplaceAll(result, "{{ managed_agents_description }}", managedAgentsDescription.String())

	return result
}

// Step executes a single step of the agent
func (a *ToolCallingAgent) Step(memoryStep *memory.ActionStep) (interface{}, error) {
	// Get memory messages
	memoryMessages := a.writeMemoryToMessages(false)

	// Set the model input messages for the step
	memoryStep.ModelInputMessages = memoryMessages

	// Call the model to get the next action
	modelOutput, err := a.Model(memoryMessages, []string{"Action:"})
	if err != nil {
		return nil, fmt.Errorf("error calling model: %w", err)
	}

	memoryStep.ModelOutput = modelOutput.Content
	memoryStep.ModelOutputMessage = modelOutput

	// Extract the action from the model output
	_, actionText, err := a.extractAction(modelOutput.Content, "Action:")
	if err != nil {
		return nil, utils.NewAgentParsingError("failed to extract action from model output", err)
	}

	// Parse the tool call
	toolName, toolArgs, err := a.parseToolCall(actionText)
	if err != nil {
		return nil, utils.NewAgentParsingError("failed to parse tool call", err)
	}

	// Generate a unique ID for the tool call
	toolCallID := fmt.Sprintf("call_%d", a.StepNumber)

	// Create a tool call
	toolCall := &memory.ToolCall{
		Name:      toolName,
		Arguments: toolArgs,
		ID:        toolCallID,
	}

	memoryStep.ToolCalls = []*memory.ToolCall{toolCall}

	// Check if the tool call is to a final answer
	if toolName == "final_answer" {
		finalAnswer, ok := toolArgs["answer"]
		if !ok {
			return nil, utils.NewAgentToolCallError("final_answer tool requires an 'answer' parameter", nil)
		}

		// Check if the final answer passes all checks
		if a.FinalAnswerChecks != nil {
			for _, check := range a.FinalAnswerChecks {
				if !check(finalAnswer, a.Memory) {
					return nil, utils.NewAgentToolCallError("final answer failed validation checks", nil)
				}
			}
		}

		return finalAnswer, nil
	}

	// Check if the tool call is to a managed agent
	if agent, ok := a.ManagedAgents[toolName]; ok {
		result, err := a.executeManagedAgentCall(agent, toolArgs)
		if err != nil {
			memoryStep.Error = utils.NewAgentToolExecutionError(fmt.Sprintf("error executing managed agent %s", toolName), err)
			memoryStep.Observations = fmt.Sprintf("Error: %v", err)
			return nil, memoryStep.Error
		}

		memoryStep.ActionOutput = result
		memoryStep.Observations = fmt.Sprintf("Result from %s: %v", toolName, result)
		return nil, nil
	}

	// Execute the tool call
	result, err := a.executeToolCall(toolName, toolArgs)
	if err != nil {
		memoryStep.Error = utils.NewAgentToolExecutionError(fmt.Sprintf("error executing tool %s", toolName), err)
		memoryStep.Observations = fmt.Sprintf("Error: %v", err)
		return nil, memoryStep.Error
	}

	memoryStep.ActionOutput = result
	memoryStep.Observations = fmt.Sprintf("Result: %v", result)

	return nil, nil
}

// parseToolCall parses a tool call from text
func (a *ToolCallingAgent) parseToolCall(actionText string) (string, map[string]interface{}, error) {
	// Regular expression to parse function call format: functionName(param1="value1", param2="value2")
	re := regexp.MustCompile(`(\w+)\s*\((.*)\)`)
	matches := re.FindStringSubmatch(strings.TrimSpace(actionText))

	if len(matches) < 3 {
		return "", nil, fmt.Errorf("could not parse tool call from action text: %s", actionText)
	}

	toolName := matches[1]
	argsText := matches[2]

	// Try to parse as JSON first
	var args map[string]interface{}
	jsonArgsText := fmt.Sprintf("{%s}", argsText)
	err := json.Unmarshal([]byte(jsonArgsText), &args)
	if err == nil {
		return toolName, args, nil
	}

	// If JSON parsing fails, try to parse as key-value pairs
	args = make(map[string]interface{})

	// Split by commas, handling quoted strings
	splitArgs := splitByComma(argsText)

	for _, arg := range splitArgs {
		if arg == "" {
			continue
		}

		// Split by = to get key-value pairs
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) < 2 {
			return "", nil, fmt.Errorf("invalid argument format for %s", arg)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes from the value if present
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		// Try to convert the value to appropriate type
		args[key] = value
	}

	if len(args) == 0 && argsText != "" {
		return "", nil, fmt.Errorf("could not parse arguments from action text: %s", argsText)
	}

	return toolName, args, nil
}

// splitByComma splits a string by commas, respecting quotes
func splitByComma(s string) []string {
	var result []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(s); i++ {
		char := s[i]

		if char == '\'' && (i == 0 || s[i-1] != '\\') {
			inSingleQuote = !inSingleQuote
			current.WriteByte(char)
		} else if char == '"' && (i == 0 || s[i-1] != '\\') {
			inDoubleQuote = !inDoubleQuote
			current.WriteByte(char)
		} else if char == ',' && !inSingleQuote && !inDoubleQuote {
			result = append(result, current.String())
			current.Reset()
		} else {
			current.WriteByte(char)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// substituteStateVariables substitutes variables from the agent's state into the arguments
func (a *ToolCallingAgent) substituteStateVariables(arguments interface{}) interface{} {
	// If it's a string, substitute variables
	if argStr, ok := arguments.(string); ok {
		// Look for variable patterns like ${varname}
		re := regexp.MustCompile(`\${(\w+)}`)
		return re.ReplaceAllStringFunc(argStr, func(match string) string {
			varName := match[2 : len(match)-1]
			if value, ok := a.State[varName]; ok {
				return fmt.Sprintf("%v", value)
			}
			return match
		})
	}

	// If it's a map, substitute variables in each value
	if argMap, ok := arguments.(map[string]interface{}); ok {
		result := make(map[string]interface{})
		for k, v := range argMap {
			result[k] = a.substituteStateVariables(v)
		}
		return result
	}

	return arguments
}

// executeToolCall executes a tool call
func (a *ToolCallingAgent) executeToolCall(toolName string, arguments map[string]interface{}) (interface{}, error) {
	// Find the tool
	tool, ok := a.Tools[toolName]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}

	// Substitute state variables in the arguments
	processedArgs := a.substituteStateVariables(arguments).(map[string]interface{})

	// Call the tool
	result, err := tool.Call(processedArgs, true)
	if err != nil {
		return nil, fmt.Errorf("error calling tool %s: %w", toolName, err)
	}

	return result, nil
}

// executeManagedAgentCall executes a managed agent call
func (a *ToolCallingAgent) executeManagedAgentCall(agent ManagedAgent, arguments map[string]interface{}) (interface{}, error) {
	// Get the task parameter
	task, ok := arguments["task"]
	if !ok {
		return nil, fmt.Errorf("missing required 'task' parameter for managed agent")
	}

	// Convert task to string
	taskStr := fmt.Sprintf("%v", task)

	// Run the agent with default parameters
	result, err := agent.Run(taskStr, false, true, nil, nil, 0)
	if err != nil {
		return nil, err
	}

	return result, nil
}

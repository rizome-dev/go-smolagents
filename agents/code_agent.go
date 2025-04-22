// Package agents provides agent implementations for various types of AI agents
package agents

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rizome-dev/smolagentsgo/memory"
	"github.com/rizome-dev/smolagentsgo/tools"
	"github.com/rizome-dev/smolagentsgo/utils"
)

// DefaultCodeAgentSystemPrompt is the default system prompt for CodeAgent
const DefaultCodeAgentSystemPrompt = "You are a problem-solving expert. You can solve complex problems using Python code.\n\nTo solve the problem, you'll go through multiple steps of reasoning and code execution. If you solve the problem, use the 'final_answer' tool with the solution to complete the task.\n\nFor each step:\n1. Think about what you've learned and what you need to do next.\n2. Write Python code to solve the problem or explore further.\n\nWhen writing code, use this format:\nThought: <your thinking about what to do next>\nCode:\n```python\n<your Python code>\n```<end_code>\n\nYour code will be executed in a Python environment. You can use the print() function to output results.\n\nYou can use these tools: {{ tools_description }}"

// DefaultCodeAgentSystemPromptWithManagedAgents extends the default prompt with managed agents
const DefaultCodeAgentSystemPromptWithManagedAgents = "You are a problem-solving expert. You can solve complex problems using Python code and delegate to specialized agents.\n\nTo solve the problem, you'll go through multiple steps of reasoning, code execution, and agent delegation. If you solve the problem, use the 'final_answer' tool with the solution to complete the task.\n\nFor each step:\n1. Think about what you've learned and what you need to do next.\n2. Either write Python code or call a managed agent.\n\nWhen writing code, use this format:\nThought: <your thinking about what to do next>\nCode:\n```python\n<your Python code>\n```<end_code>\n\nWhen calling a managed agent, follow the examples in the 'final_answer' tool description.\n\nYour code will be executed in a Python environment. You can use the print() function to output results.\n\nYou can use these tools: {{ tools_description }}\n\nYou can also delegate to these specialized agents: {{ managed_agents_description }}"

// PythonExecutor is an interface for executing Python code
type PythonExecutor interface {
	Execute(code string) (string, error)
	SendVariables(variables map[string]interface{}) error
	SendTools(tools map[string]interface{}) error
}

// CodeAgent is an agent that uses Python code to solve problems
type CodeAgent struct {
	*BaseMultiStepAgent
	PythonExecutor PythonExecutor
}

// NewCodeAgent creates a new CodeAgent
func NewCodeAgent(
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
	pythonExecutor PythonExecutor,
) (*CodeAgent, error) {

	// If no prompt templates provided, use default
	if promptTemplates.SystemPrompt == "" {
		promptTemplates = EmptyPromptTemplates()

		if managedAgents != nil && len(managedAgents) > 0 {
			promptTemplates.SystemPrompt = DefaultCodeAgentSystemPromptWithManagedAgents
		} else {
			promptTemplates.SystemPrompt = DefaultCodeAgentSystemPrompt
		}
	}

	// Set default grammar if not provided
	if grammar == nil {
		grammar = map[string]string{
			"type":  "regex",
			"value": "Thought: .+?\\nCode:\\n```(?:py|python)?\\n(?:.|\\s)+?\\n```<end_code>",
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

	agent := &CodeAgent{
		BaseMultiStepAgent: baseAgent,
		PythonExecutor:     pythonExecutor,
	}

	// Send tools and state to the Python executor
	if pythonExecutor != nil {
		pythonExecutor.SendVariables(agent.State)
		allTools := make(map[string]interface{})

		// Add regular tools
		for name, tool := range agent.Tools {
			allTools[name] = tool
		}

		// Add managed agents
		for name, managedAgent := range agent.ManagedAgents {
			allTools[name] = managedAgent
		}

		pythonExecutor.SendTools(allTools)
	}

	return agent, nil
}

// InitializeSystemPrompt initializes the system prompt with tool descriptions
func (a *CodeAgent) InitializeSystemPrompt() string {
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
func (a *CodeAgent) Step(memoryStep *memory.ActionStep) (interface{}, error) {
	// If the Python executor is not set, return an error
	if a.PythonExecutor == nil {
		return nil, fmt.Errorf("Python executor not set")
	}

	// Get memory messages
	memoryMessages := a.writeMemoryToMessages(false)

	// Set the model input messages for the step
	memoryStep.ModelInputMessages = memoryMessages

	// Call the model to get the next action
	modelOutput, err := a.Model(memoryMessages, []string{"<end_code>"})
	if err != nil {
		return nil, fmt.Errorf("error calling model: %w", err)
	}

	memoryStep.ModelOutput = modelOutput.Content
	memoryStep.ModelOutputMessage = modelOutput

	// Extract the code from the model output
	_, code, err := a.extractAction(modelOutput.Content, "Code:")
	if err != nil {
		return nil, utils.NewAgentParsingError("failed to extract code from model output", err)
	}

	// Extract code blocks
	codeBlocks := utils.ParseCodeBlobs(code, "python")
	if len(codeBlocks) == 0 {
		return nil, utils.NewAgentParsingError("no Python code found in the model output", nil)
	}

	// Get the code to execute
	codeToExecute := codeBlocks[0]

	// Check for final_answer in the code
	if a.containsFinalAnswer(codeToExecute) {
		// Extract the final answer from the code
		finalAnswer, err := a.extractFinalAnswer(codeToExecute)
		if err != nil {
			return nil, err
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

	// Execute the code
	output, err := a.PythonExecutor.Execute(codeToExecute)
	if err != nil {
		// Provide error details to the agent
		memoryStep.Error = utils.NewAgentToolExecutionError("error executing Python code", err)
		memoryStep.Observations = fmt.Sprintf("Error: %v", err)
		return nil, memoryStep.Error
	}

	// Store the observations and action output
	memoryStep.ActionOutput = output
	memoryStep.Observations = output

	return nil, nil
}

// containsFinalAnswer checks if the code contains a call to final_answer
func (a *CodeAgent) containsFinalAnswer(code string) bool {
	// Look for patterns like final_answer(...) or tools["final_answer"](...) or tools.final_answer(...)
	pattern := `(final_answer|tools\["final_answer"\]|tools\.final_answer)\s*\(`
	matched, _ := regexp.MatchString(pattern, code)
	return matched
}

// extractFinalAnswer extracts the final answer from code
func (a *CodeAgent) extractFinalAnswer(code string) (interface{}, error) {
	// Execute the code to get the final answer
	output, err := a.PythonExecutor.Execute(code)
	if err != nil {
		return nil, utils.NewAgentToolExecutionError("error executing final answer code", err)
	}

	// If the code produces output, use that as the final answer
	if output != "" {
		return output, nil
	}

	// Otherwise, look for a final_answer call in the code and extract its arguments
	pattern := `(final_answer|tools\["final_answer"\]|tools\.final_answer)\s*\(\s*["']?([^"'\)]+)["']?\s*\)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(code)

	if len(matches) >= 3 {
		return matches[2], nil
	}

	// If we can't find the specific answer, just return the code as the answer
	return code, nil
}

// Run overrides the base Run method to send variables to the Python executor
func (a *CodeAgent) Run(
	task string,
	stream bool,
	reset bool,
	images []interface{},
	additionalArgs map[string]interface{},
	maxSteps int,
) (interface{}, error) {
	// If additionalArgs is provided, send them to the Python executor
	if additionalArgs != nil && a.PythonExecutor != nil {
		a.PythonExecutor.SendVariables(additionalArgs)
	}

	// Call the base Run method
	return a.BaseMultiStepAgent.Run(task, stream, reset, nil, additionalArgs, maxSteps)
}

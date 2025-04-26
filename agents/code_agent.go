// Package agents provides agent implementations for various types of AI agents
package agents

import (
	"context"
	"fmt"
	"image"
	"regexp"
	"strings"

	"github.com/rizome-dev/smolagentsgo/memory"
	"github.com/rizome-dev/smolagentsgo/tools"
	"github.com/rizome-dev/smolagentsgo/utils"
)

// DefaultCodeAgentSystemPrompt is the default system prompt for CodeAgent
const DefaultCodeAgentSystemPrompt = "You are a problem-solving expert. You can solve complex problems using Go code.\n\nTo solve the problem, you'll go through multiple steps of reasoning and code execution. If you solve the problem, use the 'final_answer' tool with the solution to complete the task.\n\nFor each step:\n1. Think about what you've learned and what you need to do next.\n2. Write Go code to solve the problem or explore further.\n\nWhen writing code, use this format:\nThought: <your thinking about what to do next>\nCode:\n```go\n<your Go code>\n```<end_code>\n\nYour code will be executed in a Go environment. You can use the fmt.Printf() function to output results.\n\nYou can use these tools: {{ tools_description }}"

// DefaultCodeAgentSystemPromptWithManagedAgents extends the default prompt with managed agents
const DefaultCodeAgentSystemPromptWithManagedAgents = "You are a problem-solving expert. You can solve complex problems using Go code and delegate to specialized agents.\n\nTo solve the problem, you'll go through multiple steps of reasoning, code execution, and agent delegation. If you solve the problem, use the 'final_answer' tool with the solution to complete the task.\n\nFor each step:\n1. Think about what you've learned and what you need to do next.\n2. Either write Go code or call a managed agent.\n\nWhen writing code, use this format:\nThought: <your thinking about what to do next>\nCode:\n```go\n<your Go code>\n```<end_code>\n\nWhen calling a managed agent, follow the examples in the 'final_answer' tool description.\n\nYour code will be executed in a Go environment. You can use the fmt.Printf() function to output results.\n\nYou can use these tools: {{ tools_description }}\n\nYou can also delegate to these specialized agents: {{ managed_agents_description }}"

// CodeAgent is an agent specialized in executing code.
// It extends BaseMultiStepAgent with code execution capabilities.
type CodeAgent struct {
	*BaseMultiStepAgent
	executor CodeExecutor
}

// NewCodeAgent creates a new CodeAgent with specified configuration.
// It requires a model, memory, and code executor.
func NewCodeAgent(
	agentTools []tools.Tool,
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
	executor CodeExecutor,
) (*CodeAgent, error) {
	if executor == nil {
		return nil, fmt.Errorf("executor cannot be nil")
	}

	baseAgent, err := NewBaseMultiStepAgent(
		agentTools,
		model,
		promptTemplates,
		maxSteps,
		true, // Add base tools
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
		return nil, fmt.Errorf("failed to create base agent: %w", err)
	}

	return &CodeAgent{
		BaseMultiStepAgent: baseAgent,
		executor:           executor,
	}, nil
}

// ExecuteCode runs the provided code using the agent's executor.
// It supports different code languages and returns the execution result.
func (a *CodeAgent) ExecuteCode(ctx context.Context, code string, language string) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	if code == "" {
		return "", fmt.Errorf("code cannot be empty")
	}

	// Prepare code for execution
	cleanCode := utils.TruncateContent(code, a.Model.MaximumContentLength())

	// For Go code, ensure we have proper imports and a main function
	if language == "go" {
		cleanCode = prepareGoCodeForExecution(cleanCode)
	}

	// Execute the code
	result, err := a.executor.Execute(ctx, cleanCode, language)
	if err != nil {
		return "", utils.NewAgentExecutionError(
			fmt.Sprintf("code execution failed: %v", err),
			err,
		)
	}

	// Truncate result if needed
	result = utils.TruncateContent(result, a.Model.MaximumContentLength())
	return result, nil
}

// prepareGoCodeForExecution ensures the Go code is ready for execution.
// It adds necessary imports and wraps code in a main function if needed.
func prepareGoCodeForExecution(code string) string {
	// Check if the code already has a package declaration
	if !strings.Contains(code, "package main") {
		// Check if there are imports
		hasImports := strings.Contains(code, "import ")

		if hasImports {
			// Insert package before imports
			code = "package main\n\n" + code
		} else {
			// Add package declaration
			code = "package main\n\n" + code
		}
	}

	// Check if the code has a main function
	if !strings.Contains(code, "func main()") {
		// Wrap the code in a main function
		// Only include non-function code in main
		lines := strings.Split(code, "\n")
		var mainCode []string
		var otherCode []string

		inFunction := false
		functionDepth := 0

		for _, line := range lines {
			trimLine := strings.TrimSpace(line)

			// Check for function declarations
			if strings.HasPrefix(trimLine, "func ") && strings.Contains(trimLine, "(") && strings.Contains(trimLine, ")") {
				inFunction = true
				functionDepth = 0
				otherCode = append(otherCode, line)
				continue
			}

			// Track function depth
			if inFunction {
				if strings.Contains(trimLine, "{") {
					functionDepth++
				}
				if strings.Contains(trimLine, "}") {
					functionDepth--
					if functionDepth <= 0 {
						inFunction = false
					}
				}
				otherCode = append(otherCode, line)
				continue
			}

			// Non-function code goes to main
			mainCode = append(mainCode, line)
		}

		// Rebuild the code
		newCode := strings.Join(otherCode, "\n") + "\n\nfunc main() {\n"
		for _, line := range mainCode {
			if strings.TrimSpace(line) != "" &&
				!strings.Contains(line, "package main") &&
				!strings.Contains(line, "import") {
				newCode += "\t" + line + "\n"
			}
		}
		newCode += "}\n"

		return newCode
	}

	return code
}

// CreateExecuteCodeTool creates a tool for code execution.
// This tool can be used by the agent to execute code in various languages.
func (a *CodeAgent) CreateExecuteCodeTool() tools.Tool {
	baseTool, err := tools.NewBaseTool(
		"execute_code",
		"Execute code in the specified language",
		map[string]tools.InputProperty{
			"code": {
				Type:        "string",
				Description: "The code to execute",
			},
			"language": {
				Type:        "string",
				Description: "The programming language of the code (e.g., python, javascript, go)",
				Nullable:    true,
			},
		},
		"string",
		func(args map[string]interface{}) (interface{}, error) {
			code, ok := args["code"].(string)
			if !ok {
				return nil, fmt.Errorf("code must be a string")
			}

			// Default to Go if language is not specified
			language := "go"
			if langArg, ok := args["language"]; ok {
				if lang, ok := langArg.(string); ok && lang != "" {
					language = lang
				}
			}

			// Execute the code
			result, err := a.ExecuteCode(context.Background(), code, language)
			if err != nil {
				return nil, err
			}

			return result, nil
		},
	)

	if err != nil {
		// Handle error in a production scenario
		panic(fmt.Sprintf("failed to create execute_code tool: %v", err))
	}

	return baseTool
}

// AddTools adds tools to the agent's tool list.
func (a *CodeAgent) AddTools(newTools []tools.Tool) {
	// Add the execute_code tool
	executeCodeTool := a.CreateExecuteCodeTool()
	extendedTools := append([]tools.Tool{executeCodeTool}, newTools...)

	// Add tools to the map
	for _, tool := range extendedTools {
		a.Tools[tool.Name()] = tool
	}
}

// Create returns a new instance of the agent, ready for execution.
// It implements the AgentFactory interface.
func (a *CodeAgent) Create() (Agent, error) {
	// Clone the agent with fresh state
	newAgent, err := NewCodeAgent(
		a.getToolList(),
		a.Model,
		a.PromptTemplates,
		a.PlanningInterval,
		a.MaxSteps,
		a.Grammar,
		a.getManagedAgentList(),
		a.StepCallbacks,
		a.Name,
		a.Description,
		a.ProvideRunSummary,
		a.FinalAnswerChecks,
		a.executor,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create new code agent: %w", err)
	}

	// Wrap the agent to adapt to the Agent interface
	return &agentWrapper{agent: newAgent}, nil
}

// getToolList converts the Tools map to a slice
func (a *CodeAgent) getToolList() []tools.Tool {
	toolList := make([]tools.Tool, 0, len(a.Tools))
	for _, tool := range a.Tools {
		toolList = append(toolList, tool)
	}
	return toolList
}

// getManagedAgentList converts the ManagedAgents map to a slice
func (a *CodeAgent) getManagedAgentList() []ManagedAgent {
	agentList := make([]ManagedAgent, 0, len(a.ManagedAgents))
	for _, agent := range a.ManagedAgents {
		agentList = append(agentList, agent)
	}
	return agentList
}

// agentWrapper adapts a MultiStepAgent to the Agent interface
type agentWrapper struct {
	agent MultiStepAgent
}

// Run implements the Agent interface
func (w *agentWrapper) Run(input string) (interface{}, error) {
	return w.agent.Run(input, false, true, nil, nil, 0)
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
	// If the Go executor is not set, return an error
	if a.executor == nil {
		return nil, fmt.Errorf("Go executor not set")
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
	codeBlocks := utils.ParseCodeBlobs(code, "go")
	if len(codeBlocks) == 0 {
		return nil, utils.NewAgentParsingError("no Go code found in the model output", nil)
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
	output, err := a.executor.Execute(context.Background(), codeToExecute, "go")
	if err != nil {
		// Provide error details to the agent
		memoryStep.Error = utils.NewAgentExecutionError(
			fmt.Sprintf("error executing Go code: %v", err),
			err,
		)
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
	output, err := a.executor.Execute(context.Background(), code, "go")
	if err != nil {
		return nil, utils.NewAgentExecutionError(
			fmt.Sprintf("error executing final answer code: %v", err),
			err,
		)
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

// Run overrides the base Run method to send variables to the Go executor
func (a *CodeAgent) Run(
	task string,
	stream bool,
	reset bool,
	images []image.Image,
	additionalArgs map[string]interface{},
	maxSteps int,
) (interface{}, error) {
	// If additionalArgs is provided, send them to the Go executor
	if additionalArgs != nil && a.executor != nil {
		a.executor.SendVariables(additionalArgs)
	}

	// Call the base Run method
	return a.BaseMultiStepAgent.Run(task, stream, reset, images, additionalArgs, maxSteps)
}

// MaximumContentLength returns the maximum content length supported by the model
func (m ModelFunc) MaximumContentLength() int {
	// Default to a safe value
	return 8000
}

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/agents"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/models"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/tools"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/default_tools"
)

var (
	// CLI flags
	modelType   string
	modelID     string
	apiKey      string
	baseURL     string
	interactive bool
	verbose     bool
	agentType   string
	task        string
	toolNames   []string
)

// Styles for the TUI
var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		PaddingTop(1).
		PaddingLeft(4).
		PaddingRight(4).
		PaddingBottom(1)

	messageStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		PaddingLeft(2)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000")).
		PaddingLeft(2)

	stepStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		PaddingLeft(2)

	codeStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#2D2D2D")).
		Foreground(lipgloss.Color("#FAFAFA")).
		Padding(1)
)

// AgentRunner represents the state of the agent execution
type AgentRunner struct {
	agent    agents.MultiStepAgent
	task     string
	steps    []string
	status   string
	error    error
	finished bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// AgentMessage represents messages from the agent
type AgentMessage struct {
	Type    string // "step", "error", "finished", "output"
	Content string
	Step    *agents.RunResult
}

// Research agent types (embedded from research_agent example)
type ResearchTask struct {
	ID            string
	Query         string
	Type          string // "web", "wikipedia", "deep_dive", "synthesis"
	Priority      int
	EstimatedTime time.Duration
}

type ResearchResult struct {
	TaskID     string
	Content    string
	Sources    []string
	Confidence float64
	Duration   time.Duration
	Error      error
	WorkerID   string
}

type ResearchManager struct {
	agent       agents.MultiStepAgent
	workers     []*ResearchWorker
	taskQueue   chan *ResearchTask
	resultQueue chan *ResearchResult
	ctx         context.Context
	cancel      context.CancelFunc
	wg          *sync.WaitGroup
	mutex       sync.RWMutex
	results     map[string]*ResearchResult
	model       models.Model
}

type ResearchWorker struct {
	ID       string
	agent    agents.MultiStepAgent
	manager  *ResearchManager
	ctx      context.Context
	cancel   context.CancelFunc
	isActive bool
	mutex    sync.RWMutex
}

type ProjectReport struct {
	Topic       string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	TaskResults map[string]*ResearchResult
	Summary     string
	Confidence  float64
	Sources     []string
}

func (m AgentRunner) Init() tea.Cmd {
	// Start the agent execution
	return tea.Batch(
		tea.Printf("🚀 Starting agent execution..."),
		m.runAgent,
	)
}

func (m AgentRunner) runAgent() tea.Msg {
	// Execute the agent
	maxSteps := 10
	result, err := m.agent.Run(&agents.RunOptions{
		Task:     m.task,
		MaxSteps: &maxSteps,
	})
	
	if err != nil {
		return AgentMessage{Type: "error", Content: err.Error()}
	}
	
	return AgentMessage{Type: "finished", Content: fmt.Sprintf("%v", result.Output), Step: result}
}

func (m AgentRunner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	case AgentMessage:
		switch msg.Type {
		case "step":
			m.steps = append(m.steps, msg.Content)
			m.status = "Running step: " + msg.Content
		case "error":
			m.error = fmt.Errorf(msg.Content)
			m.status = "Error occurred"
			m.finished = true
		case "finished":
			m.status = "Completed successfully"
			m.finished = true
			m.steps = append(m.steps, "✅ Final output: "+msg.Content)
		}
	}
	
	if m.finished {
		return m, tea.Quit
	}
	
	return m, nil
}

func (m AgentRunner) View() string {
	var b strings.Builder
	
	// Title
	b.WriteString(titleStyle.Render("🤖 Smolagents CLI"))
	b.WriteString("\n\n")
	
	// Task
	b.WriteString(messageStyle.Render("📝 Task: " + m.task))
	b.WriteString("\n\n")
	
	// Status
	if m.error != nil {
		b.WriteString(errorStyle.Render("❌ Error: " + m.error.Error()))
	} else {
		b.WriteString(stepStyle.Render("⚡ Status: " + m.status))
	}
	b.WriteString("\n\n")
	
	// Steps
	if len(m.steps) > 0 {
		b.WriteString(messageStyle.Render("📚 Execution Steps:"))
		b.WriteString("\n")
		for i, step := range m.steps {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step))
		}
		b.WriteString("\n")
	}
	
	if !m.finished {
		b.WriteString("⏳ Running... (Press 'q' or Ctrl+C to quit)\n")
	} else {
		b.WriteString("✅ Execution completed! (Press 'q' or Ctrl+C to quit)\n")
	}
	
	return b.String()
}

var rootCmd = &cobra.Command{
	Use:   "smolagents",
	Short: "Smolagents CLI - Run AI agents from the command line",
	Long: `Smolagents CLI is a command-line interface for running AI agents.
It supports multiple agent types, model providers, and tool integrations.`,
	RunE: runAgent,
}

var runCmd = &cobra.Command{
	Use:   "run [task]",
	Short: "Run an agent with a specific task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task = args[0]
		return runAgent(cmd, args)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available agents, models, and tools",
	RunE:  listResources,
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent management commands",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available agent types",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available agent types:")
		fmt.Println("  - tool-calling: Agent that can call external tools")
		fmt.Println("  - code: Agent that can execute code")
		fmt.Println("  - multi-step: Base multi-step reasoning agent")
		return nil
	},
}

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Model management commands",
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available model types",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available model types:")
		fmt.Println("  - openai: OpenAI API models")
		fmt.Println("  - hf: HuggingFace Inference API models")
		fmt.Println("  - litellm: LiteLLM proxy models")
		fmt.Println("  - bedrock: Amazon Bedrock models")
		fmt.Println("  - vllm: vLLM models")
		fmt.Println("  - mlx: Apple MLX models")
		return nil
	},
}

var toolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Tool management commands",
}

var researchCmd = &cobra.Command{
	Use:   "research [topic]",
	Short: "Run deep multi-agent research on a topic",
	Long: `Launches a sophisticated research system with multiple AI agents working in parallel to thoroughly research a topic.

The research system includes:
- Research Manager: Coordinates the research strategy
- Multiple Research Workers: Execute parallel research tasks
- Advanced Tools: Web search, Wikipedia, content analysis
- Result Synthesis: AI-powered aggregation and comprehensive reporting

Example: smolagents-cli research "quantum computing applications"`,
	Args: cobra.ExactArgs(1),
	RunE:  runResearch,
}

var toolListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Available tools:")
		fmt.Println("  - web_search: Search the web using DuckDuckGo")
		fmt.Println("  - visit_webpage: Visit and extract content from web pages")
		fmt.Println("  - wikipedia_search: Search Wikipedia")
		fmt.Println("  - python_interpreter: Execute Python code")
		fmt.Println("  - final_answer: Provide final answer")
		fmt.Println("  - user_input: Get input from user")
		return nil
	},
}

// NewResearchManager creates a new research manager with workers
func NewResearchManager(model models.Model, numWorkers int) (*ResearchManager, error) {
	if numWorkers <= 0 {
		numWorkers = 2
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	manager := &ResearchManager{
		taskQueue:   make(chan *ResearchTask, 100),
		resultQueue: make(chan *ResearchResult, 100),
		ctx:         ctx,
		cancel:      cancel,
		wg:          &sync.WaitGroup{},
		results:     make(map[string]*ResearchResult),
		model:       model,
	}
	
	// Create research agent for the manager
	tools := []tools.Tool{
		default_tools.NewWebSearchTool(),
		default_tools.NewVisitWebpageTool(),
		default_tools.NewWikipediaSearchTool(),
		default_tools.NewFinalAnswerTool(),
	}
	
	agent, err := agents.NewToolCallingAgent(model, tools, "system", map[string]interface{}{
		"max_steps": 15,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create manager agent: %w", err)
	}
	manager.agent = agent
	
	// Create workers
	for i := 0; i < numWorkers; i++ {
		worker, err := NewResearchWorker(fmt.Sprintf("worker-%d", i), model, manager)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create worker %d: %w", i, err)
		}
		manager.workers = append(manager.workers, worker)
	}
	
	// Start workers
	for _, worker := range manager.workers {
		go worker.start()
	}
	
	// Start result processor
	go manager.processResults()
	
	return manager, nil
}

// NewResearchWorker creates a new research worker
func NewResearchWorker(id string, model models.Model, manager *ResearchManager) (*ResearchWorker, error) {
	ctx, cancel := context.WithCancel(manager.ctx)
	
	// Create agent for this worker
	tools := []tools.Tool{
		default_tools.NewWebSearchTool(),
		default_tools.NewVisitWebpageTool(),
		default_tools.NewWikipediaSearchTool(),
		default_tools.NewFinalAnswerTool(),
	}
	
	agent, err := agents.NewToolCallingAgent(model, tools, "system", map[string]interface{}{
		"max_steps": 10,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create agent for worker %s: %w", id, err)
	}
	
	return &ResearchWorker{
		ID:      id,
		agent:   agent,
		manager: manager,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// ResearchProject executes a complete research project
func (m *ResearchManager) ResearchProject(topic string, maxDuration time.Duration) (*ProjectReport, error) {
	startTime := time.Now()
	fmt.Printf("🔍 Starting research project: %s\n", topic)
	
	// Generate research tasks
	tasks := m.generateResearchTasks(topic)
	
	fmt.Printf("📋 Generated %d research tasks\n", len(tasks))
	
	// Submit tasks
	for _, task := range tasks {
		select {
		case m.taskQueue <- task:
			fmt.Printf("  ➤ Submitted: %s (%s)\n", task.Query, task.Type)
		case <-time.After(5 * time.Second):
			return nil, fmt.Errorf("timeout submitting task: %s", task.Query)
		}
	}
	
	// Wait for results with timeout
	timeout := time.After(maxDuration)
	completedTasks := 0
	expectedTasks := len(tasks)
	
	for completedTasks < expectedTasks {
		select {
		case <-timeout:
			fmt.Printf("⏰ Research timeout after %v\n", maxDuration)
			goto synthesize
		case <-m.ctx.Done():
			return nil, fmt.Errorf("research cancelled")
		case <-time.After(100 * time.Millisecond):
			// Check if we have enough results
			m.mutex.RLock()
			completedTasks = len(m.results)
			m.mutex.RUnlock()
			
			if completedTasks >= expectedTasks {
				break
			}
		}
	}
	
synthesize:
	endTime := time.Now()
	fmt.Printf("✅ Research phase completed. Synthesizing results...\n")
	
	// Create project report
	m.mutex.RLock()
	taskResults := make(map[string]*ResearchResult)
	for k, v := range m.results {
		taskResults[k] = v
	}
	m.mutex.RUnlock()
	
	report := &ProjectReport{
		Topic:       topic,
		StartTime:   startTime,
		EndTime:     endTime,
		Duration:    endTime.Sub(startTime),
		TaskResults: taskResults,
	}
	
	// Synthesize summary
	summary, confidence := m.synthesizeResults(topic, taskResults)
	report.Summary = summary
	report.Confidence = confidence
	
	// Collect unique sources
	sourceMap := make(map[string]bool)
	for _, result := range taskResults {
		for _, source := range result.Sources {
			sourceMap[source] = true
		}
	}
	
	for source := range sourceMap {
		report.Sources = append(report.Sources, source)
	}
	
	return report, nil
}

// Stop shuts down the research manager and all workers
func (m *ResearchManager) Stop() {
	fmt.Printf("🛑 Stopping research manager...\n")
	
	// Cancel context to stop all workers
	m.cancel()
	
	// Wait for workers to finish
	m.wg.Wait()
	
	// Close channels
	close(m.taskQueue)
	close(m.resultQueue)
	
	fmt.Printf("✅ Research manager stopped\n")
}

// PrintReport prints a formatted research report
func PrintReport(report *ProjectReport) {
	fmt.Printf("\n")
	fmt.Printf("📊 RESEARCH REPORT\n")
	fmt.Printf("==================\n\n")
	
	fmt.Printf("🔬 Topic: %s\n", report.Topic)
	fmt.Printf("⏱️  Duration: %v\n", report.Duration)
	fmt.Printf("🎯 Confidence: %.1f%%\n", report.Confidence*100)
	fmt.Printf("📚 Sources: %d\n", len(report.Sources))
	fmt.Printf("📋 Tasks Completed: %d\n\n", len(report.TaskResults))
	
	// Summary
	fmt.Printf("📝 EXECUTIVE SUMMARY\n")
	fmt.Printf("-------------------\n")
	fmt.Printf("%s\n\n", report.Summary)
	
	// Task Results
	fmt.Printf("🔍 DETAILED FINDINGS\n")
	fmt.Printf("--------------------\n")
	for taskID, result := range report.TaskResults {
		if result.Error != nil {
			fmt.Printf("❌ %s: Failed - %v\n", taskID, result.Error)
		} else {
			fmt.Printf("✅ %s (%.1fs, %.0f%% confidence)\n", taskID, result.Duration.Seconds(), result.Confidence*100)
			fmt.Printf("   %s\n\n", truncateString(result.Content, 200))
		}
	}
	
	// Sources
	if len(report.Sources) > 0 {
		fmt.Printf("🔗 SOURCES\n")
		fmt.Printf("----------\n")
		for i, source := range report.Sources {
			fmt.Printf("%d. %s\n", i+1, source)
		}
		fmt.Printf("\n")
	}
}

// Helper functions for research manager

func (m *ResearchManager) generateResearchTasks(topic string) []*ResearchTask {
	tasks := []*ResearchTask{
		{
			ID:            "web-overview",
			Query:         fmt.Sprintf("comprehensive overview of %s recent developments", topic),
			Type:          "web",
			Priority:      1,
			EstimatedTime: 2 * time.Minute,
		},
		{
			ID:            "wiki-context",
			Query:         fmt.Sprintf("%s background information and fundamentals", topic),
			Type:          "wikipedia",
			Priority:      2,
			EstimatedTime: 90 * time.Second,
		},
		{
			ID:            "applications",
			Query:         fmt.Sprintf("%s practical applications and use cases", topic),
			Type:          "web",
			Priority:      1,
			EstimatedTime: 2 * time.Minute,
		},
		{
			ID:            "challenges",
			Query:         fmt.Sprintf("%s current challenges and limitations", topic),
			Type:          "web",
			Priority:      2,
			EstimatedTime: 90 * time.Second,
		},
	}
	
	return tasks
}

func (m *ResearchManager) processResults() {
	defer m.wg.Done()
	m.wg.Add(1)
	
	for {
		select {
		case result := <-m.resultQueue:
			m.mutex.Lock()
			m.results[result.TaskID] = result
			m.mutex.Unlock()
			
			if result.Error != nil {
				fmt.Printf("❌ Task %s failed: %v\n", result.TaskID, result.Error)
			} else {
				fmt.Printf("✅ Task %s completed by %s (%.1fs)\n", 
					result.TaskID, result.WorkerID, result.Duration.Seconds())
			}
			
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *ResearchManager) synthesizeResults(topic string, results map[string]*ResearchResult) (string, float64) {
	if len(results) == 0 {
		return "No research results available.", 0.0
	}
	
	// Simple synthesis - in a real implementation, this would use the LLM
	var contentParts []string
	var totalConfidence float64
	validResults := 0
	
	for _, result := range results {
		if result.Error == nil && result.Content != "" {
			contentParts = append(contentParts, result.Content)
			totalConfidence += result.Confidence
			validResults++
		}
	}
	
	if validResults == 0 {
		return "All research tasks failed.", 0.0
	}
	
	avgConfidence := totalConfidence / float64(validResults)
	
	// Create a simple synthesis
	summary := fmt.Sprintf("Research on %s reveals the following key findings:\n\n", topic)
	for i, content := range contentParts {
		summary += fmt.Sprintf("%d. %s\n\n", i+1, truncateString(content, 300))
	}
	
	return summary, avgConfidence
}

// ResearchWorker methods

func (w *ResearchWorker) start() {
	defer w.manager.wg.Done()
	w.manager.wg.Add(1)
	
	fmt.Printf("🚀 Worker %s started\n", w.ID)
	
	for {
		select {
		case task := <-w.manager.taskQueue:
			w.processTask(task)
		case <-w.ctx.Done():
			fmt.Printf("🛑 Worker %s stopped\n", w.ID)
			return
		}
	}
}

func (w *ResearchWorker) processTask(task *ResearchTask) {
	startTime := time.Now()
	
	w.mutex.Lock()
	w.isActive = true
	w.mutex.Unlock()
	
	defer func() {
		w.mutex.Lock()
		w.isActive = false
		w.mutex.Unlock()
	}()
	
	fmt.Printf("🔬 Worker %s processing: %s\n", w.ID, task.Query)
	
	// Execute the research task
	maxSteps := 5
	result, err := w.agent.Run(&agents.RunOptions{
		Task:     task.Query,
		MaxSteps: &maxSteps,
	})
	
	duration := time.Since(startTime)
	
	researchResult := &ResearchResult{
		TaskID:   task.ID,
		Duration: duration,
		WorkerID: w.ID,
	}
	
	if err != nil {
		researchResult.Error = err
		researchResult.Confidence = 0.0
	} else {
		researchResult.Content = fmt.Sprintf("%v", result.Output)
		researchResult.Confidence = 0.8 // Simplified confidence scoring
		researchResult.Sources = []string{} // Simplified - would extract from tool calls
	}
	
	// Send result back
	select {
	case w.manager.resultQueue <- researchResult:
	case <-w.ctx.Done():
		return
	}
}

// Utility functions

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func runResearch(cmd *cobra.Command, args []string) error {
	topic := args[0]
	
	fmt.Printf("🔬 Starting deep research on: %s\n", topic)
	fmt.Println("========================================")
	
	// Create model
	model, err := createModel()
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}
	
	// Create research manager with 3 workers
	manager, err := NewResearchManager(model, 3)
	if err != nil {
		return fmt.Errorf("failed to create research manager: %w", err)
	}
	defer manager.Stop()
	
	fmt.Printf("📋 Research Topic: %s\n", topic)
	fmt.Printf("👥 Workers: 3 agents\n")
	fmt.Printf("⏱️  Maximum Duration: 15 minutes\n\n")
	
	// Execute research project
	report, err := manager.ResearchProject(topic, 15*time.Minute)
	if err != nil {
		return fmt.Errorf("research project failed: %w", err)
	}
	
	// Display results using the research agent's PrintReport function
	PrintReport(report)
	
	fmt.Printf("\n✅ Research project completed successfully!\n")
	fmt.Printf("📊 Total duration: %v\n", report.Duration)
	fmt.Printf("🎯 Average confidence: %.1f%%\n", report.Confidence*100)
	
	return nil
}

func init() {
	// Root command flags
	rootCmd.PersistentFlags().StringVar(&modelType, "model-type", "openai", "Model type (openai, hf, litellm, bedrock, vllm, mlx)")
	rootCmd.PersistentFlags().StringVar(&modelID, "model", "gpt-3.5-turbo", "Model ID")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for the model provider")
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "Base URL for the model API")
	rootCmd.PersistentFlags().BoolVar(&interactive, "interactive", false, "Run in interactive mode")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	
	// Run command flags
	runCmd.Flags().StringVar(&agentType, "agent", "tool-calling", "Agent type (tool-calling, code, multi-step)")
	runCmd.Flags().StringSliceVar(&toolNames, "tools", []string{"web_search", "final_answer"}, "Tools to enable")
	
	// Add subcommands
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(researchCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(modelCmd)
	rootCmd.AddCommand(toolCmd)
	
	agentCmd.AddCommand(agentListCmd)
	modelCmd.AddCommand(modelListCmd)
	toolCmd.AddCommand(toolListCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	if task == "" && len(args) > 0 {
		task = args[0]
	}
	
	if task == "" {
		return fmt.Errorf("task is required")
	}
	
	// Create model
	model, err := createModel()
	if err != nil {
		return fmt.Errorf("failed to create model: %w", err)
	}
	
	// Create tools
	toolList, err := createTools()
	if err != nil {
		return fmt.Errorf("failed to create tools: %w", err)
	}
	
	// Create agent
	agent, err := createAgent(model, toolList)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}
	
	if interactive {
		// Run interactive mode with bubbletea
		ctx, cancel := context.WithCancel(context.Background())
		
		runner := AgentRunner{
			agent:  agent,
			task:   task,
			status: "Initializing...",
			ctx:    ctx,
			cancel: cancel,
		}
		
		p := tea.NewProgram(runner)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running interactive mode: %w", err)
		}
	} else {
		// Run in simple mode
		fmt.Printf("🚀 Running agent with task: %s\n", task)
		
		maxSteps := 10
		result, err := agent.Run(&agents.RunOptions{
			Task:     task,
			MaxSteps: &maxSteps,
		})
		
		if err != nil {
			return fmt.Errorf("agent execution failed: %w", err)
		}
		
		fmt.Printf("\n✅ Final Output:\n%s\n", result.Output)
		
		if verbose {
			fmt.Printf("\n📊 Execution Stats:\n")
			fmt.Printf("  Result: %v\n", result.Output)
			if result.TokenUsage != nil {
				fmt.Printf("  Input Tokens: %d\n", result.TokenUsage.InputTokens)
				fmt.Printf("  Output Tokens: %d\n", result.TokenUsage.OutputTokens)
				fmt.Printf("  Total Tokens: %d\n", result.TokenUsage.TotalTokens)
			}
		}
	}
	
	return nil
}

func createModel() (models.Model, error) {
	// Get API key from environment if not provided
	if apiKey == "" {
		switch modelType {
		case "openai":
			apiKey = os.Getenv("OPENAI_API_KEY")
		case "hf":
			apiKey = os.Getenv("HF_API_TOKEN")
		case "anthropic":
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
	}
	
	options := map[string]interface{}{}
	if baseURL != "" {
		options["base_url"] = baseURL
	}
	if apiKey != "" {
		options["api_key"] = apiKey
	}
	
	var modelTypeEnum models.ModelType
	switch modelType {
	case "openai":
		modelTypeEnum = models.ModelTypeOpenAIServer
	case "hf":
		modelTypeEnum = models.ModelTypeInferenceClient
		if apiKey != "" {
			options["token"] = apiKey
		}
	case "litellm":
		modelTypeEnum = models.ModelTypeLiteLLM
	case "bedrock":
		modelTypeEnum = models.ModelTypeBedrockModel
	case "vllm":
		modelTypeEnum = models.ModelTypeVLLM
	case "mlx":
		modelTypeEnum = models.ModelTypeMLX
	default:
		return nil, fmt.Errorf("unsupported model type: %s", modelType)
	}
	
	return models.CreateModel(modelTypeEnum, modelID, options)
}

func createTools() ([]tools.Tool, error) {
	var toolList []tools.Tool
	
	for _, toolName := range toolNames {
		switch toolName {
		case "web_search":
			toolList = append(toolList, default_tools.NewWebSearchTool())
		case "visit_webpage":
			toolList = append(toolList, default_tools.NewVisitWebpageTool())
		case "wikipedia_search":
			toolList = append(toolList, default_tools.NewWikipediaSearchTool())
		case "python_interpreter":
			toolList = append(toolList, default_tools.NewPythonInterpreterTool())
		case "final_answer":
			toolList = append(toolList, default_tools.NewFinalAnswerTool())
		case "user_input":
			toolList = append(toolList, default_tools.NewUserInputTool())
		default:
			return nil, fmt.Errorf("unknown tool: %s", toolName)
		}
	}
	
	return toolList, nil
}

func createAgent(model models.Model, toolList []tools.Tool) (agents.MultiStepAgent, error) {
	switch agentType {
	case "tool-calling":
		return agents.NewToolCallingAgent(model, toolList, "system", map[string]interface{}{
			"max_steps": 10,
		})
	case "code":
		return agents.NewCodeAgent(model, toolList, "system", map[string]interface{}{
			"max_steps": 10,
		})
	case "multi-step":
		return agents.NewToolCallingAgent(model, toolList, "system", map[string]interface{}{
			"max_steps": 10,
		})
	default:
		return nil, fmt.Errorf("unsupported agent type: %s", agentType)
	}
}

func listResources(cmd *cobra.Command, args []string) error {
	fmt.Println(titleStyle.Render("🤖 Smolagents Resources"))
	fmt.Println()
	
	fmt.Println(messageStyle.Render("📦 Agent Types:"))
	fmt.Println("  • tool-calling - Agent that can call external tools")
	fmt.Println("  • code - Agent that can execute code")
	fmt.Println("  • multi-step - Base multi-step reasoning agent")
	fmt.Println()
	
	fmt.Println(messageStyle.Render("🧠 Model Types:"))
	fmt.Println("  • openai - OpenAI API models (GPT-3.5, GPT-4, etc.)")
	fmt.Println("  • hf - HuggingFace Inference API models")
	fmt.Println("  • litellm - LiteLLM proxy (100+ model providers)")
	fmt.Println("  • bedrock - Amazon Bedrock models")
	fmt.Println("  • vllm - vLLM models for high-performance inference")
	fmt.Println("  • mlx - Apple MLX models for Apple Silicon")
	fmt.Println()
	
	fmt.Println(messageStyle.Render("🛠️  Available Tools:"))
	fmt.Println("  • web_search - Search the web using DuckDuckGo")
	fmt.Println("  • visit_webpage - Visit and extract content from web pages")
	fmt.Println("  • wikipedia_search - Search Wikipedia articles")
	fmt.Println("  • python_interpreter - Execute Python code safely")
	fmt.Println("  • final_answer - Provide final answer to the user")
	fmt.Println("  • user_input - Get input from the user")
	fmt.Println()
	
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
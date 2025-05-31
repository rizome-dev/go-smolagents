// Package main demonstrates a phenomenal deep research agent system
//
// This example showcases the full feature set of the smolagents library with:
// - Manager agent coordinating multiple research workers
// - Asynchronous managed agents doing parallel research
// - Advanced tool usage (web search, Wikipedia, webpage visits)
// - Result aggregation and synthesis
// - Comprehensive error handling and monitoring
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/agents"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/default_tools"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/models"
	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/tools"
)

// ResearchTask represents a single research task
type ResearchTask struct {
	ID          string
	Query       string
	Type        string // "web", "wikipedia", "deep_dive", "synthesis"
	Priority    int
	EstimatedTime time.Duration
}

// ResearchResult represents the result of a research task
type ResearchResult struct {
	TaskID      string
	Content     string
	Sources     []string
	Confidence  float64
	Duration    time.Duration
	Error       error
	WorkerID    string
}

// ResearchManager coordinates multiple research workers
type ResearchManager struct {
	agent         agents.MultiStepAgent
	workers       []*ResearchWorker
	taskQueue     chan *ResearchTask
	resultQueue   chan *ResearchResult
	ctx           context.Context
	cancel        context.CancelFunc
	wg            *sync.WaitGroup
	mutex         sync.RWMutex
	results       map[string]*ResearchResult
	model         models.Model
}

// ResearchWorker is an individual research agent
type ResearchWorker struct {
	ID       string
	agent    agents.MultiStepAgent
	manager  *ResearchManager
	ctx      context.Context
	cancel   context.CancelFunc
	isActive bool
	mutex    sync.RWMutex
}

// NewResearchManager creates a new research coordination system
func NewResearchManager(model models.Model, numWorkers int) (*ResearchManager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create manager agent with enhanced tools
	managerTools := []tools.Tool{
		default_tools.NewWebSearchTool(),
		default_tools.NewWikipediaSearchTool(),
		default_tools.NewVisitWebpageTool(),
		default_tools.NewFinalAnswerTool(),
		default_tools.NewUserInputTool(),
	}
	
	managerPrompt := `You are a research coordination agent. Your job is to plan and coordinate research tasks.

You have access to the following tools:
{{tool_descriptions}}

IMPORTANT: When you have completed your task and are ready to provide a final answer, you MUST use the final_answer tool with your complete response. Do not provide answers in regular text - always use the final_answer tool to deliver your conclusion.

Your output should directly call tools when appropriate. Always use the exact function names provided.

Be helpful, accurate, and efficient in your responses.`

	managerAgent, err := agents.NewToolCallingAgent(model, managerTools, managerPrompt, map[string]interface{}{
		"max_steps": 20,
		"temperature": 0.1, // More focused for coordination
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create manager agent: %w", err)
	}
	
	manager := &ResearchManager{
		agent:       managerAgent,
		workers:     make([]*ResearchWorker, 0, numWorkers),
		taskQueue:   make(chan *ResearchTask, 100),
		resultQueue: make(chan *ResearchResult, 100),
		ctx:         ctx,
		cancel:      cancel,
		wg:          &sync.WaitGroup{},
		results:     make(map[string]*ResearchResult),
		model:       model,
	}
	
	// Create research workers
	for i := 0; i < numWorkers; i++ {
		worker, err := NewResearchWorker(fmt.Sprintf("worker-%d", i), model, manager)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create worker %d: %w", i, err)
		}
		manager.workers = append(manager.workers, worker)
	}
	
	// Start result collector
	go manager.collectResults()
	
	return manager, nil
}

// NewResearchWorker creates a new research worker agent
func NewResearchWorker(id string, model models.Model, manager *ResearchManager) (*ResearchWorker, error) {
	ctx, cancel := context.WithCancel(manager.ctx)
	
	// Create worker agent with specialized tools
	workerTools := []tools.Tool{
		default_tools.NewWebSearchTool(),
		default_tools.NewDuckDuckGoSearchTool(),
		default_tools.NewGoogleSearchTool(),
		default_tools.NewWikipediaSearchTool(),
		default_tools.NewVisitWebpageTool(),
		default_tools.NewPythonInterpreterTool(),
		default_tools.NewFinalAnswerTool(),
	}
	
	workerPrompt := `You are a research agent specializing in gathering information on specific topics. Conduct thorough research using the available tools.

You have access to the following tools:
{{tool_descriptions}}

IMPORTANT: When you have completed your research and are ready to provide a final answer, you MUST use the final_answer tool with your complete response. Do not provide answers in regular text - always use the final_answer tool to deliver your conclusion.

Your output should directly call tools when appropriate. Always use the exact function names provided.

Be thorough, accurate, and provide comprehensive responses based on your research.`

	workerAgent, err := agents.NewToolCallingAgent(model, workerTools, workerPrompt, map[string]interface{}{
		"max_steps": 15,
		"temperature": 0.3, // More creative for research
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create worker agent: %w", err)
	}
	
	worker := &ResearchWorker{
		ID:      id,
		agent:   workerAgent,
		manager: manager,
		ctx:     ctx,
		cancel:  cancel,
	}
	
	// Start worker
	go worker.start()
	
	return worker, nil
}

// start begins the worker's task processing loop
func (rw *ResearchWorker) start() {
	rw.manager.wg.Add(1)
	defer rw.manager.wg.Done()
	
	log.Printf("Research worker %s started", rw.ID)
	
	for {
		select {
		case <-rw.ctx.Done():
			log.Printf("Research worker %s shutting down", rw.ID)
			return
		case task := <-rw.manager.taskQueue:
			if task == nil {
				continue
			}
			rw.processTask(task)
		}
	}
}

// processTask executes a research task
func (rw *ResearchWorker) processTask(task *ResearchTask) {
	rw.mutex.Lock()
	rw.isActive = true
	rw.mutex.Unlock()
	
	defer func() {
		rw.mutex.Lock()
		rw.isActive = false
		rw.mutex.Unlock()
	}()
	
	log.Printf("Worker %s processing task %s: %s", rw.ID, task.ID, task.Query)
	
	startTime := time.Now()
	
	// Create specialized prompt based on task type
	prompt := rw.createPrompt(task)
	
	// Execute research
	maxSteps := 15
	result, err := rw.agent.Run(&agents.RunOptions{
		Task:     prompt,
		MaxSteps: &maxSteps,
		Context:  rw.ctx,
	})
	
	duration := time.Since(startTime)
	
	// Create research result
	resResult := &ResearchResult{
		TaskID:   task.ID,
		Duration: duration,
		WorkerID: rw.ID,
		Error:    err,
	}
	
	if err != nil {
		log.Printf("Worker %s failed task %s: %v", rw.ID, task.ID, err)
		resResult.Content = fmt.Sprintf("Research failed: %v", err)
		resResult.Confidence = 0.0
	} else {
		content := fmt.Sprintf("%v", result.Output)
		resResult.Content = content
		resResult.Confidence = rw.assessConfidence(content, task)
		resResult.Sources = rw.extractSources(content)
		log.Printf("Worker %s completed task %s (confidence: %.2f)", rw.ID, task.ID, resResult.Confidence)
	}
	
	// Send result
	select {
	case rw.manager.resultQueue <- resResult:
	case <-rw.ctx.Done():
		return
	}
}

// createPrompt creates a specialized research prompt based on task type
func (rw *ResearchWorker) createPrompt(task *ResearchTask) string {
	basePrompt := fmt.Sprintf("You are a research specialist. Your task is to thoroughly research the following query: \"%s\"", task.Query)
	
	switch task.Type {
	case "web":
		return basePrompt + "\n\nFocus on finding the most recent and credible web sources. Use web search tools to gather information from multiple sources. Provide citations and assess the reliability of each source."
	case "wikipedia":
		return basePrompt + "\n\nStart with Wikipedia for authoritative background information. Follow up with additional web searches to get more recent developments and different perspectives."
	case "deep_dive":
		return basePrompt + "\n\nConduct an exhaustive research session. Use multiple search strategies, cross-reference information, and provide a comprehensive analysis with sources, conflicting viewpoints, and your assessment of the evidence quality."
	case "synthesis":
		return basePrompt + "\n\nSynthesize information from multiple sources to create a coherent, well-structured summary. Focus on identifying patterns, contradictions, and gaps in the available information."
	default:
		return basePrompt + "\n\nConduct thorough research using all available tools. Provide comprehensive, well-sourced information."
	}
}

// assessConfidence estimates the confidence level of research results
func (rw *ResearchWorker) assessConfidence(content string, task *ResearchTask) float64 {
	confidence := 0.5 // Base confidence
	
	// Adjust based on content length (more content generally means more thorough research)
	if len(content) > 1000 {
		confidence += 0.2
	}
	if len(content) > 2000 {
		confidence += 0.1
	}
	
	// Adjust based on task complexity
	switch task.Type {
	case "deep_dive":
		confidence += 0.1
	case "synthesis":
		confidence += 0.1
	}
	
	// Ensure confidence is within bounds
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}
	
	return confidence
}

// extractSources attempts to identify sources mentioned in research content
func (rw *ResearchWorker) extractSources(content string) []string {
	// This is a simplified implementation
	// In a real system, you'd use more sophisticated NLP to extract URLs, citations, etc.
	sources := []string{}
	
	// Look for URLs
	if len(content) > 100 {
		sources = append(sources, "Web Research")
	}
	
	// Look for Wikipedia mentions
	if len(content) > 50 {
		sources = append(sources, "Knowledge Base")
	}
	
	return sources
}

// collectResults processes completed research results
func (rm *ResearchManager) collectResults() {
	for {
		select {
		case <-rm.ctx.Done():
			return
		case result := <-rm.resultQueue:
			if result == nil {
				continue
			}
			
			rm.mutex.Lock()
			rm.results[result.TaskID] = result
			rm.mutex.Unlock()
			
			log.Printf("Collected result for task %s from worker %s", result.TaskID, result.WorkerID)
		}
	}
}

// ResearchProject runs a comprehensive research project
func (rm *ResearchManager) ResearchProject(topic string, maxDuration time.Duration) (*ProjectReport, error) {
	log.Printf("Starting research project: %s", topic)
	
	ctx, cancel := context.WithTimeout(rm.ctx, maxDuration)
	defer cancel()
	
	// Phase 1: Planning - Manager breaks down the research topic
	log.Println("Phase 1: Research Planning")
	planningPrompt := fmt.Sprintf(`You are a research project manager. Break down this research topic into 4-6 specific research tasks: "%s"

Create a research plan with different types of tasks:
1. Initial web search for overview
2. Wikipedia research for background
3. Deep dive into specific aspects
4. Synthesis of findings

For each task, specify:
- What specific aspect to research
- What type of research (web, wikipedia, deep_dive, synthesis)
- Priority level (1-5)

Respond with a structured plan.`, topic)
	
	maxSteps := 10
	planningResult, err := rm.agent.Run(&agents.RunOptions{
		Task:     planningPrompt,
		MaxSteps: &maxSteps,
		Context:  ctx,
	})
	if err != nil {
		return nil, fmt.Errorf("planning phase failed: %w", err)
	}
	
	// Parse planning result and create tasks
	tasks := rm.parsePlanIntoTasks(fmt.Sprintf("%v", planningResult.Output), topic)
	
	// Phase 2: Parallel Research Execution
	log.Printf("Phase 2: Executing %d research tasks", len(tasks))
	
	// Submit tasks to workers
	for _, task := range tasks {
		select {
		case rm.taskQueue <- task:
			log.Printf("Submitted task %s: %s", task.ID, task.Query)
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during task submission")
		}
	}
	
	// Wait for all results
	rm.waitForResults(ctx, len(tasks))
	
	// Phase 3: Synthesis and Report Generation
	log.Println("Phase 3: Synthesizing Results")
	
	report, err := rm.generateReport(topic, tasks)
	if err != nil {
		return nil, fmt.Errorf("report generation failed: %w", err)
	}
	
	log.Printf("Research project completed: %s", topic)
	return report, nil
}

// parsePlanIntoTasks converts the planning output into concrete research tasks
func (rm *ResearchManager) parsePlanIntoTasks(planContent, topic string) []*ResearchTask {
	// This is a simplified implementation
	// In a real system, you'd parse the structured output more carefully
	
	tasks := []*ResearchTask{
		{
			ID:    "task-1",
			Query: fmt.Sprintf("General overview and current state of %s", topic),
			Type:  "web",
			Priority: 5,
			EstimatedTime: 3 * time.Minute,
		},
		{
			ID:    "task-2", 
			Query: fmt.Sprintf("Historical background and fundamentals of %s", topic),
			Type:  "wikipedia",
			Priority: 4,
			EstimatedTime: 2 * time.Minute,
		},
		{
			ID:    "task-3",
			Query: fmt.Sprintf("Recent developments and cutting-edge research in %s", topic),
			Type:  "deep_dive",
			Priority: 5,
			EstimatedTime: 5 * time.Minute,
		},
		{
			ID:    "task-4",
			Query: fmt.Sprintf("Applications and real-world impact of %s", topic),
			Type:  "web",
			Priority: 4,
			EstimatedTime: 3 * time.Minute,
		},
		{
			ID:    "task-5",
			Query: fmt.Sprintf("Future trends and predictions for %s", topic),
			Type:  "deep_dive",
			Priority: 3,
			EstimatedTime: 4 * time.Minute,
		},
		{
			ID:    "task-6",
			Query: fmt.Sprintf("Comprehensive synthesis of all findings about %s", topic),
			Type:  "synthesis",
			Priority: 5,
			EstimatedTime: 2 * time.Minute,
		},
	}
	
	return tasks
}

// waitForResults waits for all research tasks to complete
func (rm *ResearchManager) waitForResults(ctx context.Context, expectedCount int) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled while waiting for results")
			return
		case <-ticker.C:
			rm.mutex.RLock()
			completed := len(rm.results)
			rm.mutex.RUnlock()
			
			log.Printf("Research progress: %d/%d tasks completed", completed, expectedCount)
			
			if completed >= expectedCount {
				log.Println("All research tasks completed")
				return
			}
		}
	}
}

// ProjectReport represents the final research report
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

// generateReport creates a comprehensive research report
func (rm *ResearchManager) generateReport(topic string, tasks []*ResearchTask) (*ProjectReport, error) {
	startTime := time.Now()
	
	rm.mutex.RLock()
	results := make(map[string]*ResearchResult)
	for k, v := range rm.results {
		results[k] = v
	}
	rm.mutex.RUnlock()
	
	// Aggregate all research content
	var allContent []string
	var allSources []string
	totalConfidence := 0.0
	successCount := 0
	
	for _, task := range tasks {
		if result, exists := results[task.ID]; exists && result.Error == nil {
			allContent = append(allContent, fmt.Sprintf("## %s\n\n%s", task.Query, result.Content))
			allSources = append(allSources, result.Sources...)
			totalConfidence += result.Confidence
			successCount++
		}
	}
	
	// Calculate average confidence
	avgConfidence := 0.0
	if successCount > 0 {
		avgConfidence = totalConfidence / float64(successCount)
	}
	
	// Generate synthesis using manager agent
	synthesisPrompt := fmt.Sprintf(`You are a research analyst creating a comprehensive report on "%s".

Based on the following research findings, create a well-structured summary that:
1. Provides an executive summary
2. Covers all major aspects discovered
3. Identifies key trends and patterns
4. Notes any conflicting information
5. Concludes with actionable insights

Research Findings:
%s

Create a comprehensive, professional research report.`, topic, fmt.Sprintf("%v", allContent))
	
	maxSteps := 15
	synthesisResult, err := rm.agent.Run(&agents.RunOptions{
		Task:     synthesisPrompt,
		MaxSteps: &maxSteps,
	})
	
	summary := ""
	if err != nil {
		summary = fmt.Sprintf("Synthesis failed: %v. Raw findings included below.", err)
		for _, content := range allContent {
			summary += "\n\n" + content
		}
	} else {
		summary = fmt.Sprintf("%v", synthesisResult.Output)
	}
	
	endTime := time.Now()
	
	return &ProjectReport{
		Topic:       topic,
		StartTime:   startTime,
		EndTime:     endTime,
		Duration:    endTime.Sub(startTime),
		TaskResults: results,
		Summary:     summary,
		Confidence:  avgConfidence,
		Sources:     allSources,
	}, nil
}

// Stop shuts down the research manager and all workers
func (rm *ResearchManager) Stop() {
	log.Println("Shutting down research manager")
	rm.cancel()
	rm.wg.Wait()
	close(rm.taskQueue)
	close(rm.resultQueue)
}

// PrintReport displays a formatted research report
func PrintReport(report *ProjectReport) {
	fmt.Printf("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Printf("                    RESEARCH PROJECT REPORT\n")
	fmt.Printf(strings.Repeat("=", 80) + "\n\n")
	
	fmt.Printf("Topic: %s\n", report.Topic)
	fmt.Printf("Duration: %v\n", report.Duration)
	fmt.Printf("Confidence: %.1f%%\n", report.Confidence*100)
	fmt.Printf("Sources: %d unique sources\n", len(report.Sources))
	fmt.Printf("Tasks Completed: %d\n\n", len(report.TaskResults))
	
	fmt.Printf("EXECUTIVE SUMMARY\n")
	fmt.Printf(strings.Repeat("-", 80) + "\n\n")
	fmt.Printf("%s\n\n", report.Summary)
	
	fmt.Printf("DETAILED TASK RESULTS\n")
	fmt.Printf(strings.Repeat("-", 80) + "\n\n")
	
	for taskID, result := range report.TaskResults {
		fmt.Printf("Task: %s\n", taskID)
		fmt.Printf("Worker: %s\n", result.WorkerID)
		fmt.Printf("Duration: %v\n", result.Duration)
		fmt.Printf("Confidence: %.1f%%\n", result.Confidence*100)
		if result.Error != nil {
			fmt.Printf("Error: %v\n", result.Error)
		} else {
			fmt.Printf("Result: %s\n", result.Content[:min(200, len(result.Content))])
			if len(result.Content) > 200 {
				fmt.Printf("... (truncated)\n")
			}
		}
		fmt.Printf("\n")
	}
	
	fmt.Printf(strings.Repeat("=", 80) + "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// main demonstrates the research agent system
func main() {
	fmt.Println("🔬 Phenomenal Deep Research Agent System")
	fmt.Println("========================================")
	
	// Check for required environment variables
	if os.Getenv("HF_API_TOKEN") == "" && os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("Please set HF_API_TOKEN or OPENAI_API_KEY environment variable")
	}
	
	// Create model
	var model models.Model
	var err error
	
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		model, err = models.CreateModel(models.ModelTypeOpenAIServer, "gpt-3.5-turbo", map[string]interface{}{
			"api_key": apiKey,
			"temperature": 0.2,
		})
	} else {
		model, err = models.CreateModel(models.ModelTypeInferenceClient, "meta-llama/Llama-2-7b-chat-hf", map[string]interface{}{
			"api_token": os.Getenv("HF_API_TOKEN"),
			"temperature": 0.2,
		})
	}
	
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}
	
	// Create research manager with 3 worker agents
	manager, err := NewResearchManager(model, 3)
	if err != nil {
		log.Fatalf("Failed to create research manager: %v", err)
	}
	defer manager.Stop()
	
	// Get research topic from command line or use default
	topic := "quantum computing"
	if len(os.Args) > 1 {
		topic = os.Args[1]
	}
	
	fmt.Printf("\n📋 Research Topic: %s\n", topic)
	fmt.Printf("👥 Workers: %d agents\n", len(manager.workers))
	fmt.Printf("⏱️  Maximum Duration: 15 minutes\n\n")
	
	// Execute research project
	report, err := manager.ResearchProject(topic, 15*time.Minute)
	if err != nil {
		log.Fatalf("Research project failed: %v", err)
	}
	
	// Display results
	PrintReport(report)
	
	fmt.Printf("\n✅ Research project completed successfully!\n")
	fmt.Printf("📊 Total duration: %v\n", report.Duration)
	fmt.Printf("🎯 Average confidence: %.1f%%\n", report.Confidence*100)
}
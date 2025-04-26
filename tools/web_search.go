// Package tools provides tool implementations for AI agents
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rizome-dev/smolagentsgo/utils"
)

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// DuckDuckGoSearchTool is a tool for searching the web using the DuckDuckGo API
type DuckDuckGoSearchTool struct {
	name        string
	description string
	inputs      map[string]InputProperty
	outputType  string
	client      *http.Client
	initialized bool
}

// NewDuckDuckGoSearchTool creates a new DuckDuckGoSearchTool
func NewDuckDuckGoSearchTool(options ...func(*DuckDuckGoSearchTool)) (*DuckDuckGoSearchTool, error) {
	// Default tool configuration
	tool := &DuckDuckGoSearchTool{
		name:        "web_search",
		description: "Search the web for real-time information using DuckDuckGo",
		inputs: map[string]InputProperty{
			"query": {
				Type:        "string",
				Description: "The search query to look up on the web",
			},
			"num_results": {
				Type:        "integer",
				Description: "The number of results to return (default: 5)",
			},
		},
		outputType: "array",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		initialized: false,
	}

	// Apply optional configurations
	for _, option := range options {
		option(tool)
	}

	// Validate configuration
	if !utils.IsValidName(tool.name) {
		return nil, fmt.Errorf("tool name '%s' must be a valid identifier", tool.name)
	}

	return tool, nil
}

// Name returns the name of the tool
func (t *DuckDuckGoSearchTool) Name() string {
	return t.name
}

// Description returns the description of the tool
func (t *DuckDuckGoSearchTool) Description() string {
	return t.description
}

// Inputs returns the input properties of the tool
func (t *DuckDuckGoSearchTool) Inputs() map[string]InputProperty {
	return t.inputs
}

// InputProperties returns the input properties of the tool (deprecated, use Inputs instead)
func (t *DuckDuckGoSearchTool) InputProperties() map[string]InputProperty {
	return t.inputs
}

// OutputType returns the output type of the tool
func (t *DuckDuckGoSearchTool) OutputType() string {
	return t.outputType
}

// Setup initializes the tool
func (t *DuckDuckGoSearchTool) Setup() error {
	t.initialized = true
	return nil
}

// Forward implements the actual search functionality
func (t *DuckDuckGoSearchTool) Forward(args map[string]interface{}) (interface{}, error) {
	// Extract query
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query must be a non-empty string")
	}

	// Extract number of results (default to 5)
	numResults := 5
	if val, ok := args["num_results"]; ok {
		if num, ok := val.(float64); ok {
			numResults = int(num)
		} else if num, ok := val.(int); ok {
			numResults = num
		}
	}

	// Limit number of results
	if numResults < 1 {
		numResults = 1
	} else if numResults > 10 {
		numResults = 10
	}

	// Use context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Perform search
	return t.search(ctx, query, numResults)
}

// Call executes the tool with the given arguments
func (t *DuckDuckGoSearchTool) Call(args map[string]interface{}, sanitizeInputsOutputs bool) (interface{}, error) {
	if !t.initialized {
		if err := t.Setup(); err != nil {
			return nil, fmt.Errorf("failed to setup tool: %w", err)
		}
	}

	// Validate inputs if sanitization is enabled
	if sanitizeInputsOutputs {
		// Simple validation (could be more comprehensive)
		query, ok := args["query"]
		if !ok {
			return nil, fmt.Errorf("missing required input 'query'")
		}
		if queryStr, ok := query.(string); !ok || queryStr == "" {
			return nil, fmt.Errorf("query must be a non-empty string")
		}
	}

	// Call the forward function
	result, err := t.Forward(args)
	if err != nil {
		return nil, err
	}

	// Sanitize output if needed
	if sanitizeInputsOutputs {
		// Ensure we're returning an array
		if result == nil {
			return []SearchResult{}, nil
		}
	}

	return result, nil
}

// search performs the actual search using DuckDuckGo API
func (t *DuckDuckGoSearchTool) search(ctx context.Context, query string, numResults int) ([]SearchResult, error) {
	// Escape query for URL
	encodedQuery := url.QueryEscape(query)

	// Create request to DuckDuckGo API
	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&pretty=0", encodedQuery)
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "smolagentsgo/1.0")

	// Execute request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	var ddgResponse struct {
		AbstractText  string `json:"AbstractText"`
		AbstractURL   string `json:"AbstractURL"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}

	if err := json.Unmarshal(body, &ddgResponse); err != nil {
		return nil, fmt.Errorf("failed to parse DuckDuckGo response: %w", err)
	}

	// Process results concurrently using goroutines
	var results []SearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Process abstract
	if ddgResponse.AbstractText != "" && ddgResponse.AbstractURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := SearchResult{
				Title:   "Abstract",
				URL:     ddgResponse.AbstractURL,
				Snippet: ddgResponse.AbstractText,
			}
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}()
	}

	// Process related topics
	relatedTopicsCount := len(ddgResponse.RelatedTopics)
	if relatedTopicsCount > 0 {
		// Create a buffered channel for concurrent processing
		resultChan := make(chan SearchResult, relatedTopicsCount)

		// Maximum results to process from related topics
		maxToProcess := numResults
		if relatedTopicsCount < maxToProcess {
			maxToProcess = relatedTopicsCount
		}

		// Process each topic concurrently
		for i := 0; i < maxToProcess; i++ {
			wg.Add(1)
			go func(topic struct {
				Text     string `json:"Text"`
				FirstURL string `json:"FirstURL"`
			}) {
				defer wg.Done()

				// Skip empty results
				if topic.Text == "" || topic.FirstURL == "" {
					return
				}

				// Extract title from text
				title := topic.Text
				if idx := strings.Index(title, " - "); idx > 0 {
					title = title[:idx]
				}

				resultChan <- SearchResult{
					Title:   title,
					URL:     topic.FirstURL,
					Snippet: topic.Text,
				}
			}(ddgResponse.RelatedTopics[i])
		}

		// Wait for all goroutines to finish
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// Collect results from channel
		for result := range resultChan {
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}
	}

	// Limit results to requested number
	if len(results) > numResults {
		results = results[:numResults]
	}

	// Return empty slice if no results
	if len(results) == 0 {
		return []SearchResult{}, nil
	}

	return results, nil
}

// WithClient sets a custom HTTP client
func WithClient(client *http.Client) func(*DuckDuckGoSearchTool) {
	return func(t *DuckDuckGoSearchTool) {
		t.client = client
	}
}

// WithName sets a custom name for the tool
func WithName(name string) func(*DuckDuckGoSearchTool) {
	return func(t *DuckDuckGoSearchTool) {
		t.name = name
	}
}

// WithDescription sets a custom description for the tool
func WithDescription(description string) func(*DuckDuckGoSearchTool) {
	return func(t *DuckDuckGoSearchTool) {
		t.description = description
	}
}

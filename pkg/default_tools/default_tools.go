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

// Package default_tools provides built-in tools for smolagents.
//
// This includes core tools like GoInterpreter, WebSearch, FinalAnswer,
// and other essential tools that agents commonly use.
package default_tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/executors"
	"github.com/rizome-dev/go-smolagents/pkg/tools"
	"github.com/rizome-dev/go-smolagents/pkg/utils"
)

// BaseBuiltinPackages lists Go packages allowed in GoInterpreterTool
var BaseBuiltinPackages = []string{
	"fmt", "math", "math/rand", "strings", "strconv", "time",
	"encoding/json", "encoding/csv", "encoding/base64",
	"regexp", "sort", "unicode", "unicode/utf8",
	"crypto/md5", "crypto/sha1", "crypto/sha256",
	"path", "path/filepath", "net/url",
	"compress/gzip", "archive/zip",
	"bytes", "bufio", "io", "container/list", "container/heap",
}

// ToolMapping maps tool names to their constructor functions
var ToolMapping = map[string]func() tools.Tool{
	"go_interpreter":    func() tools.Tool { return NewGoInterpreterTool() },
	"final_answer":      func() tools.Tool { return NewFinalAnswerTool() },
	"user_input":        func() tools.Tool { return NewUserInputTool() },
	"web_search":        func() tools.Tool { return NewWebSearchTool() },
	"duckduckgo_search": func() tools.Tool { return NewDuckDuckGoSearchTool() },
	"google_search":     func() tools.Tool { return NewGoogleSearchTool() },
	"visit_webpage":     func() tools.Tool { return NewVisitWebpageTool() },
	"wikipedia_search":  func() tools.Tool { return NewWikipediaSearchTool() },
	"speech_to_text":    func() tools.Tool { return NewSpeechToTextTool() },
	"pipeline_tool":     func() tools.Tool { return NewPipelineTool() },
	"vision_browser":    func() tools.Tool { return NewVisionBrowser() },
}

// GoInterpreterTool evaluates Go code safely
type GoInterpreterTool struct {
	*tools.BaseTool
	AuthorizedPackages []string                               `json:"authorized_packages"`
	GoEvaluator        func(string, []string) (string, error) `json:"-"`
	executor           GoExecutor                             `json:"-"`
}

// GoExecutor interface for executing Go code
type GoExecutor interface {
	Execute(code string, authorizedImports []string) (interface{}, error)
	SendVariables(variables map[string]interface{}) error
	SendTools(tools map[string]tools.Tool) error
	GetState() map[string]interface{}
	Reset() error
}

// NewGoInterpreterTool creates a new Go interpreter tool
func NewGoInterpreterTool(authorizedPackages ...[]string) *GoInterpreterTool {
	var packages []string
	if len(authorizedPackages) > 0 {
		packages = authorizedPackages[0]
	} else {
		packages = make([]string, len(BaseBuiltinPackages))
		copy(packages, BaseBuiltinPackages)
	}

	inputs := map[string]*tools.ToolInput{
		"code": tools.NewToolInput(
			"string",
			fmt.Sprintf("The Go code snippet to evaluate. All variables used in this snippet must be defined in this same snippet, "+
				"else you will get an error. This code can only import the following Go packages: %v.", packages),
		),
	}

	baseTool := tools.NewBaseTool(
		"go_interpreter",
		"This is a tool that evaluates Go code. It can be used to perform calculations and data processing.",
		inputs,
		"string",
	)

	// Create real Go executor
	executor, err := executors.NewGoExecutor(map[string]interface{}{
		"authorized_packages": packages,
	})
	if err != nil {
		// Fall back to simple evaluator if executor creation fails
		git := &GoInterpreterTool{
			BaseTool:           baseTool,
			AuthorizedPackages: packages,
			GoEvaluator:        evaluateGoCode,
		}
		git.ForwardFunc = git.forward
		return git
	}

	git := &GoInterpreterTool{
		BaseTool:           baseTool,
		AuthorizedPackages: packages,
		GoEvaluator:        evaluateGoCode, // Fallback implementation
		executor:           executor,
	}

	// Set the forward function
	git.ForwardFunc = git.forward

	return git
}

// NewGoInterpreterToolWithExecutor creates a new Go interpreter tool with a custom executor
func NewGoInterpreterToolWithExecutor(executor GoExecutor, authorizedPackages ...[]string) *GoInterpreterTool {
	var packages []string
	if len(authorizedPackages) > 0 {
		packages = authorizedPackages[0]
	} else {
		packages = make([]string, len(BaseBuiltinPackages))
		copy(packages, BaseBuiltinPackages)
	}

	inputs := map[string]*tools.ToolInput{
		"code": tools.NewToolInput(
			"string",
			fmt.Sprintf("The Go code snippet to evaluate. All variables used in this snippet must be defined in this same snippet, "+
				"else you will get an error. This code can only import the following Go packages: %v.", packages),
		),
	}

	baseTool := tools.NewBaseTool(
		"go_interpreter",
		"This is a tool that evaluates Go code. It can be used to perform calculations and data processing.",
		inputs,
		"string",
	)

	git := &GoInterpreterTool{
		BaseTool:           baseTool,
		AuthorizedPackages: packages,
		GoEvaluator:        evaluateGoCode, // Fallback implementation
		executor:           executor,
	}

	// Set the forward function
	git.ForwardFunc = git.forward

	return git
}

// forward implements the tool logic
func (git *GoInterpreterTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("code parameter is required")
	}

	code, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("code must be a string")
	}

	// Use the real executor if available
	if git.executor != nil {
		result, err := git.executor.Execute(code, git.AuthorizedPackages)
		if err != nil {
			return fmt.Sprintf("Error: %s", err.Error()), nil
		}
		return result, nil
	}

	// Fall back to the simple evaluator
	result, err := git.GoEvaluator(code, git.AuthorizedPackages)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error()), nil
	}

	return result, nil
}

// SetExecutor sets the Go executor for this tool
func (git *GoInterpreterTool) SetExecutor(executor GoExecutor) {
	git.executor = executor
}

// GetExecutor returns the Go executor for this tool
func (git *GoInterpreterTool) GetExecutor() GoExecutor {
	return git.executor
}

// evaluateGoCode is a placeholder implementation for Go code execution
func evaluateGoCode(code string, authorizedPackages []string) (string, error) {
	// This is a placeholder implementation
	// In a real implementation, this would use the GoExecutor
	// 1. Set up a sandboxed Go environment
	// 2. Validate imports against authorized package list
	// 3. Execute the code safely
	// 4. Capture stdout and return value
	// 5. Handle errors appropriately

	// For now, return a placeholder
	return fmt.Sprintf("Go code execution (placeholder):\n%s\n\nAuthorized packages: %v",
		code, authorizedPackages), nil
}

// FinalAnswerTool provides the final answer to a problem
type FinalAnswerTool struct {
	*tools.BaseTool
}

// NewFinalAnswerTool creates a new final answer tool
func NewFinalAnswerTool() *FinalAnswerTool {
	inputs := map[string]*tools.ToolInput{
		"answer": tools.NewToolInput("any", "The final answer to the problem"),
	}

	baseTool := tools.NewBaseTool(
		"final_answer",
		"Provides a final answer to the given problem.",
		inputs,
		"any",
	)

	fat := &FinalAnswerTool{
		BaseTool: baseTool,
	}

	// Set the forward function
	fat.ForwardFunc = fat.forward

	return fat
}

// forward implements the tool logic
func (fat *FinalAnswerTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, nil
	}

	// Pass through the answer as-is
	return args[0], nil
}

// UserInputTool asks for user input on specific questions
type UserInputTool struct {
	*tools.BaseTool
}

// NewUserInputTool creates a new user input tool
func NewUserInputTool() *UserInputTool {
	inputs := map[string]*tools.ToolInput{
		"question": tools.NewToolInput("string", "The question to ask the user"),
	}

	baseTool := tools.NewBaseTool(
		"user_input",
		"Asks for user's input on a specific question",
		inputs,
		"string",
	)

	uit := &UserInputTool{
		BaseTool: baseTool,
	}

	// Set the forward function
	uit.ForwardFunc = uit.forward

	return uit
}

// forward implements the tool logic
func (uit *UserInputTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("question parameter is required")
	}

	question, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("question must be a string")
	}

	// Ask user for input
	fmt.Printf("%s => Type your answer here: ", question)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read user input: %w", err)
	}

	return strings.TrimSpace(response), nil
}

// WebSearchTool performs web searches using multiple engines
type WebSearchTool struct {
	*tools.BaseTool
	MaxResults int    `json:"max_results"`
	Engine     string `json:"engine"`
}

// NewWebSearchTool creates a new web search tool
func NewWebSearchTool(options ...map[string]interface{}) *WebSearchTool {
	inputs := map[string]*tools.ToolInput{
		"query": tools.NewToolInput("string", "The search query to execute"),
	}

	baseTool := tools.NewBaseTool(
		"web_search",
		"Performs web search and returns formatted results. Supports multiple search engines.",
		inputs,
		"string",
	)

	wst := &WebSearchTool{
		BaseTool:   baseTool,
		MaxResults: 10,           // Default
		Engine:     "duckduckgo", // Default engine
	}

	if len(options) > 0 {
		if maxResults, ok := options[0]["max_results"].(int); ok {
			wst.MaxResults = maxResults
		}
		if engine, ok := options[0]["engine"].(string); ok {
			wst.Engine = engine
		}
	}

	// Set the forward function
	wst.ForwardFunc = wst.forward

	return wst
}

// forward implements the tool logic
func (wst *WebSearchTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("query parameter is required")
	}

	query, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("query must be a string")
	}

	switch wst.Engine {
	case "duckduckgo":
		return wst.searchDuckDuckGo(query)
	default:
		return nil, fmt.Errorf("unsupported search engine: %s", wst.Engine)
	}
}

// searchDuckDuckGo performs a DuckDuckGo search using HTML scraping
func (wst *WebSearchTool) searchDuckDuckGo(query string) (string, error) {
	// Use DuckDuckGo HTML search (more reliable than JSON API for web results)
	encodedQuery := url.QueryEscape(query)
	searchURL := fmt.Sprintf("https://duckduckgo.com/html/?q=%s", encodedQuery)

	// Create HTTP client with user agent
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}

	// Set user agent to avoid blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; smolagents/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search request failed with status: %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Parse HTML results (simplified extraction)
	htmlContent := string(body)

	// Extract search results using regex (basic approach)
	results := wst.extractSearchResults(htmlContent, query)

	if len(results) == 0 {
		return fmt.Sprintf("Search results for '%s':\n\nNo results found or search blocked. Consider using different search terms.", query), nil
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Search results for '%s':\n\n", query))

	for i, result := range results {
		if i >= wst.MaxResults {
			break
		}
		output.WriteString(fmt.Sprintf("%d. %s\n\n", i+1, result))
	}

	return output.String(), nil
}

// extractSearchResults extracts search results from DuckDuckGo HTML
func (wst *WebSearchTool) extractSearchResults(htmlContent, query string) []string {
	var results []string

	// Look for result links and titles
	// DuckDuckGo HTML structure: <a class="result__a" href="...">title</a>
	linkPattern := regexp.MustCompile(`<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]*)"[^>]*>([^<]*)</a>`)
	matches := linkPattern.FindAllStringSubmatch(htmlContent, wst.MaxResults)

	for _, match := range matches {
		if len(match) >= 3 {
			url := strings.TrimSpace(match[1])
			title := strings.TrimSpace(match[2])
			if title != "" && url != "" {
				// Clean up the title
				title = html.UnescapeString(title)
				result := fmt.Sprintf("**%s**\n%s", title, url)
				results = append(results, result)
			}
		}
	}

	// If no results found with that pattern, try a more generic approach
	if len(results) == 0 {
		// Return a placeholder indicating search was attempted
		return []string{fmt.Sprintf("Search attempted for '%s' but no results could be extracted. This may be due to rate limiting or changes in the search provider's HTML structure.", query)}
	}

	return results
}

// VisitWebpageTool visits a webpage and converts it to markdown
type VisitWebpageTool struct {
	*tools.BaseTool
	MaxOutputLength int `json:"max_output_length"`
	Timeout         int `json:"timeout"`
}

// NewVisitWebpageTool creates a new webpage visitor tool
func NewVisitWebpageTool(options ...map[string]interface{}) *VisitWebpageTool {
	inputs := map[string]*tools.ToolInput{
		"url": tools.NewToolInput("string", "The URL of the webpage to visit"),
	}

	baseTool := tools.NewBaseTool(
		"visit_webpage",
		"Visits a webpage and converts its content to markdown format.",
		inputs,
		"string",
	)

	vwt := &VisitWebpageTool{
		BaseTool:        baseTool,
		MaxOutputLength: 40000, // Default 40k characters
		Timeout:         20,    // Default 20 seconds
	}

	if len(options) > 0 {
		if maxLength, ok := options[0]["max_output_length"].(int); ok {
			vwt.MaxOutputLength = maxLength
		}
		if timeout, ok := options[0]["timeout"].(int); ok {
			vwt.Timeout = timeout
		}
	}

	// Set the forward function
	vwt.ForwardFunc = vwt.forward

	return vwt
}

// forward implements the tool logic
func (vwt *VisitWebpageTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("url parameter is required")
	}

	urlStr, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("url must be a string")
	}

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
		urlStr = parsedURL.String()
	}

	// Make HTTP request
	client := &http.Client{Timeout: time.Duration(vwt.Timeout) * time.Second}
	resp, err := client.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch webpage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("webpage request failed with status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read webpage content: %w", err)
	}

	// Convert HTML to markdown (simplified implementation)
	markdown := vwt.htmlToMarkdown(string(body))

	// Truncate if necessary
	if len(markdown) > vwt.MaxOutputLength {
		markdown = utils.TruncateContent(markdown, vwt.MaxOutputLength)
	}

	return markdown, nil
}

// htmlToMarkdown converts HTML to markdown (simplified implementation)
func (vwt *VisitWebpageTool) htmlToMarkdown(htmlContent string) string {
	// This is a very simplified HTML to Markdown converter
	// In a real implementation, you'd use a proper HTML parser and markdown converter

	// Remove script and style tags
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	htmlContent = scriptRegex.ReplaceAllString(htmlContent, "")

	styleRegex := regexp.MustCompile(`(?i)<style[^>]*>.*?</style>`)
	htmlContent = styleRegex.ReplaceAllString(htmlContent, "")

	// Convert common HTML tags to markdown
	conversions := map[string]string{
		`<h1[^>]*>(.*?)</h1>`:         "# $1",
		`<h2[^>]*>(.*?)</h2>`:         "## $1",
		`<h3[^>]*>(.*?)</h3>`:         "### $1",
		`<h4[^>]*>(.*?)</h4>`:         "#### $1",
		`<h5[^>]*>(.*?)</h5>`:         "##### $1",
		`<h6[^>]*>(.*?)</h6>`:         "###### $1",
		`<strong[^>]*>(.*?)</strong>`: "**$1**",
		`<b[^>]*>(.*?)</b>`:           "**$1**",
		`<em[^>]*>(.*?)</em>`:         "*$1*",
		`<i[^>]*>(.*?)</i>`:           "*$1*",
		`<p[^>]*>(.*?)</p>`:           "$1\n\n",
		`<br[^>]*>`:                   "\n",
		`<hr[^>]*>`:                   "\n---\n",
	}

	for pattern, replacement := range conversions {
		regex := regexp.MustCompile(`(?i)` + pattern)
		htmlContent = regex.ReplaceAllString(htmlContent, replacement)
	}

	// Handle links
	linkRegex := regexp.MustCompile(`(?i)<a[^>]*href=["']([^"']*)["'][^>]*>(.*?)</a>`)
	htmlContent = linkRegex.ReplaceAllString(htmlContent, "[$2]($1)")

	// Remove remaining HTML tags
	tagRegex := regexp.MustCompile(`<[^>]*>`)
	htmlContent = tagRegex.ReplaceAllString(htmlContent, "")

	// Decode HTML entities
	htmlContent = html.UnescapeString(htmlContent)

	// Clean up whitespace
	lines := strings.Split(htmlContent, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// WikipediaSearchTool searches Wikipedia and returns formatted content
type WikipediaSearchTool struct {
	*tools.BaseTool
	UserAgent     string `json:"user_agent"`
	Language      string `json:"language"`
	ContentType   string `json:"content_type"`
	ExtractFormat string `json:"extract_format"`
}

// NewWikipediaSearchTool creates a new Wikipedia search tool
func NewWikipediaSearchTool(options ...map[string]interface{}) *WikipediaSearchTool {
	inputs := map[string]*tools.ToolInput{
		"query": tools.NewToolInput("string", "The search query for Wikipedia"),
	}

	baseTool := tools.NewBaseTool(
		"wikipedia_search",
		"Searches Wikipedia and returns formatted content for the query.",
		inputs,
		"string",
	)

	wst := &WikipediaSearchTool{
		BaseTool:      baseTool,
		UserAgent:     "smolagents/1.0 (https://github.com/huggingface/smolagents)",
		Language:      "en",
		ContentType:   "text",
		ExtractFormat: "wiki",
	}

	if len(options) > 0 {
		if userAgent, ok := options[0]["user_agent"].(string); ok {
			wst.UserAgent = userAgent
		}
		if language, ok := options[0]["language"].(string); ok {
			wst.Language = language
		}
		if contentType, ok := options[0]["content_type"].(string); ok {
			wst.ContentType = contentType
		}
		if extractFormat, ok := options[0]["extract_format"].(string); ok {
			wst.ExtractFormat = extractFormat
		}
	}

	// Set the forward function
	wst.ForwardFunc = wst.forward

	return wst
}

// forward implements the tool logic
func (wst *WikipediaSearchTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("query parameter is required")
	}

	query, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("query must be a string")
	}

	return wst.searchWikipedia(query)
}

// searchWikipedia performs a Wikipedia search
func (wst *WikipediaSearchTool) searchWikipedia(query string) (string, error) {
	// Search for the article
	searchURL := fmt.Sprintf("https://%s.wikipedia.org/api/rest_v1/page/summary/%s",
		wst.Language, url.QueryEscape(query))

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", wst.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Wikipedia request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Sprintf("No Wikipedia article found for query: %s", query), nil
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Wikipedia request failed with status: %d", resp.StatusCode)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Wikipedia response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse Wikipedia response: %w", err)
	}

	// Extract information
	title, _ := result["title"].(string)
	extract, _ := result["extract"].(string)
	pageURL, _ := result["content_urls"].(map[string]interface{})

	var urlStr string
	if pageURL != nil {
		if desktop, ok := pageURL["desktop"].(map[string]interface{}); ok {
			urlStr, _ = desktop["page"].(string)
		}
	}

	// Format result
	output := fmt.Sprintf("# %s\n\n%s", title, extract)
	if urlStr != "" {
		output += fmt.Sprintf("\n\nSource: %s", urlStr)
	}

	return output, nil
}

// GetToolByName returns a tool instance by name
func GetToolByName(name string) (tools.Tool, error) {
	constructor, exists := ToolMapping[name]
	if !exists {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	return constructor(), nil
}

// DuckDuckGoSearchTool performs web searches using DuckDuckGo
type DuckDuckGoSearchTool struct {
	*tools.BaseTool
	MaxResults int    `json:"max_results"`
	UserAgent  string `json:"user_agent"`
}

// NewDuckDuckGoSearchTool creates a new DuckDuckGo search tool
func NewDuckDuckGoSearchTool() *DuckDuckGoSearchTool {
	inputs := map[string]*tools.ToolInput{
		"query": {
			Type:        "string",
			Description: "Search query",
			Nullable:    false,
		},
		"max_results": {
			Type:        "integer",
			Description: "Maximum number of results to return (default: 5)",
			Nullable:    true,
		},
	}

	tool := &DuckDuckGoSearchTool{
		BaseTool:   tools.NewBaseTool("duckduckgo_search", "Search the web using DuckDuckGo", inputs, "string"),
		MaxResults: 5,
		UserAgent:  "Mozilla/5.0 (compatible; SmolagentsBot/1.0)",
	}

	tool.ForwardFunc = tool.forward
	return tool
}

func (ddg *DuckDuckGoSearchTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("query parameter is required")
	}

	query, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("query must be a string")
	}

	maxResults := ddg.MaxResults
	if len(args) > 1 {
		if max, ok := args[1].(int); ok && max > 0 {
			maxResults = max
		}
	}

	return ddg.searchDuckDuckGo(query, maxResults)
}

// makeRequest creates and executes the HTTP request to DuckDuckGo
func (ddg *DuckDuckGoSearchTool) makeRequest(query string) ([]byte, error) {
	searchURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1",
		url.QueryEscape(query))

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", ddg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DuckDuckGo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DuckDuckGo request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read DuckDuckGo response: %w", err)
	}

	return body, nil
}

// parseResponse parses the JSON response from DuckDuckGo
func (ddg *DuckDuckGoSearchTool) parseResponse(body []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse DuckDuckGo response: %w", err)
	}
	return result, nil
}

// formatAbstract adds the abstract/summary to the output if present
func (ddg *DuckDuckGoSearchTool) formatAbstract(result map[string]interface{}, output *strings.Builder) {
	if abstract, ok := result["Abstract"].(string); ok && abstract != "" {
		output.WriteString(fmt.Sprintf("Summary: %s\n\n", abstract))
	}
}

// formatRelatedTopics adds related topics to the output
func (ddg *DuckDuckGoSearchTool) formatRelatedTopics(result map[string]interface{}, output *strings.Builder, maxResults int) {
	relatedTopics, ok := result["RelatedTopics"].([]interface{})
	if !ok || len(relatedTopics) == 0 {
		return
	}

	output.WriteString("Related topics:\n")
	count := 0
	for _, topic := range relatedTopics {
		if count >= maxResults {
			break
		}
		
		topicMap, ok := topic.(map[string]interface{})
		if !ok {
			continue
		}
		
		text, textOk := topicMap["Text"].(string)
		firstURL, urlOk := topicMap["FirstURL"].(string)
		
		if textOk && text != "" && urlOk {
			output.WriteString(fmt.Sprintf("- %s\n  URL: %s\n", text, firstURL))
			count++
		}
	}
}

// searchDuckDuckGo performs the search and returns formatted results
func (ddg *DuckDuckGoSearchTool) searchDuckDuckGo(query string, maxResults int) (string, error) {
	// Make the HTTP request
	body, err := ddg.makeRequest(query)
	if err != nil {
		return "", err
	}

	// Parse the response
	result, err := ddg.parseResponse(body)
	if err != nil {
		return "", err
	}

	// Format results
	var output strings.Builder
	output.WriteString(fmt.Sprintf("DuckDuckGo search results for: %s\n\n", query))

	// Add abstract if present
	ddg.formatAbstract(result, &output)

	// Add related topics
	ddg.formatRelatedTopics(result, &output, maxResults)

	// Check if we have any results
	if output.Len() == len(fmt.Sprintf("DuckDuckGo search results for: %s\n\n", query)) {
		return fmt.Sprintf("No results found for query: %s", query), nil
	}

	return output.String(), nil
}

// GoogleSearchTool performs web searches using Google Search API
type GoogleSearchTool struct {
	*tools.BaseTool
	APIKey       string `json:"api_key"`
	SearchEngine string `json:"search_engine"` // "serp" or "serper"
	MaxResults   int    `json:"max_results"`
}

// NewGoogleSearchTool creates a new Google search tool
func NewGoogleSearchTool() *GoogleSearchTool {
	inputs := map[string]*tools.ToolInput{
		"query": {
			Type:        "string",
			Description: "Search query",
			Nullable:    false,
		},
		"max_results": {
			Type:        "integer",
			Description: "Maximum number of results to return (default: 5)",
			Nullable:    true,
		},
	}

	apiKey := os.Getenv("SERP_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("SERPER_API_KEY")
	}

	tool := &GoogleSearchTool{
		BaseTool:     tools.NewBaseTool("google_search", "Search Google using SerpAPI or Serper", inputs, "string"),
		APIKey:       apiKey,
		SearchEngine: "serp", // Default to SerpAPI
		MaxResults:   5,
	}

	tool.ForwardFunc = tool.forward
	return tool
}

func (gs *GoogleSearchTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("query parameter is required")
	}

	query, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("query must be a string")
	}

	maxResults := gs.MaxResults
	if len(args) > 1 {
		if max, ok := args[1].(int); ok && max > 0 {
			maxResults = max
		}
	}

	if gs.APIKey == "" {
		return "", fmt.Errorf("Google Search API key not configured. Set SERP_API_KEY or SERPER_API_KEY environment variable")
	}

	return gs.searchGoogle(query, maxResults)
}

func (gs *GoogleSearchTool) searchGoogle(query string, maxResults int) (string, error) {
	var searchURL string
	var headers map[string]string

	switch gs.SearchEngine {
	case "serp":
		searchURL = fmt.Sprintf("https://serpapi.com/search?q=%s&api_key=%s&num=%d",
			url.QueryEscape(query), gs.APIKey, maxResults)
		headers = map[string]string{
			"User-Agent": "Mozilla/5.0 (compatible; SmolagentsBot/1.0)",
		}
	case "serper":
		searchURL = "https://google.serper.dev/search"
		headers = map[string]string{
			"X-API-KEY":    gs.APIKey,
			"Content-Type": "application/json",
		}
	default:
		return "", fmt.Errorf("unsupported search engine: %s", gs.SearchEngine)
	}

	client := &http.Client{Timeout: 15 * time.Second}

	var req *http.Request
	var err error

	if gs.SearchEngine == "serper" {
		requestBody := map[string]interface{}{
			"q":   query,
			"num": maxResults,
		}
		jsonBody, _ := json.Marshal(requestBody)
		req, err = http.NewRequest("POST", searchURL, bytes.NewBuffer(jsonBody))
	} else {
		req, err = http.NewRequest("GET", searchURL, nil)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Google search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Google search request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Google search response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse Google search response: %w", err)
	}

	return gs.formatGoogleResults(query, result)
}

func (gs *GoogleSearchTool) formatGoogleResults(query string, result map[string]interface{}) (string, error) {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Google search results for: %s\n\n", query))

	// Handle different response formats
	var organicResults []interface{}

	if results, ok := result["organic_results"].([]interface{}); ok {
		organicResults = results // SerpAPI format
	} else if results, ok := result["organic"].([]interface{}); ok {
		organicResults = results // Serper format
	}

	if len(organicResults) == 0 {
		return fmt.Sprintf("No search results found for: %s", query), nil
	}

	for i, item := range organicResults {
		if i >= gs.MaxResults {
			break
		}

		if itemMap, ok := item.(map[string]interface{}); ok {
			title, _ := itemMap["title"].(string)
			link, _ := itemMap["link"].(string)
			snippet, _ := itemMap["snippet"].(string)

			if title != "" && link != "" {
				output.WriteString(fmt.Sprintf("%d. %s\n", i+1, title))
				output.WriteString(fmt.Sprintf("   URL: %s\n", link))
				if snippet != "" {
					output.WriteString(fmt.Sprintf("   %s\n", snippet))
				}
				output.WriteString("\n")
			}
		}
	}

	return output.String(), nil
}

// SpeechToTextTool converts audio to text using Whisper
type SpeechToTextTool struct {
	*tools.BaseTool
	Model    string `json:"model"`
	Language string `json:"language"`
	APIKey   string `json:"api_key"`
	APIBase  string `json:"api_base"`
}

// NewSpeechToTextTool creates a new speech-to-text tool
func NewSpeechToTextTool() *SpeechToTextTool {
	inputs := map[string]*tools.ToolInput{
		"audio_path": {
			Type:        "string",
			Description: "Path to audio file or URL",
			Nullable:    false,
		},
		"language": {
			Type:        "string",
			Description: "Language code (optional, auto-detect if not provided)",
			Nullable:    true,
		},
	}

	tool := &SpeechToTextTool{
		BaseTool: tools.NewBaseTool("speech_to_text", "Convert audio to text using Whisper", inputs, "string"),
		Model:    "whisper-1",
		Language: "auto",
		APIKey:   os.Getenv("OPENAI_API_KEY"),
		APIBase:  "https://api.openai.com/v1",
	}

	tool.ForwardFunc = tool.forward
	return tool
}

func (stt *SpeechToTextTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("audio_path parameter is required")
	}

	audioPath, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("audio_path must be a string")
	}

	language := stt.Language
	if len(args) > 1 {
		if lang, ok := args[1].(string); ok && lang != "" {
			language = lang
		}
	}

	if stt.APIKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured. Set OPENAI_API_KEY environment variable")
	}

	return stt.transcribeAudio(audioPath, language)
}

func (stt *SpeechToTextTool) transcribeAudio(audioPath, language string) (string, error) {
	// This is a placeholder implementation
	// In a real implementation, this would:
	// 1. Load the audio file
	// 2. Send it to OpenAI Whisper API or run local Whisper model
	// 3. Return the transcribed text

	return fmt.Sprintf("Speech-to-text transcription placeholder for audio: %s (language: %s)\n\nThis would contain the transcribed text from the audio file.", audioPath, language), nil
}

// PipelineTool executes HuggingFace transformers pipelines
type PipelineTool struct {
	*tools.BaseTool
	PipelineType string `json:"pipeline_type"`
	ModelName    string `json:"model_name"`
	APIKey       string `json:"api_key"`
}

// NewPipelineTool creates a new pipeline tool
func NewPipelineTool() *PipelineTool {
	inputs := map[string]*tools.ToolInput{
		"text": {
			Type:        "string",
			Description: "Input text to process",
			Nullable:    false,
		},
		"pipeline_type": {
			Type:        "string",
			Description: "Type of pipeline (text-generation, sentiment-analysis, etc.)",
			Nullable:    true,
		},
		"model": {
			Type:        "string",
			Description: "Model name to use",
			Nullable:    true,
		},
	}

	tool := &PipelineTool{
		BaseTool:     tools.NewBaseTool("pipeline_tool", "Execute HuggingFace transformers pipelines", inputs, "string"),
		PipelineType: "text-generation",
		ModelName:    "gpt2",
		APIKey:       os.Getenv("HF_TOKEN"),
	}

	tool.ForwardFunc = tool.forward
	return tool
}

func (pt *PipelineTool) forward(args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("text parameter is required")
	}

	text, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("text must be a string")
	}

	pipelineType := pt.PipelineType
	if len(args) > 1 {
		if pType, ok := args[1].(string); ok && pType != "" {
			pipelineType = pType
		}
	}

	modelName := pt.ModelName
	if len(args) > 2 {
		if model, ok := args[2].(string); ok && model != "" {
			modelName = model
		}
	}

	return pt.executePipeline(text, pipelineType, modelName)
}

func (pt *PipelineTool) executePipeline(text, pipelineType, modelName string) (string, error) {
	// This is a placeholder implementation
	// In a real implementation, this would:
	// 1. Use HuggingFace Inference API or local transformers
	// 2. Execute the specified pipeline
	// 3. Return the processed results

	return fmt.Sprintf("Pipeline execution placeholder:\nType: %s\nModel: %s\nInput: %s\n\nThis would contain the actual pipeline results.", pipelineType, modelName, text), nil
}

// NewVisionBrowser creates a new vision browser tool (wrapper)
func NewVisionBrowser() tools.Tool {
	// This imports from the tools package to avoid circular imports
	// In a real implementation, this would be properly structured
	inputs := map[string]*tools.ToolInput{
		"action": {
			Type:        "string",
			Description: "Browser action to perform",
			Nullable:    false,
		},
		"url": {
			Type:        "string",
			Description: "URL to navigate to",
			Nullable:    true,
		},
	}

	tool := tools.NewBaseTool("vision_browser", "Automated web browser with vision capabilities", inputs, "string")

	tool.ForwardFunc = func(args ...interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("action parameter is required")
		}

		action, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("action must be a string")
		}

		// Placeholder implementation
		return fmt.Sprintf("Vision browser action '%s' executed successfully. This is a placeholder implementation.", action), nil
	}

	return tool
}

// ListAvailableTools returns a list of all available tool names
func ListAvailableTools() []string {
	names := make([]string, 0, len(ToolMapping))
	for name := range ToolMapping {
		names = append(names, name)
	}
	return names
}

// RegisterTool registers a new tool in the global mapping
func RegisterTool(name string, constructor func() tools.Tool) {
	ToolMapping[name] = constructor
}

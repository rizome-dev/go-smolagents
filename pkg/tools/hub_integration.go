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

// Package tools - HuggingFace Hub integration for loading tools
package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// HubClient provides access to HuggingFace Hub for tool loading
type HubClient struct {
	BaseURL    string            `json:"base_url"`
	Token      string            `json:"-"` // API token (not serialized)
	Headers    map[string]string `json:"headers"`
	UserAgent  string            `json:"user_agent"`
	Timeout    time.Duration     `json:"timeout"`
	httpClient *http.Client
}

// NewHubClient creates a new HuggingFace Hub client
func NewHubClient(token string, options map[string]interface{}) *HubClient {
	client := &HubClient{
		BaseURL:   "https://huggingface.co",
		Token:     token,
		Headers:   make(map[string]string),
		UserAgent: "smolagents-go/1.18.0",
		Timeout:   30 * time.Second,
	}

	if options != nil {
		if baseURL, ok := options["base_url"].(string); ok {
			client.BaseURL = baseURL
		}
		if headers, ok := options["headers"].(map[string]string); ok {
			client.Headers = headers
		}
		if userAgent, ok := options["user_agent"].(string); ok {
			client.UserAgent = userAgent
		}
		if timeout, ok := options["timeout"].(time.Duration); ok {
			client.Timeout = timeout
		}
	}

	client.httpClient = &http.Client{
		Timeout: client.Timeout,
	}

	return client
}

// ToolMetadata represents metadata about a tool from the Hub
type ToolMetadata struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	Version     string                 `json:"version"`
	Inputs      map[string]*ToolInput  `json:"inputs"`
	OutputType  string                 `json:"output_type"`
	Category    string                 `json:"category"`
	Tags        []string               `json:"tags"`
	License     string                 `json:"license"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
	Downloads   int                    `json:"downloads"`
	Likes       int                    `json:"likes"`
	Config      map[string]interface{} `json:"config"`
}

// HubTool represents a tool loaded from the HuggingFace Hub
type HubTool struct {
	*BaseTool
	Metadata   *ToolMetadata `json:"metadata"`
	HubID      string        `json:"hub_id"`
	Revision   string        `json:"revision"`
	LocalPath  string        `json:"local_path"`
	client     *HubClient
	executable string // Path to executable for the tool
}

// LoadToolFromHub loads a tool from the HuggingFace Hub
func LoadToolFromHub(hubID string, client *HubClient, options map[string]interface{}) (*HubTool, error) {
	if client == nil {
		client = NewHubClient("", nil)
	}

	// Parse hub ID (format: "namespace/tool-name" or "namespace/tool-name@revision")
	parts := strings.Split(hubID, "@")
	toolPath := parts[0]
	revision := "main"
	if len(parts) > 1 {
		revision = parts[1]
	}

	// Fetch tool metadata
	metadata, err := client.fetchToolMetadata(toolPath, revision)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tool metadata: %w", err)
	}

	// Create base tool from metadata
	baseTool := NewBaseTool(
		metadata.Name,
		metadata.Description,
		metadata.Inputs,
		metadata.OutputType,
	)

	// Create hub tool
	hubTool := &HubTool{
		BaseTool: baseTool,
		Metadata: metadata,
		HubID:    hubID,
		Revision: revision,
		client:   client,
	}

	// Download and prepare the tool
	if err := hubTool.downloadAndPrepare(options); err != nil {
		return nil, fmt.Errorf("failed to prepare tool: %w", err)
	}

	// Set the forward function
	hubTool.ForwardFunc = hubTool.forward

	return hubTool, nil
}

// fetchToolMetadata fetches tool metadata from the Hub
func (hc *HubClient) fetchToolMetadata(toolPath, revision string) (*ToolMetadata, error) {
	// Construct API URL for tool metadata
	url := fmt.Sprintf("%s/api/tools/%s/tree/%s",
		strings.TrimSuffix(hc.BaseURL, "/"), toolPath, revision)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", hc.UserAgent)
	if hc.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", hc.Token))
	}
	for key, value := range hc.Headers {
		req.Header.Set(key, value)
	}

	// Make request
	resp, err := hc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("tool not found: %s", toolPath)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// For now, create metadata from a simplified structure
	// In a real implementation, this would parse the actual Hub API response
	metadata := &ToolMetadata{
		ID:          toolPath,
		Name:        filepath.Base(toolPath),
		Description: fmt.Sprintf("Tool loaded from HuggingFace Hub: %s", toolPath),
		Author:      strings.Split(toolPath, "/")[0],
		Version:     revision,
		Category:    "hub-tool",
		Tags:        []string{"huggingface", "hub"},
		License:     "unknown",
		Config:      make(map[string]interface{}),
	}

	// Try to parse tool configuration if available
	var config map[string]interface{}
	if err := json.Unmarshal(body, &config); err == nil {
		if toolConfig, ok := config["tool"]; ok {
			if tc, ok := toolConfig.(map[string]interface{}); ok {
				if name, ok := tc["name"].(string); ok {
					metadata.Name = name
				}
				if desc, ok := tc["description"].(string); ok {
					metadata.Description = desc
				}
				if inputs, ok := tc["inputs"].(map[string]interface{}); ok {
					metadata.Inputs = parseInputsFromHub(inputs)
				}
				if outputType, ok := tc["output_type"].(string); ok {
					metadata.OutputType = outputType
				}
			}
		}
	}

	return metadata, nil
}

// parseInputsFromHub converts Hub input definitions to ToolInput format
func parseInputsFromHub(hubInputs map[string]interface{}) map[string]*ToolInput {
	inputs := make(map[string]*ToolInput)

	for name, inputDef := range hubInputs {
		if def, ok := inputDef.(map[string]interface{}); ok {
			inputType := "string"
			description := ""
			nullable := true

			if t, ok := def["type"].(string); ok {
				inputType = t
			}
			if d, ok := def["description"].(string); ok {
				description = d
			}
			if n, ok := def["nullable"].(bool); ok {
				nullable = n
			}

			inputs[name] = &ToolInput{
				Type:        inputType,
				Description: description,
				Nullable:    nullable,
			}
		}
	}

	return inputs
}

// downloadAndPrepare downloads and prepares the tool for execution
func (ht *HubTool) downloadAndPrepare(options map[string]interface{}) error {
	// For now, this is a simplified implementation
	// In a real implementation, this would:
	// 1. Download the tool files from the Hub
	// 2. Set up the execution environment
	// 3. Compile/prepare the tool for execution
	// 4. Validate the tool configuration

	// Simulate tool preparation
	ht.LocalPath = fmt.Sprintf("/tmp/hub-tools/%s", strings.ReplaceAll(ht.HubID, "/", "_"))
	ht.executable = filepath.Join(ht.LocalPath, "tool")

	return nil
}

// forward implements the tool execution logic
func (ht *HubTool) forward(args ...interface{}) (interface{}, error) {
	// Convert args to input map
	inputs := make(map[string]interface{})

	// If we have a single map argument, use it directly
	if len(args) == 1 {
		if inputMap, ok := args[0].(map[string]interface{}); ok {
			inputs = inputMap
		} else {
			// Single argument - map to first input parameter
			if len(ht.Inputs) > 0 {
				for name := range ht.Inputs {
					inputs[name] = args[0]
					break
				}
			}
		}
	} else {
		// Multiple arguments - map positionally
		i := 0
		for name := range ht.Inputs {
			if i < len(args) {
				inputs[name] = args[i]
				i++
			}
		}
	}

	// Execute the tool
	return ht.executeHubTool(inputs)
}

// executeHubTool executes the hub tool with given inputs
func (ht *HubTool) executeHubTool(inputs map[string]interface{}) (interface{}, error) {
	// This is a placeholder implementation
	// In a real implementation, this would:
	// 1. Validate inputs against the tool schema
	// 2. Execute the tool (could be Python script, binary, API call, etc.)
	// 3. Parse and return the output
	// 4. Handle errors appropriately

	inputJSON, _ := json.Marshal(inputs)

	return fmt.Sprintf("Hub tool '%s' executed with inputs: %s\n\n(This is a placeholder - real implementation would execute the actual tool from %s)",
		ht.Name, string(inputJSON), ht.HubID), nil
}

// SearchTools searches for tools on the HuggingFace Hub
func (hc *HubClient) SearchTools(query string, options map[string]interface{}) ([]*ToolMetadata, error) {
	// Construct search URL
	url := fmt.Sprintf("%s/api/tools?search=%s",
		strings.TrimSuffix(hc.BaseURL, "/"), query)

	// Add optional parameters
	if options != nil {
		if limit, ok := options["limit"].(int); ok {
			url += fmt.Sprintf("&limit=%d", limit)
		}
		if category, ok := options["category"].(string); ok {
			url += fmt.Sprintf("&category=%s", category)
		}
		if author, ok := options["author"].(string); ok {
			url += fmt.Sprintf("&author=%s", author)
		}
	}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", hc.UserAgent)
	if hc.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", hc.Token))
	}

	// Make request
	resp, err := hc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var searchResults struct {
		Tools []*ToolMetadata `json:"tools"`
		Total int             `json:"total"`
	}

	if err := json.Unmarshal(body, &searchResults); err != nil {
		// If parsing fails, return mock results for now
		return []*ToolMetadata{
			{
				ID:          fmt.Sprintf("example/%s-tool", query),
				Name:        fmt.Sprintf("%s_tool", query),
				Description: fmt.Sprintf("Example tool for %s", query),
				Author:      "example",
				Version:     "main",
				Category:    "utility",
				Tags:        []string{query, "example"},
			},
		}, nil
	}

	return searchResults.Tools, nil
}

// ListPopularTools returns a list of popular tools from the Hub
func (hc *HubClient) ListPopularTools(limit int) ([]*ToolMetadata, error) {
	return hc.SearchTools("", map[string]interface{}{
		"limit": limit,
		"sort":  "downloads",
	})
}

// GetToolDetails returns detailed information about a specific tool
func (hc *HubClient) GetToolDetails(toolPath string) (*ToolMetadata, error) {
	return hc.fetchToolMetadata(toolPath, "main")
}

// HubToolRegistry manages a collection of Hub tools
type HubToolRegistry struct {
	tools  map[string]*HubTool
	client *HubClient
}

// NewHubToolRegistry creates a new hub tool registry
func NewHubToolRegistry(client *HubClient) *HubToolRegistry {
	if client == nil {
		client = NewHubClient("", nil)
	}

	return &HubToolRegistry{
		tools:  make(map[string]*HubTool),
		client: client,
	}
}

// LoadTool loads a tool from the Hub and adds it to the registry
func (htr *HubToolRegistry) LoadTool(hubID string, options map[string]interface{}) (*HubTool, error) {
	tool, err := LoadToolFromHub(hubID, htr.client, options)
	if err != nil {
		return nil, err
	}

	htr.tools[tool.Name] = tool
	return tool, nil
}

// GetTool retrieves a tool from the registry
func (htr *HubToolRegistry) GetTool(name string) (*HubTool, bool) {
	tool, exists := htr.tools[name]
	return tool, exists
}

// ListTools returns all tools in the registry
func (htr *HubToolRegistry) ListTools() map[string]*HubTool {
	result := make(map[string]*HubTool)
	for k, v := range htr.tools {
		result[k] = v
	}
	return result
}

// RemoveTool removes a tool from the registry
func (htr *HubToolRegistry) RemoveTool(name string) bool {
	if _, exists := htr.tools[name]; exists {
		delete(htr.tools, name)
		return true
	}
	return false
}

// UpdateTool updates a tool to the latest version
func (htr *HubToolRegistry) UpdateTool(name string, options map[string]interface{}) error {
	tool, exists := htr.tools[name]
	if !exists {
		return fmt.Errorf("tool %s not found in registry", name)
	}

	// Reload the tool
	newTool, err := LoadToolFromHub(tool.HubID, htr.client, options)
	if err != nil {
		return fmt.Errorf("failed to update tool: %w", err)
	}

	htr.tools[name] = newTool
	return nil
}

// DefaultHubClient is the default Hub client instance
var DefaultHubClient *HubClient

// init initializes the default Hub client
func init() {
	DefaultHubClient = NewHubClient("", nil)
}

// LoadTool is a convenience function to load a tool using the default client
func LoadTool(hubID string, options map[string]interface{}) (*HubTool, error) {
	return LoadToolFromHub(hubID, DefaultHubClient, options)
}

// SearchTools is a convenience function to search tools using the default client
func SearchTools(query string, options map[string]interface{}) ([]*ToolMetadata, error) {
	return DefaultHubClient.SearchTools(query, options)
}

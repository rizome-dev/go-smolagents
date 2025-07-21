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

// Package models - LiteLLMModel implementation
package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/monitoring"
)

// LiteLLMModel represents a model using LiteLLM proxy
type LiteLLMModel struct {
	*BaseModel
	APIBase               string            `json:"api_base"`
	APIKey                string            `json:"-"` // API key (not serialized)
	Headers               map[string]string `json:"headers"`
	Client                interface{}       `json:"-"`       // HTTP client
	Timeout               int               `json:"timeout"` // Request timeout in seconds
	CustomRoleConversions map[string]string `json:"custom_role_conversions"`
	Organization          string            `json:"organization"`
	Project               string            `json:"project"`
}

// NewLiteLLMModel creates a new LiteLLM model
func NewLiteLLMModel(modelID string, options map[string]interface{}) *LiteLLMModel {
	base := NewBaseModel(modelID, options)

	model := &LiteLLMModel{
		BaseModel:             base,
		Headers:               make(map[string]string),
		Timeout:               30, // Default 30 seconds
		CustomRoleConversions: make(map[string]string),
	}

	if options != nil {
		if apiBase, ok := options["api_base"].(string); ok {
			model.APIBase = apiBase
		}
		if apiKey, ok := options["api_key"].(string); ok {
			model.APIKey = apiKey
		}
		if headers, ok := options["headers"].(map[string]string); ok {
			model.Headers = headers
		}
		if timeout, ok := options["timeout"].(int); ok {
			model.Timeout = timeout
		}
		if roleConversions, ok := options["custom_role_conversions"].(map[string]string); ok {
			model.CustomRoleConversions = roleConversions
		}
		if org, ok := options["organization"].(string); ok {
			model.Organization = org
		}
		if project, ok := options["project"].(string); ok {
			model.Project = project
		}
	}

	// Set default base URL if not provided
	if model.APIBase == "" {
		model.APIBase = "https://api.openai.com/v1"
	}

	return model
}

// Generate implements Model interface
func (llm *LiteLLMModel) Generate(messages []interface{}, options *GenerateOptions) (*ChatMessage, error) {
	// Convert messages to the required format with custom role conversions
	roleConversions := ToolRoleConversions
	if len(llm.CustomRoleConversions) > 0 {
		// Merge custom role conversions
		roleConversions = make(map[MessageRole]MessageRole)
		for k, v := range ToolRoleConversions {
			roleConversions[k] = v
		}
		for k, v := range llm.CustomRoleConversions {
			roleConversions[MessageRole(k)] = MessageRole(v)
		}
	}

	cleanMessages, err := GetCleanMessageList(messages, roleConversions, true, llm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
	}

	// Prepare completion parameters
	defaultParams := map[string]interface{}{
		"model":       llm.ModelID,
		"messages":    cleanMessages,
		"temperature": 0.7,
		"max_tokens":  2048,
	}

	priorityParams := map[string]interface{}{}

	// Add tools if provided
	if options != nil && len(options.ToolsToCallFrom) > 0 {
		tools := make([]map[string]interface{}, len(options.ToolsToCallFrom))
		for i, tool := range options.ToolsToCallFrom {
			tools[i] = GetToolJSONSchema(tool)
		}
		priorityParams["tools"] = tools
		priorityParams["tool_choice"] = "auto"
	}

	// Add response format if provided
	if options != nil && options.ResponseFormat != nil {
		priorityParams["response_format"] = options.ResponseFormat
	}

	// Add stop sequences if supported
	if options != nil && len(options.StopSequences) > 0 && SupportsStopParameter(llm.ModelID) {
		priorityParams["stop"] = options.StopSequences
	}

	kwargs := llm.PrepareCompletionKwargs(options, defaultParams, priorityParams)

	// Make the API call
	result, err := llm.callAPI(kwargs)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}

	// Parse the response
	return llm.parseResponse(result)
}

// GenerateStream implements Model interface
func (llm *LiteLLMModel) GenerateStream(messages []interface{}, options *GenerateOptions) (<-chan *ChatMessageStreamDelta, error) {
	// Convert messages to the required format
	roleConversions := ToolRoleConversions
	if len(llm.CustomRoleConversions) > 0 {
		// Merge custom role conversions
		roleConversions = make(map[MessageRole]MessageRole)
		for k, v := range ToolRoleConversions {
			roleConversions[k] = v
		}
		for k, v := range llm.CustomRoleConversions {
			roleConversions[MessageRole(k)] = MessageRole(v)
		}
	}

	cleanMessages, err := GetCleanMessageList(messages, roleConversions, true, llm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
	}

	// Prepare completion parameters
	defaultParams := map[string]interface{}{
		"model":       llm.ModelID,
		"messages":    cleanMessages,
		"temperature": 0.7,
		"max_tokens":  2048,
		"stream":      true,
	}

	kwargs := llm.PrepareCompletionKwargs(options, defaultParams, map[string]interface{}{})

	// Create the stream channel
	streamChan := make(chan *ChatMessageStreamDelta, 100)

	// Start streaming in a goroutine
	go func() {
		defer close(streamChan)

		// Make streaming API call
		err := llm.callStreamingAPI(kwargs, streamChan)
		if err != nil {
			// In a real implementation, you'd send an error delta or handle differently
			return
		}
	}()

	return streamChan, nil
}

// SupportsStreaming implements Model interface
func (llm *LiteLLMModel) SupportsStreaming() bool {
	return true // LiteLLM typically supports streaming
}

// ToDict implements Model interface
func (llm *LiteLLMModel) ToDict() map[string]interface{} {
	result := llm.BaseModel.ToDict()
	result["api_base"] = llm.APIBase
	result["headers"] = llm.Headers
	result["timeout"] = llm.Timeout
	result["custom_role_conversions"] = llm.CustomRoleConversions
	result["organization"] = llm.Organization
	result["project"] = llm.Project
	return result
}

// callAPI makes the actual API call to LiteLLM proxy
func (llm *LiteLLMModel) callAPI(kwargs map[string]interface{}) (map[string]interface{}, error) {
	// Prepare JSON request body
	jsonData, err := json.Marshal(kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(llm.APIBase, "/"))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if llm.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", llm.APIKey))
	}
	if llm.Organization != "" {
		req.Header.Set("OpenAI-Organization", llm.Organization)
	}
	if llm.Project != "" {
		req.Header.Set("OpenAI-Project", llm.Project)
	}
	for key, value := range llm.Headers {
		req.Header.Set(key, value)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(llm.Timeout) * time.Second,
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// callStreamingAPI makes a streaming API call to LiteLLM proxy
func (llm *LiteLLMModel) callStreamingAPI(kwargs map[string]interface{}, streamChan chan<- *ChatMessageStreamDelta) error {
	// Prepare JSON request body for streaming
	kwargs["stream"] = true
	jsonData, err := json.Marshal(kwargs)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(llm.APIBase, "/"))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for streaming
	req.Header.Set("Content-Type", "application/json")
	if llm.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", llm.APIKey))
	}
	if llm.Organization != "" {
		req.Header.Set("OpenAI-Organization", llm.Organization)
	}
	if llm.Project != "" {
		req.Header.Set("OpenAI-Project", llm.Project)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for key, value := range llm.Headers {
		req.Header.Set(key, value)
	}

	// Create HTTP client with longer timeout for streaming
	client := &http.Client{
		Timeout: time.Duration(llm.Timeout*10) * time.Second, // 10x timeout for streaming
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Process Server-Sent Events
	return llm.processSSEStream(resp.Body, streamChan)
}

// processSSEStream processes Server-Sent Events from LiteLLM streaming response
func (llm *LiteLLMModel) processSSEStream(body io.ReadCloser, streamChan chan<- *ChatMessageStreamDelta) error {
	buffer := make([]byte, 4096)
	var accumulated strings.Builder

	for {
		n, err := body.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("error reading stream: %w", err)
		}

		if n == 0 {
			break
		}

		accumulated.Write(buffer[:n])
		data := accumulated.String()

		// Process complete SSE events
		lines := strings.Split(data, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				jsonData := strings.TrimPrefix(line, "data: ")
				if jsonData == "[DONE]" {
					return nil
				}

				var delta map[string]interface{}
				if err := json.Unmarshal([]byte(jsonData), &delta); err == nil {
					if streamDelta := llm.parseStreamDelta(delta); streamDelta != nil {
						streamChan <- streamDelta
					}
				}

				// Remove processed lines from buffer
				accumulated.Reset()
				accumulated.WriteString(strings.Join(lines[i+1:], "\n"))
				break
			}
		}
	}

	return nil
}

// parseStreamDelta parses a streaming delta from LiteLLM API response
func (llm *LiteLLMModel) parseStreamDelta(delta map[string]interface{}) *ChatMessageStreamDelta {
	if choices, ok := delta["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if deltaData, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := deltaData["content"].(string); ok {
					return &ChatMessageStreamDelta{
						Content: &content,
					}
				}
			}
		}
	}
	return nil
}

// parseResponse parses the API response into a ChatMessage
func (llm *LiteLLMModel) parseResponse(response map[string]interface{}) (*ChatMessage, error) {
	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("invalid response format: no choices found")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: invalid choice")
	}

	messageData, ok := choice["message"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: no message found")
	}

	message := &ChatMessage{}
	err := message.FromDict(messageData, response, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	// Parse token usage if available
	if usage, ok := response["usage"].(map[string]interface{}); ok {
		if promptTokens, ok := usage["prompt_tokens"].(float64); ok {
			if completionTokens, ok := usage["completion_tokens"].(float64); ok {
				message.TokenUsage = monitoring.NewTokenUsage(int(promptTokens), int(completionTokens))
			}
		}
	}

	// Store raw response
	message.Raw = response

	return message, nil
}

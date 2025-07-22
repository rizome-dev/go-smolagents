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

// Package models - InferenceClientModel implementation
package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/monitoring"
)

// extractTextContent extracts text content from a message regardless of format
func extractTextContent(msg map[string]interface{}) string {
	// First try direct string (for backward compatibility)
	if content, ok := msg["content"].(string); ok {
		return content
	}

	// Handle array of content items (new format)
	if contentArray, ok := msg["content"].([]map[string]interface{}); ok {
		var textParts []string
		for _, item := range contentArray {
			if text, ok := item["text"].(string); ok {
				textParts = append(textParts, text)
			}
		}
		return strings.Join(textParts, " ")
	}

	// Handle array of interface{} (in case of type variations)
	if contentArray, ok := msg["content"].([]interface{}); ok {
		var textParts []string
		for _, item := range contentArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					textParts = append(textParts, text)
				}
			}
		}
		return strings.Join(textParts, " ")
	}

	return ""
}

// InferenceClientModel represents a model using Hugging Face Inference API
type InferenceClientModel struct {
	*BaseModel
	Provider string            `json:"provider"`
	Client   interface{}       `json:"-"` // HTTP client or SDK client
	Token    string            `json:"-"` // API token
	BaseURL  string            `json:"base_url"`
	Headers  map[string]string `json:"headers"`
}

// NewInferenceClientModel creates a new inference client model
func NewInferenceClientModel(modelID string, token string, options map[string]interface{}) *InferenceClientModel {
	if token == "" {
		// Try to get token from environment if not provided
		token = os.Getenv("HF_TOKEN")
		if token == "" {
			fmt.Println("Warning: No HuggingFace token provided. API calls may fail.")
		}
	}
	base := NewBaseModel(modelID, options)

	model := &InferenceClientModel{
		BaseModel: base,
		Token:     token,
		Headers:   make(map[string]string),
	}

	if options != nil {
		if provider, ok := options["provider"].(string); ok {
			model.Provider = provider
		}
		if baseURL, ok := options["base_url"].(string); ok {
			model.BaseURL = baseURL
		}
		if headers, ok := options["headers"].(map[string]string); ok {
			model.Headers = headers
		}
	}

	// Set default base URL if not provided - use new Inference Providers endpoint
	if model.BaseURL == "" {
		model.BaseURL = "https://router.huggingface.co/v1/chat/completions"
	}

	return model
}

// Generate implements Model interface
func (icm *InferenceClientModel) Generate(messages []interface{}, options *GenerateOptions) (*ChatMessage, error) {
	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, icm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("[Model: %s] failed to clean messages: %w", icm.ModelID, err)
	}

	// Prepare completion parameters
	defaultParams := map[string]interface{}{
		"model":       icm.ModelID,
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

	// Check if provider supports structured generation
	if options != nil && options.ResponseFormat != nil && icm.supportsStructuredGeneration() {
		priorityParams["response_format"] = options.ResponseFormat
	}

	kwargs := icm.PrepareCompletionKwargs(options, defaultParams, priorityParams)

	// Make the API call
	result, err := icm.callAPI(kwargs)
	if err != nil {
		return nil, err // Error already has detailed info from callAPI
	}

	// Parse the response
	parsed, err := icm.parseResponse(result)
	if err != nil {
		return nil, fmt.Errorf("[Model: %s, Provider: %s, BaseURL: %s] failed to parse response: %w", icm.ModelID, icm.Provider, icm.BaseURL, err)
	}
	return parsed, nil
}

// GenerateStream implements Model interface
func (icm *InferenceClientModel) GenerateStream(messages []interface{}, options *GenerateOptions) (<-chan *ChatMessageStreamDelta, error) {
	// Check if streaming is supported
	if !icm.SupportsStreaming() {
		return nil, fmt.Errorf("[Model: %s] streaming not supported for this model", icm.ModelID)
	}

	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, icm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("[Model: %s] failed to clean messages: %w", icm.ModelID, err)
	}

	// Prepare completion parameters
	defaultParams := map[string]interface{}{
		"model":       icm.ModelID,
		"messages":    cleanMessages,
		"temperature": 0.7,
		"max_tokens":  2048,
		"stream":      true,
	}

	kwargs := icm.PrepareCompletionKwargs(options, defaultParams, map[string]interface{}{})

	// Create the stream channel
	streamChan := make(chan *ChatMessageStreamDelta, 100)

	// Start streaming in a goroutine
	go func() {
		defer close(streamChan)

		// Make streaming API call (placeholder implementation)
		err := icm.callStreamingAPI(kwargs, streamChan)
		if err != nil {
			// Send error as last delta (in real implementation, you'd handle this differently)
			return
		}
	}()

	return streamChan, nil
}

// SupportsStreaming implements Model interface
func (icm *InferenceClientModel) SupportsStreaming() bool {
	return true // Most inference API providers support streaming
}

// ToDict implements Model interface
func (icm *InferenceClientModel) ToDict() map[string]interface{} {
	result := icm.BaseModel.ToDict()
	result["provider"] = icm.Provider
	result["base_url"] = icm.BaseURL
	result["headers"] = icm.Headers
	return result
}

// supportsStructuredGeneration checks if the provider supports structured generation
func (icm *InferenceClientModel) supportsStructuredGeneration() bool {
	for _, provider := range StructuredGenerationProviders {
		if icm.Provider == provider {
			return true
		}
	}
	return false
}

// callAPI makes the actual API call to HuggingFace Inference API
// logAPIDebugInfo logs debug information for API calls
func (icm *InferenceClientModel) logAPIDebugInfo(kwargs map[string]interface{}) {
	fmt.Printf("[DEBUG] Model callAPI - Model: %s, Provider: %s, BaseURL: %s\n", icm.ModelID, icm.Provider, icm.BaseURL)
	if messages, ok := kwargs["messages"].([]map[string]interface{}); ok {
		fmt.Printf("[DEBUG] Number of messages: %d\n", len(messages))
	}
	if stopSeqs, ok := kwargs["stop"]; ok {
		fmt.Printf("[DEBUG] Stop sequences: %v\n", stopSeqs)
	}
}

// callAPI makes the actual API call to HuggingFace Inference API
func (icm *InferenceClientModel) callAPI(kwargs map[string]interface{}) (map[string]interface{}, error) {
	// Debug logging
	if os.Getenv("SMOLAGENTS_DEBUG") == "true" {
		icm.logAPIDebugInfo(kwargs)
	}

	// Always use OpenAI-compatible API for new Inference Providers
	// The old inference API is deprecated
	return icm.callOpenAICompatibleAPI(kwargs)
}

// logDebugRequest logs debug information for the request
func (icm *InferenceClientModel) logDebugRequest(jsonData []byte) {
	fmt.Printf("[DEBUG] Request to: %s\n", icm.BaseURL)
	fmt.Printf("[DEBUG] Request payload size: %d bytes\n", len(jsonData))
	// Log first 500 chars of request for debugging
	if len(jsonData) > 500 {
		fmt.Printf("[DEBUG] Request preview: %s...\n", string(jsonData[:500]))
	} else {
		fmt.Printf("[DEBUG] Request: %s\n", string(jsonData))
	}
}

// createHTTPRequest creates and configures the HTTP request
func (icm *InferenceClientModel) createHTTPRequest(jsonData []byte) (*http.Request, error) {
	req, err := http.NewRequest("POST", icm.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", icm.Token))

	// Set provider header if specified
	if icm.Provider != "" && icm.Provider != "auto" {
		req.Header.Set("X-Provider", icm.Provider)
	}

	for key, value := range icm.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// executeHTTPRequest executes the HTTP request with optional debug logging
func (icm *InferenceClientModel) executeHTTPRequest(req *http.Request, debugMode bool) (*http.Response, []byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}

	if debugMode {
		fmt.Printf("[DEBUG] Sending HTTP request at %s...\n", time.Now().Format("15:04:05.000"))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if debugMode {
		fmt.Printf("[DEBUG] Received response at %s\n", time.Now().Format("15:04:05.000"))
		fmt.Printf("[DEBUG] Response status: %d\n", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if debugMode {
		fmt.Printf("[DEBUG] Response body size: %d bytes\n", len(body))
	}

	return resp, body, nil
}

// handleErrorResponse processes error responses from the API
func (icm *InferenceClientModel) handleErrorResponse(resp *http.Response, body []byte, debugMode bool) error {
	if debugMode {
		fmt.Printf("[DEBUG] Error response body: %s\n", string(body))
	}
	
	var errorDetails map[string]interface{}
	if json.Unmarshal(body, &errorDetails) == nil {
		if error, ok := errorDetails["error"].(map[string]interface{}); ok {
			if msg, ok := error["message"].(string); ok {
				return fmt.Errorf("[Model: %s, Provider: %s, BaseURL: %s] API error (status %d): %s", icm.ModelID, icm.Provider, icm.BaseURL, resp.StatusCode, msg)
			}
		} else if errorMsg, ok := errorDetails["error"].(string); ok {
			return fmt.Errorf("[Model: %s, Provider: %s, BaseURL: %s] API error (status %d): %s", icm.ModelID, icm.Provider, icm.BaseURL, resp.StatusCode, errorMsg)
		}
	}
	return fmt.Errorf("[Model: %s, Provider: %s, BaseURL: %s] API request failed with status %d: %s", icm.ModelID, icm.Provider, icm.BaseURL, resp.StatusCode, string(body))
}

// parseSuccessResponse parses a successful API response
func (icm *InferenceClientModel) parseSuccessResponse(body []byte, debugMode bool) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		if debugMode {
			fmt.Printf("[DEBUG] Failed to parse JSON response. Body: %s\n", string(body))
		}
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if debugMode {
		icm.logDebugResponse(result)
	}

	return result, nil
}

// logDebugResponse logs debug information for the response
func (icm *InferenceClientModel) logDebugResponse(result map[string]interface{}) {
	if choices, ok := result["choices"].([]interface{}); ok {
		fmt.Printf("[DEBUG] Response has %d choices\n", len(choices))
		if len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if msg, ok := choice["message"].(map[string]interface{}); ok {
					if content, ok := msg["content"].(string); ok {
						fmt.Printf("[DEBUG] Response content length: %d characters\n", len(content))
					}
				}
			}
		}
	}
}

// callOpenAICompatibleAPI handles OpenAI-compatible endpoints (like new HF inference providers)
func (icm *InferenceClientModel) callOpenAICompatibleAPI(kwargs map[string]interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	debugMode := os.Getenv("SMOLAGENTS_DEBUG") == "true"
	if debugMode {
		icm.logDebugRequest(jsonData)
	}

	req, err := icm.createHTTPRequest(jsonData)
	if err != nil {
		return nil, err
	}

	resp, body, err := icm.executeHTTPRequest(req, debugMode)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, icm.handleErrorResponse(resp, body, debugMode)
	}

	return icm.parseSuccessResponse(body, debugMode)
}

// callStreamingAPI makes a streaming API call to HuggingFace Inference API
func (icm *InferenceClientModel) callStreamingAPI(kwargs map[string]interface{}, streamChan chan<- *ChatMessageStreamDelta) error {
	// Prepare the request body for streaming
	requestBody := kwargs
	requestBody["stream"] = true

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Always use the base URL directly for the new Inference Providers API
	url := icm.BaseURL

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for streaming
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", icm.Token))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for key, value := range icm.Headers {
		req.Header.Set(key, value)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 300 * time.Second, // Longer timeout for streaming
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
		return fmt.Errorf("[Model: %s, Provider: %s] API request failed with status %d: %s", icm.ModelID, icm.Provider, resp.StatusCode, string(body))
	}

	// Process Server-Sent Events
	return icm.processSSEStream(resp.Body, streamChan)
}

// parseResponse parses the API response into a ChatMessage
// extractChoices extracts and normalizes the choices array from response
func (icm *InferenceClientModel) extractChoices(response map[string]interface{}, debugMode bool) ([]interface{}, error) {
	var choices []interface{}
	if c, ok := response["choices"].([]interface{}); ok {
		choices = c
	} else if c, ok := response["choices"].([]map[string]interface{}); ok {
		choices = make([]interface{}, len(c))
		for i, v := range c {
			choices[i] = v
		}
	} else {
		if debugMode {
			fmt.Printf("[DEBUG] Invalid response format - full response: %v\n", response)
		}
		return nil, fmt.Errorf("[Model: %s] invalid response format: no choices found", icm.ModelID)
	}
	
	if len(choices) == 0 {
		if debugMode {
			fmt.Printf("[DEBUG] Empty choices array in response\n")
		}
		return nil, fmt.Errorf("[Model: %s] invalid response format: empty choices", icm.ModelID)
	}
	
	return choices, nil
}

// extractMessageFromChoice extracts message data from a choice
func (icm *InferenceClientModel) extractMessageFromChoice(choice interface{}, debugMode bool) (map[string]interface{}, error) {
	choiceMap, ok := choice.(map[string]interface{})
	if !ok {
		if debugMode {
			fmt.Printf("[DEBUG] First choice is not a map: %T\n", choice)
		}
		return nil, fmt.Errorf("[Model: %s] invalid response format: choice is not a map", icm.ModelID)
	}
	
	messageData, ok := choiceMap["message"].(map[string]interface{})
	if !ok {
		if debugMode {
			fmt.Printf("[DEBUG] No message in choice: %v\n", choiceMap)
		}
		return nil, fmt.Errorf("[Model: %s] invalid response format: no message found in choice", icm.ModelID)
	}
	
	return messageData, nil
}

// extractContentFromMessage extracts content from message data, with reasoning_content fallback
func (icm *InferenceClientModel) extractContentFromMessage(messageData map[string]interface{}, debugMode bool) (string, error) {
	content, _ := messageData["content"].(string)
	
	// Check for reasoning_content if content is empty (Kimi model specific)
	if content == "" {
		if reasoningContent, ok := messageData["reasoning_content"].(string); ok && reasoningContent != "" {
			content = reasoningContent
			if debugMode {
				fmt.Printf("[DEBUG] Using reasoning_content field for Kimi model (length: %d)\n", len(content))
			}
		}
	}
	
	// Validate content is not empty
	if content == "" {
		if debugMode {
			fmt.Printf("[DEBUG] Empty content in message: %v\n", messageData)
		}
		return "", fmt.Errorf("[Model: %s] received empty content in response", icm.ModelID)
	}
	
	return content, nil
}

// parseTokenUsage parses token usage information from response
func (icm *InferenceClientModel) parseTokenUsage(response map[string]interface{}, debugMode bool) *monitoring.TokenUsage {
	usage, ok := response["usage"].(map[string]interface{})
	if !ok {
		return nil
	}
	
	promptTokens := 0
	completionTokens := 0
	
	if pt, ok := usage["prompt_tokens"].(float64); ok {
		promptTokens = int(pt)
	}
	if ct, ok := usage["completion_tokens"].(float64); ok {
		completionTokens = int(ct)
	}
	
	if promptTokens == 0 && completionTokens == 0 {
		return nil
	}
	
	if debugMode {
		fmt.Printf("[DEBUG] Token usage - Input: %d, Output: %d, Total: %d\n",
			promptTokens, completionTokens, promptTokens+completionTokens)
	}
	
	return &monitoring.TokenUsage{
		InputTokens:  promptTokens,
		OutputTokens: completionTokens,
		TotalTokens:  promptTokens + completionTokens,
	}
}

func (icm *InferenceClientModel) parseResponse(response map[string]interface{}) (*ChatMessage, error) {
	debugMode := os.Getenv("SMOLAGENTS_DEBUG") == "true"
	
	// Extract choices
	choices, err := icm.extractChoices(response, debugMode)
	if err != nil {
		return nil, err
	}
	
	// Extract message from first choice
	messageData, err := icm.extractMessageFromChoice(choices[0], debugMode)
	if err != nil {
		return nil, err
	}
	
	// Extract content from message
	content, err := icm.extractContentFromMessage(messageData, debugMode)
	if err != nil {
		return nil, err
	}
	
	// Create ChatMessage
	role, _ := messageData["role"].(string)
	message := &ChatMessage{
		Role:    role,
		Content: &content,
		Raw:     response,
	}
	
	// Parse token usage
	message.TokenUsage = icm.parseTokenUsage(response, debugMode)
	
	return message, nil
}

// convertHFToOpenAIFormat converts HuggingFace API response to OpenAI format
func (icm *InferenceClientModel) convertHFToOpenAIFormat(hfResponse interface{}) map[string]interface{} {
	// HuggingFace responses vary by model, this handles common formats

	// Check if it's already in OpenAI format
	if responseMap, ok := hfResponse.(map[string]interface{}); ok {
		if _, hasChoices := responseMap["choices"]; hasChoices {
			return responseMap
		}

		// Handle single generated_text response
		if generated, ok := responseMap["generated_text"].(string); ok {
			// Clean up the generated text by removing the input prompt if present
			content := strings.TrimSpace(generated)
			return icm.createOpenAIResponse(content)
		}

		// Handle error responses
		if errorMsg, ok := responseMap["error"].(string); ok {
			return map[string]interface{}{
				"error": map[string]interface{}{
					"message": errorMsg,
					"type":    "api_error",
				},
			}
		}
	}

	// Handle array response format (most common for HF Inference API)
	if responseArray, ok := hfResponse.([]interface{}); ok && len(responseArray) > 0 {
		if firstResponse, ok := responseArray[0].(map[string]interface{}); ok {
			if generated, ok := firstResponse["generated_text"].(string); ok {
				// Clean up the generated text
				content := strings.TrimSpace(generated)
				return icm.createOpenAIResponse(content)
			}
		}
	}

	// Handle direct string response
	if responseStr, ok := hfResponse.(string); ok {
		content := strings.TrimSpace(responseStr)
		return icm.createOpenAIResponse(content)
	}

	// Fallback: create error response for unexpected format
	return map[string]interface{}{
		"error": map[string]interface{}{
			"message": fmt.Sprintf("Unexpected response format from HuggingFace API: %T", hfResponse),
			"type":    "parse_error",
		},
	}
}

// createOpenAIResponse creates a standardized OpenAI-compatible response
func (icm *InferenceClientModel) createOpenAIResponse(content string) map[string]interface{} {
	// Estimate token usage (rough approximation)
	wordCount := len(strings.Fields(content))
	estimatedTokens := int(float64(wordCount) * 1.3) // Rough token-to-word ratio

	return map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   icm.ModelID,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     100, // HF doesn't provide accurate prompt token counts
			"completion_tokens": estimatedTokens,
			"total_tokens":      100 + estimatedTokens,
		},
	}
}

// processSSEStream processes Server-Sent Events from streaming response
func (icm *InferenceClientModel) processSSEStream(body io.ReadCloser, streamChan chan<- *ChatMessageStreamDelta) error {
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
					if streamDelta := icm.parseStreamDelta(delta); streamDelta != nil {
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

// parseStreamDelta parses a streaming delta from the API response
func (icm *InferenceClientModel) parseStreamDelta(delta map[string]interface{}) *ChatMessageStreamDelta {
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

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}

// Package models - InferenceClientModel implementation
package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/monitoring"
)

// Helper function for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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

	// Set default base URL if not provided
	if model.BaseURL == "" {
		model.BaseURL = "https://api-inference.huggingface.co"
	}

	return model
}

// Generate implements Model interface
func (icm *InferenceClientModel) Generate(messages []interface{}, options *GenerateOptions) (*ChatMessage, error) {
	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, icm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
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

	// Make the API call (placeholder implementation)
	result, err := icm.callAPI(kwargs)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}

	// Parse the response
	return icm.parseResponse(result)
}

// GenerateStream implements Model interface
func (icm *InferenceClientModel) GenerateStream(messages []interface{}, options *GenerateOptions) (<-chan *ChatMessageStreamDelta, error) {
	// Check if streaming is supported
	if !icm.SupportsStreaming() {
		return nil, fmt.Errorf("streaming not supported for this model")
	}

	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, icm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
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
func (icm *InferenceClientModel) callAPI(kwargs map[string]interface{}) (map[string]interface{}, error) {
	// Check if we're using the new chat completions endpoint
	if strings.Contains(icm.BaseURL, "chat/completions") || strings.Contains(icm.BaseURL, "v1/") {
		return icm.callOpenAICompatibleAPI(kwargs)
	}

	// For standard HuggingFace Inference API, we need to format the request properly
	messages, ok := kwargs["messages"].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid messages format: %T", kwargs["messages"])
	}

	// Convert messages to a single input string for HF Inference API
	var inputText strings.Builder
	for _, msg := range messages {
		role, ok := msg["role"].(string)
		if !ok {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok {
			continue
		}

		switch role {
		case "system":
			inputText.WriteString(fmt.Sprintf("System: %s\n", content))
		case "user":
			inputText.WriteString(fmt.Sprintf("User: %s\n", content))
		case "assistant":
			inputText.WriteString(fmt.Sprintf("Assistant: %s\n", content))
		}
	}
	inputText.WriteString("Assistant:")

	// Prepare parameters with correct HF parameter names
	parameters := map[string]interface{}{
		"return_full_text": false,
		"do_sample":        true,
	}

	// Map standard parameters to HF parameter names
	if maxTokens, ok := kwargs["max_tokens"]; ok {
		parameters["max_new_tokens"] = maxTokens
	} else {
		parameters["max_new_tokens"] = 2048 // Default
	}

	if temperature, ok := kwargs["temperature"]; ok {
		parameters["temperature"] = temperature
	} else {
		parameters["temperature"] = 0.7 // Default
	}

	if topP, ok := kwargs["top_p"]; ok {
		parameters["top_p"] = topP
	}

	if topK, ok := kwargs["top_k"]; ok {
		parameters["top_k"] = topK
	}

	if seed, ok := kwargs["seed"]; ok {
		parameters["seed"] = seed
	}

	// Prepare the request body for HuggingFace Inference API
	requestBody := map[string]interface{}{
		"inputs":     inputText.String(),
		"parameters": parameters,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request - HuggingFace Inference API endpoint format
	url := fmt.Sprintf("%s/models/%s", icm.BaseURL, icm.ModelID)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers required by HF Inference API
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", icm.Token))
	req.Header.Set("User-Agent", "smolagents-go/1.0")
	
	// Add optional headers
	for key, value := range icm.Headers {
		req.Header.Set(key, value)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 60 * time.Second, // Longer timeout for HF API
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

	// Check for HTTP errors with more detailed error messages
	if resp.StatusCode >= 400 {
		var errorDetails map[string]interface{}
		if json.Unmarshal(body, &errorDetails) == nil {
			if errorMsg, ok := errorDetails["error"].(string); ok {
				return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, errorMsg)
			}
		}
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response - HF can return either object or array
	var rawResult interface{}
	if err := json.Unmarshal(body, &rawResult); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert HuggingFace format to OpenAI-compatible format
	result := icm.convertHFToOpenAIFormat(rawResult)
	return result, nil
}

// callOpenAICompatibleAPI handles OpenAI-compatible endpoints (like new HF inference providers)
func (icm *InferenceClientModel) callOpenAICompatibleAPI(kwargs map[string]interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", icm.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", icm.Token))
	for key, value := range icm.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
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

	// Create HTTP request
	url := fmt.Sprintf("%s/models/%s", icm.BaseURL, icm.ModelID)
	if strings.Contains(icm.BaseURL, "chat/completions") {
		url = icm.BaseURL
	}

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
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Process Server-Sent Events
	return icm.processSSEStream(resp.Body, streamChan)
}

// parseResponse parses the API response into a ChatMessage
func (icm *InferenceClientModel) parseResponse(response map[string]interface{}) (*ChatMessage, error) {
	choices, ok := response["choices"].([]map[string]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("invalid response format: no choices found")
	}

	choice := choices[0]
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
		if inputTokens, ok := usage["prompt_tokens"].(float64); ok {
			if outputTokens, ok := usage["completion_tokens"].(float64); ok {
				message.TokenUsage = monitoring.NewTokenUsage(int(inputTokens), int(outputTokens))
			}
		}
	}

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

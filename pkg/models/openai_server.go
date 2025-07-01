// Package models - OpenAIServerModel implementation
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

// OpenAIServerModel represents a model using OpenAI-compatible API servers
type OpenAIServerModel struct {
	*BaseModel
	BaseURL string            `json:"base_url"`
	APIKey  string            `json:"-"` // API key (not serialized)
	Headers map[string]string `json:"headers"`
	Client  interface{}       `json:"-"`       // HTTP client
	Timeout int               `json:"timeout"` // Request timeout in seconds
}

// NewOpenAIServerModel creates a new OpenAI server model
func NewOpenAIServerModel(modelID string, baseURL string, apiKey string, options map[string]interface{}) *OpenAIServerModel {
	base := NewBaseModel(modelID, options)

	model := &OpenAIServerModel{
		BaseModel: base,
		BaseURL:   baseURL,
		APIKey:    apiKey,
		Headers:   make(map[string]string),
		Timeout:   30, // Default 30 seconds
	}

	if options != nil {
		if headers, ok := options["headers"].(map[string]string); ok {
			model.Headers = headers
		}
		if timeout, ok := options["timeout"].(int); ok {
			model.Timeout = timeout
		}
	}

	// Set default base URL if not provided
	if model.BaseURL == "" {
		model.BaseURL = "https://api.openai.com/v1"
	}

	return model
}

// Generate implements Model interface
func (osm *OpenAIServerModel) Generate(messages []interface{}, options *GenerateOptions) (*ChatMessage, error) {
	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, osm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
	}

	// Prepare completion parameters
	defaultParams := map[string]interface{}{
		"model":       osm.ModelID,
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
	if options != nil && len(options.StopSequences) > 0 && SupportsStopParameter(osm.ModelID) {
		priorityParams["stop"] = options.StopSequences
	}

	kwargs := osm.PrepareCompletionKwargs(options, defaultParams, priorityParams)

	// Make the API call
	result, err := osm.callAPI(kwargs)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}

	// Parse the response
	return osm.parseResponse(result)
}

// GenerateStream implements Model interface
func (osm *OpenAIServerModel) GenerateStream(messages []interface{}, options *GenerateOptions) (<-chan *ChatMessageStreamDelta, error) {
	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, osm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
	}

	// Prepare completion parameters
	defaultParams := map[string]interface{}{
		"model":       osm.ModelID,
		"messages":    cleanMessages,
		"temperature": 0.7,
		"max_tokens":  2048,
		"stream":      true,
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

	kwargs := osm.PrepareCompletionKwargs(options, defaultParams, priorityParams)

	// Create the stream channel
	streamChan := make(chan *ChatMessageStreamDelta, 100)

	// Start streaming in a goroutine
	go func() {
		defer close(streamChan)

		// Make streaming API call
		err := osm.callStreamingAPI(kwargs, streamChan)
		if err != nil {
			// In a real implementation, you'd send an error delta or handle differently
			return
		}
	}()

	return streamChan, nil
}

// SupportsStreaming implements Model interface
func (osm *OpenAIServerModel) SupportsStreaming() bool {
	return true // OpenAI-compatible APIs typically support streaming
}

// ToDict implements Model interface
func (osm *OpenAIServerModel) ToDict() map[string]interface{} {
	result := osm.BaseModel.ToDict()
	result["base_url"] = osm.BaseURL
	result["headers"] = osm.Headers
	result["timeout"] = osm.Timeout
	return result
}

// callAPI makes the actual API call to OpenAI-compatible server
func (osm *OpenAIServerModel) callAPI(kwargs map[string]interface{}) (map[string]interface{}, error) {
	// Prepare JSON request body
	jsonData, err := json.Marshal(kwargs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(osm.BaseURL, "/"))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if osm.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", osm.APIKey))
	}
	for key, value := range osm.Headers {
		req.Header.Set(key, value)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(osm.Timeout) * time.Second,
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

// callStreamingAPI makes a streaming API call to OpenAI-compatible server
func (osm *OpenAIServerModel) callStreamingAPI(kwargs map[string]interface{}, streamChan chan<- *ChatMessageStreamDelta) error {
	// Prepare JSON request body for streaming
	kwargs["stream"] = true
	jsonData, err := json.Marshal(kwargs)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(osm.BaseURL, "/"))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for streaming
	req.Header.Set("Content-Type", "application/json")
	if osm.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", osm.APIKey))
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for key, value := range osm.Headers {
		req.Header.Set(key, value)
	}

	// Create HTTP client with longer timeout for streaming
	client := &http.Client{
		Timeout: time.Duration(osm.Timeout*10) * time.Second, // 10x timeout for streaming
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
	return osm.processSSEStream(resp.Body, streamChan)
}

// parseResponse parses the API response into a ChatMessage
func (osm *OpenAIServerModel) parseResponse(response map[string]interface{}) (*ChatMessage, error) {
	// Handle choices field - it can be either []interface{} or []map[string]interface{}
	var choices []map[string]interface{}

	if choicesRaw, ok := response["choices"]; ok {
		switch c := choicesRaw.(type) {
		case []interface{}:
			// Convert []interface{} to []map[string]interface{}
			choices = make([]map[string]interface{}, len(c))
			for i, choice := range c {
				if choiceMap, ok := choice.(map[string]interface{}); ok {
					choices[i] = choiceMap
				} else {
					return nil, fmt.Errorf("invalid choice format at index %d: %T", i, choice)
				}
			}
		case []map[string]interface{}:
			choices = c
		default:
			return nil, fmt.Errorf("invalid choices type: %T", choicesRaw)
		}
	}

	if len(choices) == 0 {
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

// AzureOpenAIServerModel extends OpenAIServerModel for Azure-specific configurations
type AzureOpenAIServerModel struct {
	*OpenAIServerModel
	APIVersion   string `json:"api_version"`
	Deployment   string `json:"deployment"`
	ResourceName string `json:"resource_name"`
}

// NewAzureOpenAIServerModel creates a new Azure OpenAI server model
func NewAzureOpenAIServerModel(modelID string, resourceName string, deployment string, apiKey string, options map[string]interface{}) *AzureOpenAIServerModel {
	// Construct Azure-specific base URL
	baseURL := fmt.Sprintf("https://%s.openai.azure.com/openai/deployments/%s", resourceName, deployment)

	if options == nil {
		options = make(map[string]interface{})
	}

	// Set default API version
	apiVersion := "2024-08-01-preview"
	if version, ok := options["api_version"].(string); ok {
		apiVersion = version
	}

	openaiModel := NewOpenAIServerModel(modelID, baseURL, apiKey, options)

	return &AzureOpenAIServerModel{
		OpenAIServerModel: openaiModel,
		APIVersion:        apiVersion,
		Deployment:        deployment,
		ResourceName:      resourceName,
	}
}

// ToDict implements Model interface for Azure variant
func (azm *AzureOpenAIServerModel) ToDict() map[string]interface{} {
	result := azm.OpenAIServerModel.ToDict()
	result["api_version"] = azm.APIVersion
	result["deployment"] = azm.Deployment
	result["resource_name"] = azm.ResourceName
	return result
}

// TransformersModel represents a local model using Hugging Face Transformers
type TransformersModel struct {
	*BaseModel
	ModelPath        string                 `json:"model_path"`
	TokenizerPath    string                 `json:"tokenizer_path"`
	Device           string                 `json:"device"`
	TorchDtype       string                 `json:"torch_dtype"`
	ModelKwargs      map[string]interface{} `json:"model_kwargs"`
	TokenizerKwargs  map[string]interface{} `json:"tokenizer_kwargs"`
	StoppingCriteria interface{}            `json:"-"` // StoppingCriteriaList
}

// NewTransformersModelImpl creates a new transformers model implementation
func NewTransformersModelImpl(modelID string, options map[string]interface{}) *TransformersModel {
	base := NewBaseModel(modelID, options)

	model := &TransformersModel{
		BaseModel:       base,
		ModelPath:       modelID, // Default to model ID as path
		Device:          "auto",
		TorchDtype:      "auto",
		ModelKwargs:     make(map[string]interface{}),
		TokenizerKwargs: make(map[string]interface{}),
	}

	if options != nil {
		if modelPath, ok := options["model_path"].(string); ok {
			model.ModelPath = modelPath
		}
		if tokenizerPath, ok := options["tokenizer_path"].(string); ok {
			model.TokenizerPath = tokenizerPath
		}
		if device, ok := options["device"].(string); ok {
			model.Device = device
		}
		if torchDtype, ok := options["torch_dtype"].(string); ok {
			model.TorchDtype = torchDtype
		}
		if modelKwargs, ok := options["model_kwargs"].(map[string]interface{}); ok {
			model.ModelKwargs = modelKwargs
		}
		if tokenizerKwargs, ok := options["tokenizer_kwargs"].(map[string]interface{}); ok {
			model.TokenizerKwargs = tokenizerKwargs
		}
	}

	return model
}

// Generate implements Model interface for Transformers
func (tm *TransformersModel) Generate(messages []interface{}, options *GenerateOptions) (*ChatMessage, error) {
	// This is a placeholder implementation
	// In a real implementation, this would:
	// 1. Load the model and tokenizer using transformers library
	// 2. Apply chat template to format messages
	// 3. Tokenize the input
	// 4. Generate with the model
	// 5. Decode the output
	// 6. Handle tool calling if supported

	return &ChatMessage{
		Role:       "assistant",
		Content:    openAIStrPtr("This is a placeholder response from TransformersModel"),
		TokenUsage: monitoring.NewTokenUsage(100, 50),
	}, nil
}

// GenerateStream implements Model interface for Transformers
func (tm *TransformersModel) GenerateStream(messages []interface{}, options *GenerateOptions) (<-chan *ChatMessageStreamDelta, error) {
	return nil, fmt.Errorf("streaming not yet implemented for TransformersModel")
}

// SupportsStreaming implements Model interface for Transformers
func (tm *TransformersModel) SupportsStreaming() bool {
	return false // Not implemented yet
}

// processSSEStream processes Server-Sent Events from OpenAI streaming response
func (osm *OpenAIServerModel) processSSEStream(body io.ReadCloser, streamChan chan<- *ChatMessageStreamDelta) error {
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
					if streamDelta := osm.parseStreamDelta(delta); streamDelta != nil {
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

// parseStreamDelta parses a streaming delta from OpenAI API response
func (osm *OpenAIServerModel) parseStreamDelta(delta map[string]interface{}) *ChatMessageStreamDelta {
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

// Helper function to create string pointer for OpenAI models
func openAIStrPtr(s string) *string {
	return &s
}

// ToDict implements Model interface for Transformers
func (tm *TransformersModel) ToDict() map[string]interface{} {
	result := tm.BaseModel.ToDict()
	result["model_path"] = tm.ModelPath
	result["tokenizer_path"] = tm.TokenizerPath
	result["device"] = tm.Device
	result["torch_dtype"] = tm.TorchDtype
	result["model_kwargs"] = tm.ModelKwargs
	result["tokenizer_kwargs"] = tm.TokenizerKwargs
	return result
}

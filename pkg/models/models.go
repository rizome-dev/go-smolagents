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

// Package models provides interfaces and implementations for different LLM backends.
//
// This includes local models, API-based models, and utilities for message processing,
// tool calling, and structured generation.
package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/rizome-dev/go-smolagents/pkg/monitoring"
	"github.com/rizome-dev/go-smolagents/pkg/tools"
)

// MessageRole represents the role of a message in conversation
type MessageRole string

const (
	RoleUser         MessageRole = "user"
	RoleAssistant    MessageRole = "assistant"
	RoleSystem       MessageRole = "system"
	RoleToolCall     MessageRole = "tool-call"
	RoleToolResponse MessageRole = "tool-response"
)

// Roles returns all available message roles
func (MessageRole) Roles() []string {
	return []string{
		string(RoleUser),
		string(RoleAssistant),
		string(RoleSystem),
		string(RoleToolCall),
		string(RoleToolResponse),
	}
}

// ToolRoleConversions maps tool-specific roles to standard roles
var ToolRoleConversions = map[MessageRole]MessageRole{
	RoleToolCall:     RoleAssistant,
	RoleToolResponse: RoleUser,
}

// StructuredGenerationProviders lists providers that support structured generation
var StructuredGenerationProviders = []string{"cerebras", "fireworks-ai"}

// ModelsWithoutStopSequences lists models that don't support stop sequences parameter
var ModelsWithoutStopSequences = []string{
	"moonshotai/Kimi-K2-Instruct",
	// Add other models here as needed
}

// CodeAgentResponseFormat defines the JSON schema for code agent responses
var CodeAgentResponseFormat = &ResponseFormat{
	Type: "json_schema",
	JSONSchema: &JSONSchema{
		Name:        "ThoughtAndCodeAnswer",
		Description: "Structured response format for code agents with thought process and code",
		Schema: map[string]interface{}{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]interface{}{
				"thought": map[string]interface{}{
					"type":        "string",
					"description": "A free form text description of the thought process.",
					"title":       "Thought",
				},
				"code": map[string]interface{}{
					"type":        "string",
					"description": "Valid Go code snippet implementing the thought.",
					"title":       "Code",
				},
			},
			"required": []string{"thought", "code"},
			"title":    "ThoughtAndCodeAnswer",
		},
		Strict: true,
	},
	Strict: true,
}

// ChatMessageToolCallDefinition defines a function call within a tool call
type ChatMessageToolCallDefinition struct {
	Arguments   interface{} `json:"arguments"`
	Name        string      `json:"name"`
	Description *string     `json:"description,omitempty"`
}

// ChatMessageToolCall represents a tool call made by the model
type ChatMessageToolCall struct {
	Function ChatMessageToolCallDefinition `json:"function"`
	ID       string                        `json:"id"`
	Type     string                        `json:"type"`
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role       string                 `json:"role"`
	Content    *string                `json:"content,omitempty"`
	ToolCalls  []ChatMessageToolCall  `json:"tool_calls,omitempty"`
	Raw        interface{}            `json:"-"` // Stores raw output from API
	TokenUsage *monitoring.TokenUsage `json:"token_usage,omitempty"`
}

// NewChatMessage creates a new chat message
func NewChatMessage(role string, content string) *ChatMessage {
	return &ChatMessage{
		Role:    role,
		Content: &content,
	}
}

// ModelDumpJSON returns JSON representation excluding raw field
func (cm *ChatMessage) ModelDumpJSON() ([]byte, error) {
	// Create a copy without the Raw field for serialization
	copy := struct {
		Role       string                 `json:"role"`
		Content    *string                `json:"content,omitempty"`
		ToolCalls  []ChatMessageToolCall  `json:"tool_calls,omitempty"`
		TokenUsage *monitoring.TokenUsage `json:"token_usage,omitempty"`
	}{
		Role:       cm.Role,
		Content:    cm.Content,
		ToolCalls:  cm.ToolCalls,
		TokenUsage: cm.TokenUsage,
	}
	return json.Marshal(copy)
}

// FromDict creates a ChatMessage from a dictionary
func (cm *ChatMessage) FromDict(data map[string]interface{}, raw interface{}, tokenUsage *monitoring.TokenUsage) error {
	if role, ok := data["role"].(string); ok {
		cm.Role = role
	}

	if content, ok := data["content"].(string); ok {
		cm.Content = &content
	}

	if toolCallsData, ok := data["tool_calls"].([]interface{}); ok {
		cm.ToolCalls = make([]ChatMessageToolCall, len(toolCallsData))
		for i, tcData := range toolCallsData {
			if tcMap, ok := tcData.(map[string]interface{}); ok {
				var tc ChatMessageToolCall
				if id, ok := tcMap["id"].(string); ok {
					tc.ID = id
				}
				if tcType, ok := tcMap["type"].(string); ok {
					tc.Type = tcType
				}
				if function, ok := tcMap["function"].(map[string]interface{}); ok {
					if name, ok := function["name"].(string); ok {
						tc.Function.Name = name
					}
					if args, ok := function["arguments"]; ok {
						tc.Function.Arguments = args
					}
					if desc, ok := function["description"].(string); ok {
						tc.Function.Description = &desc
					}
				}
				cm.ToolCalls[i] = tc
			}
		}
	}

	cm.Raw = raw
	cm.TokenUsage = tokenUsage
	return nil
}

// ToDict returns a dictionary representation of the message
func (cm *ChatMessage) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"role": cm.Role,
	}

	if cm.Content != nil {
		result["content"] = *cm.Content
	}

	if len(cm.ToolCalls) > 0 {
		toolCalls := make([]map[string]interface{}, len(cm.ToolCalls))
		for i, tc := range cm.ToolCalls {
			function := map[string]interface{}{
				"name":      tc.Function.Name,
				"arguments": tc.Function.Arguments,
			}
			if tc.Function.Description != nil {
				function["description"] = *tc.Function.Description
			}

			toolCalls[i] = map[string]interface{}{
				"id":       tc.ID,
				"type":     tc.Type,
				"function": function,
			}
		}
		result["tool_calls"] = toolCalls
	}

	if cm.TokenUsage != nil {
		result["token_usage"] = map[string]interface{}{
			"input_tokens":  cm.TokenUsage.InputTokens,
			"output_tokens": cm.TokenUsage.OutputTokens,
			"total_tokens":  cm.TokenUsage.TotalTokens,
		}
	}

	return result
}

// ChatMessageStreamDelta represents a streaming delta for chat messages
type ChatMessageStreamDelta struct {
	Content    *string                `json:"content,omitempty"`
	ToolCalls  []ChatMessageToolCall  `json:"tool_calls,omitempty"`
	TokenUsage *monitoring.TokenUsage `json:"token_usage,omitempty"`
}

// GenerateOptions represents options for model generation
type GenerateOptions struct {
	StopSequences     []string               `json:"stop_sequences,omitempty"`
	ResponseFormat    *ResponseFormat        `json:"response_format,omitempty"`     // Enhanced structured format
	ResponseFormatRaw map[string]interface{} `json:"response_format_raw,omitempty"` // Raw format for backwards compatibility
	ToolsToCallFrom   []tools.Tool           `json:"tools_to_call_from,omitempty"`
	Grammar           map[string]string      `json:"grammar,omitempty"`
	MaxTokens         *int                   `json:"max_tokens,omitempty"`
	Temperature       *float64               `json:"temperature,omitempty"`
	TopP              *float64               `json:"top_p,omitempty"`
	TopK              *int                   `json:"top_k,omitempty"`
	FrequencyPenalty  *float64               `json:"frequency_penalty,omitempty"`
	PresencePenalty   *float64               `json:"presence_penalty,omitempty"`
	Seed              *int                   `json:"seed,omitempty"`
	ValidateOutput    bool                   `json:"validate_output"`  // Whether to validate structured output
	RetryOnFailure    bool                   `json:"retry_on_failure"` // Whether to retry on validation failure
	MaxRetries        int                    `json:"max_retries"`      // Maximum number of retries
	CustomParams      map[string]interface{} `json:"custom_params,omitempty"`
}

// Model represents the main interface for all LLM models
type Model interface {
	// Generate generates a response from the model
	Generate(
		messages []interface{}, // Can be dict or ChatMessage
		options *GenerateOptions,
	) (*ChatMessage, error)

	// GenerateStream generates a streaming response (if supported)
	GenerateStream(
		messages []interface{},
		options *GenerateOptions,
	) (<-chan *ChatMessageStreamDelta, error)

	// ParseToolCalls parses tool calls from message content
	ParseToolCalls(message *ChatMessage) (*ChatMessage, error)

	// ToDict converts the model to a dictionary representation
	ToDict() map[string]interface{}

	// GetModelID returns the model identifier
	GetModelID() string

	// SupportsStreaming returns true if the model supports streaming
	SupportsStreaming() bool

	// Close cleans up model resources
	Close() error
}

// BaseModel provides common functionality for model implementations
type BaseModel struct {
	FlattenMessagesAsText bool   `json:"flatten_messages_as_text"`
	ToolNameKey           string `json:"tool_name_key"`
	ToolArgumentsKey      string `json:"tool_arguments_key"`
	ModelID               string `json:"model_id"`

	// Deprecated token counting properties (maintained for compatibility)
	LastInputTokenCount  *int `json:"_last_input_token_count,omitempty"`
	LastOutputTokenCount *int `json:"_last_output_token_count,omitempty"`

	CustomParams map[string]interface{} `json:"custom_params,omitempty"`
}

// NewBaseModel creates a new base model
func NewBaseModel(modelID string, options map[string]interface{}) *BaseModel {
	base := &BaseModel{
		ModelID:               modelID,
		FlattenMessagesAsText: false,
		ToolNameKey:           "name",
		ToolArgumentsKey:      "arguments",
		CustomParams:          make(map[string]interface{}),
	}

	if options != nil {
		if flatten, ok := options["flatten_messages_as_text"].(bool); ok {
			base.FlattenMessagesAsText = flatten
		}
		if toolNameKey, ok := options["tool_name_key"].(string); ok {
			base.ToolNameKey = toolNameKey
		}
		if toolArgsKey, ok := options["tool_arguments_key"].(string); ok {
			base.ToolArgumentsKey = toolArgsKey
		}

		// Store any additional custom parameters
		for k, v := range options {
			if k != "flatten_messages_as_text" && k != "tool_name_key" && k != "tool_arguments_key" {
				base.CustomParams[k] = v
			}
		}
	}

	return base
}

// GetModelID implements Model
func (bm *BaseModel) GetModelID() string {
	return bm.ModelID
}

// SupportsStreaming implements Model (default: false)
func (bm *BaseModel) SupportsStreaming() bool {
	return false
}

// Close implements Model (default: no-op)
func (bm *BaseModel) Close() error {
	return nil
}

// ParseToolCalls implements Model
func (bm *BaseModel) ParseToolCalls(message *ChatMessage) (*ChatMessage, error) {
	if message.Content == nil || len(message.ToolCalls) > 0 {
		return message, nil // Already has tool calls or no content
	}

	// Parse tool calls from content using regex patterns
	content := *message.Content

	// Pattern to match both "Action:" style and "Calling tools:" style
	patterns := []struct {
		namePattern string
		argsPattern string
	}{
		// ReAct style: Action: tool_name and Action Input: {...}
		{`(?i)action:\s*([^\n]+)`, `(?i)action\s+input:\s*({.*?})`},
		// Tool calling style: - tool_name: map[key:value]
		{`(?i)-\s*([^:]+):\s*map\[([^\]]+)\]`, ``},
		// Tool calling style: - tool_name: {...}
		{`(?i)-\s*([^:]+):\s*({.*?})`, ``},
	}

	for _, p := range patterns {
		namePattern := regexp.MustCompile(p.namePattern)
		nameMatches := namePattern.FindStringSubmatch(content)

		if len(nameMatches) > 1 {
			toolName := strings.TrimSpace(nameMatches[1])
			var arguments interface{}

			if p.argsPattern != "" {
				// Use separate args pattern
				argsPattern := regexp.MustCompile(p.argsPattern)
				argsMatches := argsPattern.FindStringSubmatch(content)
				if len(argsMatches) > 1 {
					var jsonArgs map[string]interface{}
					if err := json.Unmarshal([]byte(argsMatches[1]), &jsonArgs); err == nil {
						arguments = jsonArgs
					} else {
						arguments = argsMatches[1]
					}
				} else {
					arguments = map[string]interface{}{}
				}
			} else if len(nameMatches) > 2 {
				// Arguments captured in the same pattern
				argsStr := nameMatches[2]
				if strings.HasPrefix(argsStr, "{") {
					// JSON format
					var jsonArgs map[string]interface{}
					if err := json.Unmarshal([]byte(argsStr), &jsonArgs); err == nil {
						arguments = jsonArgs
					} else {
						arguments = argsStr
					}
				} else {
					// map[key:value] format - parse manually
					args := make(map[string]interface{})
					// Simple parsing for map[answer:text] format
					if strings.Contains(argsStr, "answer:") {
						answerPattern := regexp.MustCompile(`answer:(.*)`)
						if matches := answerPattern.FindStringSubmatch(argsStr); len(matches) > 1 {
							args["answer"] = strings.TrimSpace(matches[1])
						}
					}
					arguments = args
				}
			} else {
				arguments = map[string]interface{}{}
			}

			toolCall := ChatMessageToolCall{
				ID:   fmt.Sprintf("call_%d", len(message.ToolCalls)),
				Type: "function",
				Function: ChatMessageToolCallDefinition{
					Name:      toolName,
					Arguments: arguments,
				},
			}

			message.ToolCalls = append(message.ToolCalls, toolCall)
			break // Found a match, stop trying other patterns
		}
	}

	return message, nil
}

// ToDict implements Model
func (bm *BaseModel) ToDict() map[string]interface{} {
	result := map[string]interface{}{
		"model_id":                 bm.ModelID,
		"flatten_messages_as_text": bm.FlattenMessagesAsText,
		"tool_name_key":            bm.ToolNameKey,
		"tool_arguments_key":       bm.ToolArgumentsKey,
	}

	if bm.LastInputTokenCount != nil {
		result["_last_input_token_count"] = *bm.LastInputTokenCount
	}
	if bm.LastOutputTokenCount != nil {
		result["_last_output_token_count"] = *bm.LastOutputTokenCount
	}

	for k, v := range bm.CustomParams {
		result[k] = v
	}

	return result
}

// addDefaultParams adds default parameters to kwargs
func (bm *BaseModel) addDefaultParams(kwargs map[string]interface{}, defaultParams map[string]interface{}) {
	for k, v := range defaultParams {
		kwargs[k] = v
	}
}

// addGenerationOptions adds generation-related options to kwargs
func (bm *BaseModel) addGenerationOptions(kwargs map[string]interface{}, options *GenerateOptions) {
	if options == nil {
		return
	}

	if options.MaxTokens != nil {
		kwargs["max_tokens"] = *options.MaxTokens
	}
	if options.Temperature != nil {
		kwargs["temperature"] = *options.Temperature
	}
	if options.TopP != nil {
		kwargs["top_p"] = *options.TopP
	}
	if options.TopK != nil {
		kwargs["top_k"] = *options.TopK
	}
	if options.FrequencyPenalty != nil {
		kwargs["frequency_penalty"] = *options.FrequencyPenalty
	}
	if options.PresencePenalty != nil {
		kwargs["presence_penalty"] = *options.PresencePenalty
	}
	if options.Seed != nil {
		kwargs["seed"] = *options.Seed
	}
	if len(options.StopSequences) > 0 && SupportsStopParameter(bm.ModelID) {
		kwargs["stop"] = options.StopSequences
	}

	// Add custom parameters
	for k, v := range options.CustomParams {
		kwargs[k] = v
	}
}

// addResponseFormat adds response format to kwargs
func (bm *BaseModel) addResponseFormat(kwargs map[string]interface{}, options *GenerateOptions) {
	if options == nil {
		return
	}

	// Prefer structured format over raw
	if options.ResponseFormat != nil {
		kwargs["response_format"] = bm.ConvertResponseFormat(options.ResponseFormat)
	} else if options.ResponseFormatRaw != nil {
		kwargs["response_format"] = options.ResponseFormatRaw
	}
}

// addPriorityParams adds priority parameters that override everything
func (bm *BaseModel) addPriorityParams(kwargs map[string]interface{}, priorityParams map[string]interface{}) {
	for k, v := range priorityParams {
		kwargs[k] = v
	}
}

// PrepareCompletionKwargs prepares keyword arguments for model completion
func (bm *BaseModel) PrepareCompletionKwargs(
	options *GenerateOptions,
	defaultParams map[string]interface{},
	priorityParams map[string]interface{},
) map[string]interface{} {
	kwargs := make(map[string]interface{})

	// Add parameters in order of precedence
	bm.addDefaultParams(kwargs, defaultParams)
	bm.addGenerationOptions(kwargs, options)
	bm.addResponseFormat(kwargs, options)
	bm.addPriorityParams(kwargs, priorityParams)

	return kwargs
}

// ConvertResponseFormat converts ResponseFormat to API-compatible format
func (bm *BaseModel) ConvertResponseFormat(format *ResponseFormat) map[string]interface{} {
	if format == nil {
		return nil
	}

	result := map[string]interface{}{
		"type": format.Type,
	}

	if format.JSONSchema != nil {
		result["json_schema"] = map[string]interface{}{
			"name":        format.JSONSchema.Name,
			"description": format.JSONSchema.Description,
			"schema":      format.JSONSchema.Schema,
			"strict":      format.JSONSchema.Strict,
		}
	} else if format.Schema != nil {
		result["schema"] = format.Schema
	}

	if format.Strict {
		result["strict"] = format.Strict
	}

	return result
}

// GenerateWithStructuredOutput generates output with structured response format
func (bm *BaseModel) GenerateWithStructuredOutput(
	model Model,
	messages []interface{},
	options *GenerateOptions,
) (*StructuredOutput, error) {
	if options == nil {
		options = &GenerateOptions{}
	}

	// Set default retry options
	if options.MaxRetries == 0 {
		options.MaxRetries = 3
	}

	var lastError error

	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		// Generate the response
		response, err := model.Generate(messages, options)
		if err != nil {
			lastError = err
			if !options.RetryOnFailure || attempt == options.MaxRetries {
				return nil, fmt.Errorf("generation failed: %w", err)
			}
			continue
		}

		// Parse structured output if format is specified
		if options.ResponseFormat != nil {
			structuredOutput, err := ParseStructuredOutput(*response.Content, options.ResponseFormat)
			if err != nil {
				lastError = err
				if !options.RetryOnFailure || attempt == options.MaxRetries {
					// Return with errors for debugging
					return &StructuredOutput{
						Content: *response.Content,
						Raw:     *response.Content,
						Format:  options.ResponseFormat,
						Valid:   false,
						Errors:  []string{err.Error()},
					}, err
				}
				continue
			}

			// Validate output if required
			if options.ValidateOutput && !structuredOutput.Valid {
				lastError = fmt.Errorf("output validation failed: %v", structuredOutput.Errors)
				if !options.RetryOnFailure || attempt == options.MaxRetries {
					return structuredOutput, lastError
				}
				continue
			}

			// Add response metadata
			structuredOutput.Metadata["attempt"] = attempt + 1
			structuredOutput.Metadata["token_usage"] = response.TokenUsage
			structuredOutput.Metadata["raw_response"] = response.Raw

			return structuredOutput, nil
		}

		// No structured format - return as basic structured output
		return &StructuredOutput{
			Content: *response.Content,
			Raw:     *response.Content,
			Format:  TextFormat,
			Valid:   true,
			Errors:  []string{},
			Metadata: map[string]interface{}{
				"attempt":      attempt + 1,
				"token_usage":  response.TokenUsage,
				"raw_response": response.Raw,
			},
		}, nil
	}

	return nil, fmt.Errorf("failed after %d attempts, last error: %w", options.MaxRetries+1, lastError)
}

// PrepareStructuredPrompt prepares a prompt for structured generation
func (bm *BaseModel) PrepareStructuredPrompt(
	basePrompt string,
	format *ResponseFormat,
) string {
	if format == nil {
		return basePrompt
	}

	return GenerateStructuredPrompt(basePrompt, format)
}

// Utility functions

// ParseJSONIfNeeded parses arguments as JSON if they're a string
func ParseJSONIfNeeded(arguments interface{}) interface{} {
	if str, ok := arguments.(string); ok {
		var parsed interface{}
		if err := json.Unmarshal([]byte(str), &parsed); err == nil {
			return parsed
		}
	}
	return arguments
}

// GetToolJSONSchema converts a tool to JSON schema format
func GetToolJSONSchema(tool tools.Tool) map[string]interface{} {
	inputs := tool.GetInputs()
	properties := make(map[string]interface{})
	required := []string{}

	for key, input := range inputs {
		prop := map[string]interface{}{
			"type":        input.Type,
			"description": input.Description,
		}

		if input.Type == "any" {
			prop["type"] = "string"
		}

		properties[key] = prop

		if !input.Nullable {
			required = append(required, key)
		}
	}

	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        tool.GetName(),
			"description": tool.GetDescription(),
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": properties,
				"required":   required,
			},
		},
	}
}

// RemoveStopSequences removes stop sequences from content
func RemoveStopSequences(content string, stopSequences []string) string {
	for _, stopSeq := range stopSequences {
		if len(content) >= len(stopSeq) && content[len(content)-len(stopSeq):] == stopSeq {
			content = content[:len(content)-len(stopSeq)]
		}
	}
	return content
}

// extractMessageContent extracts text content from a message map
func extractMessageContent(msg map[string]interface{}) string {
	// First try direct string (for backward compatibility)
	if content, ok := msg["content"].(string); ok {
		return content
	}

	// Handle array of content items
	if contentArray, ok := msg["content"].([]map[string]interface{}); ok {
		var textParts []string
		for _, item := range contentArray {
			if text, ok := item["text"].(string); ok {
				textParts = append(textParts, text)
			}
		}
		return strings.Join(textParts, " ")
	}

	// Handle array of interface{}
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

// GetCleanMessageList preprocesses messages for model consumption
func GetCleanMessageList(
	messages []interface{},
	roleConversions map[MessageRole]MessageRole,
	convertImagesToImageURLs bool,
	flattenMessagesAsText bool,
) ([]map[string]interface{}, error) {
	var result []map[string]interface{}

	for _, msg := range messages {
		var msgMap map[string]interface{}

		switch m := msg.(type) {
		case *ChatMessage:
			msgMap = m.ToDict()
		case map[string]interface{}:
			msgMap = m
		default:
			return nil, fmt.Errorf("unsupported message type: %T", msg)
		}

		// Apply role conversions
		if roleConversions != nil {
			if role, ok := msgMap["role"].(string); ok {
				if newRole, exists := roleConversions[MessageRole(role)]; exists {
					msgMap["role"] = string(newRole)
				}
			}
		}

		// Merge consecutive messages with the same role
		if len(result) > 0 && result[len(result)-1]["role"] == msgMap["role"] {
			// Merge content - need to handle both string and array formats
			lastMsg := result[len(result)-1]
			lastContent := extractMessageContent(lastMsg)
			currentContent := extractMessageContent(msgMap)

			if lastContent != "" && currentContent != "" {
				// Create a new text content item with merged text
				result[len(result)-1]["content"] = []map[string]interface{}{
					{"type": "text", "text": lastContent + "\n" + currentContent},
				}
			}
		} else {
			result = append(result, msgMap)
		}
	}

	return result, nil
}

// SupportsStopParameter checks if a model supports stop parameters
func SupportsStopParameter(modelID string) bool {
	// Some models don't support stop parameters (e.g., reasoning models)
	reasoningModels := []string{"o1", "o1-preview", "o1-mini", "o3", "o3-mini"}

	for _, reasoningModel := range reasoningModels {
		if strings.Contains(strings.ToLower(modelID), reasoningModel) {
			return false
		}
	}

	// Check if the model is in the list of models without stop sequences
	for _, model := range ModelsWithoutStopSequences {
		if modelID == model {
			return false
		}
	}

	return true
}

// ModelType represents different types of models supported
type ModelType string

const (
	ModelTypeInferenceClient ModelType = "inference_client"
	ModelTypeOpenAIServer    ModelType = "openai_server"
	ModelTypeAzureOpenAI     ModelType = "azure_openai"
	ModelTypeLiteLLM         ModelType = "litellm"
	ModelTypeBedrockModel    ModelType = "bedrock"
	ModelTypeTransformers    ModelType = "transformers"
)

// CreateModel creates a model of the specified type
func CreateModel(modelType ModelType, modelID string, options map[string]interface{}) (Model, error) {
	switch modelType {
	case ModelTypeInferenceClient:
		token, _ := options["token"].(string)
		return NewInferenceClientModel(modelID, token, options), nil
	case ModelTypeOpenAIServer:
		baseURL, _ := options["base_url"].(string)
		apiKey, _ := options["api_key"].(string)
		return NewOpenAIServerModel(modelID, baseURL, apiKey, options), nil
	case ModelTypeAzureOpenAI:
		resourceName, _ := options["resource_name"].(string)
		deployment, _ := options["deployment"].(string)
		apiKey, _ := options["api_key"].(string)
		return NewAzureOpenAIServerModel(modelID, resourceName, deployment, apiKey, options), nil
	case ModelTypeLiteLLM:
		return NewLiteLLMModel(modelID, options), nil
	case ModelTypeBedrockModel:
		return NewAmazonBedrockModel(modelID, options), nil
	case ModelTypeTransformers:
		return NewTransformersModelImpl(modelID, options), nil
	default:
		return nil, fmt.Errorf("unsupported model type: %s", modelType)
	}
}

// AutoDetectModelType attempts to detect the appropriate model type from modelID
func AutoDetectModelType(modelID string) ModelType {
	// Check for specific model patterns
	if strings.Contains(modelID, "bedrock/") || IsSupportedBedrockModel(modelID) {
		return ModelTypeBedrockModel
	}

	// Default to inference client for HuggingFace models
	return ModelTypeInferenceClient
}

// ValidateModelConfiguration validates a model configuration
func ValidateModelConfiguration(modelType ModelType, modelID string, options map[string]interface{}) error {
	if modelID == "" {
		return fmt.Errorf("model ID cannot be empty")
	}

	switch modelType {
	case ModelTypeInferenceClient:
		if options != nil {
			if token, ok := options["token"].(string); ok && token == "" {
				return fmt.Errorf("inference client requires a valid token")
			}
		}
	case ModelTypeOpenAIServer, ModelTypeAzureOpenAI:
		if options != nil {
			if baseURL, ok := options["base_url"].(string); ok && baseURL == "" {
				return fmt.Errorf("OpenAI server model requires a valid base URL")
			}
		}
	case ModelTypeBedrockModel:
		if !IsSupportedBedrockModel(modelID) {
			return fmt.Errorf("model %s is not supported by Bedrock", modelID)
		}
	}

	return nil
}

// GetModelInfo returns information about a model
func GetModelInfo(modelType ModelType, modelID string) map[string]interface{} {
	info := map[string]interface{}{
		"model_id":   modelID,
		"model_type": string(modelType),
	}

	switch modelType {
	case ModelTypeBedrockModel:
		if defaults := GetBedrockModelDefaults(modelID); defaults != nil {
			info["defaults"] = defaults
		}
	}

	return info
}

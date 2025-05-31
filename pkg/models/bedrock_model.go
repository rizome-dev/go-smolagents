// Package models - AmazonBedrockModel implementation
package models

import (
	"fmt"
	"strings"

	"github.com/rizome-dev/smolagentsgo/pkg/monitoring"
)

// BedrockClient interface for AWS Bedrock client abstraction
type BedrockClient interface {
	Converse(input map[string]interface{}) (map[string]interface{}, error)
	ConverseStream(input map[string]interface{}) (<-chan map[string]interface{}, error)
}

// AmazonBedrockModel represents a model using Amazon Bedrock API
type AmazonBedrockModel struct {
	*BaseModel
	Client                BedrockClient          `json:"-"`
	ClientKwargs          map[string]interface{} `json:"client_kwargs"`
	CustomRoleConversions map[string]string      `json:"custom_role_conversions"`
	InferenceConfig       map[string]interface{} `json:"inference_config"`
	GuardrailConfig       map[string]interface{} `json:"guardrail_config"`
	AdditionalModelFields map[string]interface{} `json:"additional_model_fields"`
}

// NewAmazonBedrockModel creates a new Amazon Bedrock model
func NewAmazonBedrockModel(modelID string, options map[string]interface{}) *AmazonBedrockModel {
	base := NewBaseModel(modelID, options)

	// Bedrock only supports `assistant` and `user` roles
	// Many Bedrock models do not allow conversations to start with the `assistant` role,
	// so the default is set to `user/user`
	defaultRoleConversions := map[string]string{
		"system":        "user",
		"assistant":     "user",
		"tool-call":     "user",
		"tool-response": "user",
	}

	model := &AmazonBedrockModel{
		BaseModel:             base,
		ClientKwargs:          make(map[string]interface{}),
		CustomRoleConversions: defaultRoleConversions,
		InferenceConfig:       make(map[string]interface{}),
		GuardrailConfig:       make(map[string]interface{}),
		AdditionalModelFields: make(map[string]interface{}),
	}

	if options != nil {
		if clientKwargs, ok := options["client_kwargs"].(map[string]interface{}); ok {
			model.ClientKwargs = clientKwargs
		}
		if roleConversions, ok := options["custom_role_conversions"].(map[string]string); ok {
			model.CustomRoleConversions = roleConversions
		}
		if inferenceConfig, ok := options["inference_config"].(map[string]interface{}); ok {
			model.InferenceConfig = inferenceConfig
		}
		if guardrailConfig, ok := options["guardrail_config"].(map[string]interface{}); ok {
			model.GuardrailConfig = guardrailConfig
		}
		if additionalFields, ok := options["additional_model_fields"].(map[string]interface{}); ok {
			model.AdditionalModelFields = additionalFields
		}
		if client, ok := options["client"].(BedrockClient); ok {
			model.Client = client
		}
	}

	// Create default client if none provided
	if model.Client == nil {
		model.Client = NewDefaultBedrockClient(model.ClientKwargs)
	}

	return model
}

// Generate implements Model interface
func (bm *AmazonBedrockModel) Generate(messages []interface{}, options *GenerateOptions) (*ChatMessage, error) {
	// Prepare completion kwargs
	kwargs, err := bm.prepareCompletionKwargs(messages, options)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare completion kwargs: %w", err)
	}

	// Make the API call
	response, err := bm.Client.Converse(kwargs)
	if err != nil {
		return nil, fmt.Errorf("Bedrock API call failed: %w", err)
	}

	// Parse the response
	return bm.parseResponse(response)
}

// GenerateStream implements Model interface
func (bm *AmazonBedrockModel) GenerateStream(messages []interface{}, options *GenerateOptions) (<-chan *ChatMessageStreamDelta, error) {
	// Prepare completion kwargs
	kwargs, err := bm.prepareCompletionKwargs(messages, options)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare completion kwargs: %w", err)
	}

	// Create the stream channel
	streamChan := make(chan *ChatMessageStreamDelta, 100)

	// Start streaming in a goroutine
	go func() {
		defer close(streamChan)

		// Make streaming API call
		responseChan, err := bm.Client.ConverseStream(kwargs)
		if err != nil {
			return
		}

		for event := range responseChan {
			if delta := bm.parseStreamDelta(event); delta != nil {
				streamChan <- delta
			}
		}
	}()

	return streamChan, nil
}

// SupportsStreaming implements Model interface
func (bm *AmazonBedrockModel) SupportsStreaming() bool {
	return true // Bedrock supports streaming
}

// ToDict implements Model interface
func (bm *AmazonBedrockModel) ToDict() map[string]interface{} {
	result := bm.BaseModel.ToDict()
	result["client_kwargs"] = bm.ClientKwargs
	result["custom_role_conversions"] = bm.CustomRoleConversions
	result["inference_config"] = bm.InferenceConfig
	result["guardrail_config"] = bm.GuardrailConfig
	result["additional_model_fields"] = bm.AdditionalModelFields
	return result
}

// prepareCompletionKwargs prepares the completion arguments for Bedrock API
func (bm *AmazonBedrockModel) prepareCompletionKwargs(messages []interface{}, options *GenerateOptions) (map[string]interface{}, error) {
	// Convert role conversions
	roleConversions := make(map[MessageRole]MessageRole)
	for k, v := range bm.CustomRoleConversions {
		roleConversions[MessageRole(k)] = MessageRole(v)
	}

	// Clean and convert messages
	cleanMessages, err := GetCleanMessageList(messages, roleConversions, true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
	}

	// Remove type field from content as Bedrock doesn't support it
	for _, message := range cleanMessages {
		if content, ok := message["content"].([]interface{}); ok {
			for _, contentItem := range content {
				if item, ok := contentItem.(map[string]interface{}); ok {
					delete(item, "type")
				}
			}
		}
	}

	// Prepare base parameters
	kwargs := map[string]interface{}{
		"modelId":  bm.ModelID,
		"messages": cleanMessages,
	}

	// Add inference configuration
	if len(bm.InferenceConfig) > 0 {
		kwargs["inferenceConfig"] = bm.InferenceConfig
	} else if options != nil {
		inferenceConfig := make(map[string]interface{})
		if options.MaxTokens != nil {
			inferenceConfig["maxTokens"] = *options.MaxTokens
		}
		if options.Temperature != nil {
			inferenceConfig["temperature"] = *options.Temperature
		}
		if options.TopP != nil {
			inferenceConfig["topP"] = *options.TopP
		}
		if len(options.StopSequences) > 0 {
			inferenceConfig["stopSequences"] = options.StopSequences
		}
		if len(inferenceConfig) > 0 {
			kwargs["inferenceConfig"] = inferenceConfig
		}
	}

	// Add guardrail configuration
	if len(bm.GuardrailConfig) > 0 {
		kwargs["guardrailConfig"] = bm.GuardrailConfig
	}

	// Add additional model fields
	for k, v := range bm.AdditionalModelFields {
		kwargs[k] = v
	}

	return kwargs, nil
}

// parseResponse parses the Bedrock API response into a ChatMessage
func (bm *AmazonBedrockModel) parseResponse(response map[string]interface{}) (*ChatMessage, error) {
	output, ok := response["output"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: no output found")
	}

	messageData, ok := output["message"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: no message found")
	}

	// Extract content from the first content item
	if content, ok := messageData["content"].([]interface{}); ok && len(content) > 0 {
		if firstContent, ok := content[0].(map[string]interface{}); ok {
			if text, ok := firstContent["text"].(string); ok {
				messageData["content"] = text
			}
		}
	}

	message := &ChatMessage{}
	err := message.FromDict(messageData, response, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	// Parse token usage if available
	if usage, ok := response["usage"].(map[string]interface{}); ok {
		if inputTokens, ok := usage["inputTokens"].(float64); ok {
			if outputTokens, ok := usage["outputTokens"].(float64); ok {
				message.TokenUsage = monitoring.NewTokenUsage(int(inputTokens), int(outputTokens))
			}
		}
	}

	// Store raw response
	message.Raw = response

	return message, nil
}

// parseStreamDelta parses a streaming delta from Bedrock API response
func (bm *AmazonBedrockModel) parseStreamDelta(event map[string]interface{}) *ChatMessageStreamDelta {
	if contentBlockDelta, ok := event["contentBlockDelta"].(map[string]interface{}); ok {
		if delta, ok := contentBlockDelta["delta"].(map[string]interface{}); ok {
			if text, ok := delta["text"].(string); ok {
				return &ChatMessageStreamDelta{
					Content: &text,
				}
			}
		}
	}

	// Check for usage information
	if metadata, ok := event["metadata"].(map[string]interface{}); ok {
		if usage, ok := metadata["usage"].(map[string]interface{}); ok {
			if inputTokens, ok := usage["inputTokens"].(float64); ok {
				if outputTokens, ok := usage["outputTokens"].(float64); ok {
					return &ChatMessageStreamDelta{
						TokenUsage: monitoring.NewTokenUsage(int(inputTokens), int(outputTokens)),
					}
				}
			}
		}
	}

	return nil
}

// DefaultBedrockClient is a placeholder implementation of BedrockClient
type DefaultBedrockClient struct {
	kwargs map[string]interface{}
}

// NewDefaultBedrockClient creates a new default Bedrock client
func NewDefaultBedrockClient(kwargs map[string]interface{}) *DefaultBedrockClient {
	return &DefaultBedrockClient{
		kwargs: kwargs,
	}
}

// Converse implements BedrockClient interface (placeholder)
func (dbc *DefaultBedrockClient) Converse(input map[string]interface{}) (map[string]interface{}, error) {
	// This is a placeholder implementation
	// In a real implementation, this would:
	// 1. Use AWS SDK to create a bedrock-runtime client
	// 2. Call the Converse API with proper authentication
	// 3. Handle AWS-specific error formats
	// 4. Return the parsed response

	// Extract model ID for response
	modelID, _ := input["modelId"].(string)

	// Create placeholder response in Bedrock format
	return map[string]interface{}{
		"ResponseMetadata": map[string]interface{}{
			"RequestId": "placeholder-request-id",
		},
		"output": map[string]interface{}{
			"message": map[string]interface{}{
				"role": "assistant",
				"content": []map[string]interface{}{
					{
						"text": "This is a placeholder response from Amazon Bedrock model: " + modelID,
					},
				},
			},
		},
		"stopReason": "end_turn",
		"usage": map[string]interface{}{
			"inputTokens":  100,
			"outputTokens": 50,
			"totalTokens":  150,
		},
	}, nil
}

// ConverseStream implements BedrockClient interface (placeholder)
func (dbc *DefaultBedrockClient) ConverseStream(input map[string]interface{}) (<-chan map[string]interface{}, error) {
	// This is a placeholder implementation
	// In a real implementation, this would:
	// 1. Use AWS SDK to create a bedrock-runtime client
	// 2. Call the ConverseStream API with proper authentication
	// 3. Process the streaming response events
	// 4. Return a channel of streaming events

	eventChan := make(chan map[string]interface{}, 10)

	go func() {
		defer close(eventChan)

		// Send sample streaming events
		words := []string{"This", " is", " a", " streaming", " response", " from", " Bedrock", "."}
		for _, word := range words {
			eventChan <- map[string]interface{}{
				"contentBlockDelta": map[string]interface{}{
					"delta": map[string]interface{}{
						"text": word,
					},
				},
			}
		}

		// Send final usage information
		eventChan <- map[string]interface{}{
			"metadata": map[string]interface{}{
				"usage": map[string]interface{}{
					"inputTokens":  100,
					"outputTokens": 8,
				},
			},
		}
	}()

	return eventChan, nil
}

// BedrockModelRegistry contains model-specific configurations for Bedrock
var BedrockModelRegistry = map[string]map[string]interface{}{
	"amazon.titan-text-express-v1": {
		"max_tokens":  8192,
		"temperature": 0.7,
	},
	"amazon.titan-text-lite-v1": {
		"max_tokens":  4096,
		"temperature": 0.7,
	},
	"anthropic.claude-3-sonnet-20240229-v1:0": {
		"max_tokens":  4096,
		"temperature": 0.7,
	},
	"anthropic.claude-3-haiku-20240307-v1:0": {
		"max_tokens":  4096,
		"temperature": 0.7,
	},
	"anthropic.claude-3-opus-20240229-v1:0": {
		"max_tokens":  4096,
		"temperature": 0.7,
	},
	"meta.llama3-8b-instruct-v1:0": {
		"max_tokens":  2048,
		"temperature": 0.7,
	},
	"meta.llama3-70b-instruct-v1:0": {
		"max_tokens":  2048,
		"temperature": 0.7,
	},
	"us.amazon.nova-pro-v1:0": {
		"max_tokens":  4096,
		"temperature": 0.7,
	},
	"us.amazon.nova-lite-v1:0": {
		"max_tokens":  4096,
		"temperature": 0.7,
	},
	"us.amazon.nova-micro-v1:0": {
		"max_tokens":  4096,
		"temperature": 0.7,
	},
}

// GetBedrockModelDefaults returns default configuration for a Bedrock model
func GetBedrockModelDefaults(modelID string) map[string]interface{} {
	if defaults, exists := BedrockModelRegistry[modelID]; exists {
		// Create a copy to avoid modifying the registry
		result := make(map[string]interface{})
		for k, v := range defaults {
			result[k] = v
		}
		return result
	}

	// Return generic defaults if model not found in registry
	return map[string]interface{}{
		"max_tokens":  2048,
		"temperature": 0.7,
	}
}

// IsSupportedBedrockModel checks if a model ID is supported by Bedrock
func IsSupportedBedrockModel(modelID string) bool {
	_, exists := BedrockModelRegistry[modelID]
	return exists || strings.Contains(modelID, "bedrock/")
}

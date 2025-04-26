package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HuggingFaceModel implements the Model interface for HuggingFace Inference API
type HuggingFaceModel struct {
	ModelID     string
	Token       string
	Streaming   bool
	Parameters  map[string]interface{}
	TokenCounts map[string]int
}

// NewHuggingFaceModel creates a new HuggingFaceModel
func NewHuggingFaceModel(
	modelID string,
	token string,
	streaming bool,
	parameters map[string]interface{},
) (*HuggingFaceModel, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model ID cannot be empty")
	}

	if token == "" {
		return nil, fmt.Errorf("HF token cannot be empty")
	}

	// Default parameters if not provided
	if parameters == nil {
		parameters = map[string]interface{}{
			"max_tokens":         1024,
			"temperature":        0.7,
			"top_p":              0.9,
			"repetition_penalty": 1.0,
		}
	}

	return &HuggingFaceModel{
		ModelID:     modelID,
		Token:       token,
		Streaming:   streaming,
		Parameters:  parameters,
		TokenCounts: make(map[string]int),
	}, nil
}

// Call implements the Model.Call method
func (m *HuggingFaceModel) Call(
	messages []map[string]interface{},
	stopSequences []string,
	grammar string,
	toolsToCallFrom []interface{},
	kwargs map[string]interface{},
) (*ChatMessage, error) {
	// Format messages into a prompt string
	prompt := ""
	for _, msg := range messages {
		role, ok := msg["role"]
		if !ok {
			return nil, fmt.Errorf("message missing role field")
		}

		// Convert role to string, handling the MessageRole type
		var roleStr string
		switch v := role.(type) {
		case string:
			roleStr = v
		case MessageRole:
			roleStr = string(v)
		default:
			return nil, fmt.Errorf("invalid role type: %T", role)
		}

		content, ok := msg["content"]
		if !ok {
			return nil, fmt.Errorf("message missing content field")
		}

		// Convert content to string
		var contentStr string
		switch v := content.(type) {
		case string:
			contentStr = v
		default:
			contentStr = fmt.Sprintf("%v", v)
		}

		if roleStr == "system" {
			prompt += contentStr + "\n\n"
		} else if roleStr == "user" {
			prompt += "User: " + contentStr + "\n"
		} else if roleStr == "assistant" {
			prompt += "Assistant: " + contentStr + "\n"
		}
	}

	if len(messages) > 0 {
		lastMsgRole, ok := messages[len(messages)-1]["role"]
		if !ok {
			return nil, fmt.Errorf("last message missing role field")
		}

		// Convert last message role to string
		var lastRoleStr string
		switch v := lastMsgRole.(type) {
		case string:
			lastRoleStr = v
		case MessageRole:
			lastRoleStr = string(v)
		default:
			return nil, fmt.Errorf("invalid role type for last message: %T", lastMsgRole)
		}

		if lastRoleStr == "user" {
			prompt += "Assistant: "
		}
	}

	// Prepare request body
	requestBody := map[string]interface{}{
		"inputs": prompt,
		"parameters": map[string]interface{}{
			"max_new_tokens":     m.Parameters["max_tokens"],
			"temperature":        m.Parameters["temperature"],
			"top_p":              m.Parameters["top_p"],
			"repetition_penalty": m.Parameters["repetition_penalty"],
			"do_sample":          true,
		},
	}

	// Add stop sequences if provided
	if len(stopSequences) > 0 {
		requestBody["parameters"].(map[string]interface{})["stop"] = stopSequences
	}

	// Add additional parameters from kwargs
	if kwargs != nil {
		params := requestBody["parameters"].(map[string]interface{})
		for k, v := range kwargs {
			params[k] = v
		}
	}

	// Convert request body to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api-inference.huggingface.co/models/"+m.ModelID, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.Token)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned error: %s, body: %s", resp.Status, string(bodyBytes))
	}

	// Parse response
	var generatedText string

	// Try parsing as array first
	var arrayResponse []struct {
		Generated_text string `json:"generated_text"`
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(bodyBytes, &arrayResponse)
	if err == nil && len(arrayResponse) > 0 {
		// Successfully parsed as array
		generatedText = arrayResponse[0].Generated_text
	} else {
		// Try parsing as single object
		var simpleResponse struct {
			Generated_text string `json:"generated_text"`
		}

		err = json.Unmarshal(bodyBytes, &simpleResponse)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, string(bodyBytes))
		}

		generatedText = simpleResponse.Generated_text
	}

	// Store token counts (not available with this API)
	m.TokenCounts["prompt_tokens"] = 0
	m.TokenCounts["completion_tokens"] = 0
	m.TokenCounts["total_tokens"] = 0

	return &ChatMessage{
		Role:    RoleAssistant,
		Content: generatedText,
	}, nil
}

// GetTokenCounts returns the token counts
func (m *HuggingFaceModel) GetTokenCounts() map[string]int {
	return m.TokenCounts
}

// ToDict converts the model to a dictionary representation
func (m *HuggingFaceModel) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"model_id":     m.ModelID,
		"streaming":    m.Streaming,
		"parameters":   m.Parameters,
		"token_counts": m.TokenCounts,
	}
}

// MaximumContentLength returns the maximum content length (tokens) for this model
func (m *HuggingFaceModel) MaximumContentLength() int {
	// Default to a reasonable value for most HF models
	return 4096
}

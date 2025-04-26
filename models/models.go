// Package models provides interfaces and implementations for working with language models
package models

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// MessageRole represents the role of a message in a conversation
type MessageRole string

const (
	// RoleSystem is the role for system messages
	RoleSystem MessageRole = "system"
	// RoleUser is the role for user messages
	RoleUser MessageRole = "user"
	// RoleAssistant is the role for assistant messages
	RoleAssistant MessageRole = "assistant"
	// RoleTool is the role for tool messages
	RoleTool MessageRole = "tool"
)

// Roles returns all possible message roles
func (m MessageRole) Roles() []string {
	return []string{
		string(RoleSystem),
		string(RoleUser),
		string(RoleAssistant),
		string(RoleTool),
	}
}

// ToolRoleConversions maps special roles to standard roles for compatibility
var ToolRoleConversions = map[MessageRole]MessageRole{
	RoleTool: RoleAssistant,
}

// MessageContent represents the content of a message
type MessageContent struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ImageURL  string      `json:"image_url,omitempty"`
	ImageData image.Image `json:"-"`
}

// Message represents a message in a conversation
type Message struct {
	Role       MessageRole           `json:"role"`
	Content    []MessageContent      `json:"content,omitempty"`
	Name       string                `json:"name,omitempty"`
	ToolCalls  []ChatMessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID string                `json:"tool_call_id,omitempty"`
}

// ChatMessageToolCallDefinition represents a tool call definition
type ChatMessageToolCallDefinition struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatMessageToolCall represents a tool call
type ChatMessageToolCall struct {
	ID       string                        `json:"id"`
	Type     string                        `json:"type"`
	Function ChatMessageToolCallDefinition `json:"function"`
}

// ChatMessage represents a message from a chat model
type ChatMessage struct {
	Content      string                `json:"content"`
	ToolCalls    []ChatMessageToolCall `json:"tool_calls,omitempty"`
	Role         MessageRole           `json:"role"`
	FunctionCall *struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function_call,omitempty"`
}

// Model defines the interface for language models
type Model interface {
	// Call processes the input messages and returns the model's response
	Call(
		messages []map[string]interface{},
		stopSequences []string,
		grammar string,
		toolsToCallFrom []interface{},
		kwargs map[string]interface{},
	) (*ChatMessage, error)

	// GetTokenCounts returns the token counts for input and output
	GetTokenCounts() map[string]int

	// ToDict converts the model to a dictionary representation
	ToDict() map[string]interface{}
}

// RemoveStopSequences removes stop sequences from text
func RemoveStopSequences(text string, stopSequences []string) string {
	result := text
	for _, seq := range stopSequences {
		result = strings.ReplaceAll(result, seq, "")
	}
	return result
}

// GetCleanMessageList converts a slice of Message to a slice of maps
func GetCleanMessageList(messages []Message) []map[string]interface{} {
	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		result[i] = map[string]interface{}{
			"role": msg.Role,
		}

		if len(msg.Content) > 0 {
			if len(msg.Content) == 1 && msg.Content[0].Type == "text" {
				result[i]["content"] = msg.Content[0].Text
			} else {
				content := make([]map[string]interface{}, len(msg.Content))
				for j, c := range msg.Content {
					contentItem := map[string]interface{}{
						"type": c.Type,
					}
					if c.Type == "text" {
						contentItem["text"] = c.Text
					} else if c.Type == "image_url" {
						contentItem["image_url"] = c.ImageURL
					} else if c.Type == "image" && c.ImageData != nil {
						contentItem["image_url"] = map[string]string{
							"url": "data:image/png;base64," + encodeImageBase64(c.ImageData),
						}
					}
					content[j] = contentItem
				}
				result[i]["content"] = content
			}
		}

		if msg.Name != "" {
			result[i]["name"] = msg.Name
		}

		if msg.ToolCallID != "" {
			result[i]["tool_call_id"] = msg.ToolCallID
		}

		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]interface{}, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				toolCalls[j] = map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				}
			}
			result[i]["tool_calls"] = toolCalls
		}
	}
	return result
}

// GetToolCallFromText extracts a tool call from text
func GetToolCallFromText(text string) (*ChatMessageToolCall, error) {
	// Parse the text as JSON
	var toolCall ChatMessageToolCall
	err := json.Unmarshal([]byte(text), &toolCall)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tool call from text: %w", err)
	}
	return &toolCall, nil
}

// ParseJSONIfNeeded parses a string as JSON if needed
func ParseJSONIfNeeded(text string) (interface{}, error) {
	// Try to parse as JSON
	var result interface{}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		err := json.Unmarshal([]byte(text), &result)
		if err == nil {
			return result, nil
		}
	}
	return text, nil
}

// SupportsStopParameter checks if a model supports the stop parameter
func SupportsStopParameter(model Model) bool {
	// This is a placeholder - in a real implementation,
	// we would check the model's capabilities
	return true
}

// encodeImageBase64 encodes an image to a base64 string
func encodeImageBase64(img image.Image) string {
	if img == nil {
		return ""
	}

	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)

	// Encode as PNG
	err := png.Encode(encoder, img)
	if err != nil {
		return ""
	}

	encoder.Close()
	return buf.String()
}

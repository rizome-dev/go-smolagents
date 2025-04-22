// Package models provides structures and interfaces for working with language models
package models

import (
	"encoding/json"
	"fmt"
	"image"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// MessageRole defines the possible roles in a conversation
type MessageRole string

const (
	// MessageRoleSystem represents the system prompt
	MessageRoleSystem MessageRole = "system"
	// MessageRoleUser represents a user message
	MessageRoleUser MessageRole = "user"
	// MessageRoleAssistant represents an assistant message
	MessageRoleAssistant MessageRole = "assistant"
	// MessageRoleToolCall represents a tool call from the assistant
	MessageRoleToolCall MessageRole = "tool-call"
	// MessageRoleToolResponse represents a response from a tool
	MessageRoleToolResponse MessageRole = "tool-response"
)

// Roles returns all possible message roles
func (m MessageRole) Roles() []string {
	return []string{
		string(MessageRoleSystem),
		string(MessageRoleUser),
		string(MessageRoleAssistant),
		string(MessageRoleToolCall),
		string(MessageRoleToolResponse),
	}
}

// ToolRoleConversions maps special roles to standard roles for compatibility
var ToolRoleConversions = map[MessageRole]MessageRole{
	MessageRoleToolCall:     MessageRoleAssistant,
	MessageRoleToolResponse: MessageRoleUser,
}

// MessageContent represents different types of content in a message
type MessageContent struct {
	Type  string      `json:"type"`
	Text  string      `json:"text,omitempty"`
	Image image.Image `json:"image,omitempty"`
	// Could add more types like audio, etc.
}

// Message represents a chat message with role and content
type Message struct {
	Role    MessageRole      `json:"role"`
	Content []MessageContent `json:"content"`
}

// ChatMessageToolCallDefinition defines the structure for a tool call
type ChatMessageToolCallDefinition struct {
	Arguments   interface{} `json:"arguments"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
}

// ChatMessageToolCall represents a call to a tool
type ChatMessageToolCall struct {
	Function ChatMessageToolCallDefinition `json:"function"`
	ID       string                        `json:"id"`
	Type     string                        `json:"type"`
}

// ChatMessage represents a message from the model
type ChatMessage struct {
	Role      string                `json:"role"`
	Content   string                `json:"content,omitempty"`
	ToolCalls []ChatMessageToolCall `json:"tool_calls,omitempty"`
	Raw       interface{}           `json:"raw,omitempty"`
}

// ModelDumpJSON serializes the ChatMessage to JSON
func (c *ChatMessage) ModelDumpJSON() (string, error) {
	// Create a copy without Raw field to avoid circular references
	type chatMessageCopy ChatMessage
	copy := chatMessageCopy(*c)
	copy.Raw = nil

	data, err := json.Marshal(copy)
	if err != nil {
		return "", fmt.Errorf("error marshaling ChatMessage: %w", err)
	}
	return string(data), nil
}

// FromDict creates a ChatMessage from a dictionary representation
func FromDict(data map[string]interface{}, raw interface{}) (*ChatMessage, error) {
	role, ok := data["role"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'role' field")
	}

	var content string
	if c, ok := data["content"].(string); ok {
		content = c
	}

	var toolCalls []ChatMessageToolCall
	if tc, ok := data["tool_calls"].([]interface{}); ok {
		toolCalls = make([]ChatMessageToolCall, len(tc))
		for i, call := range tc {
			callMap, ok := call.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid tool call format")
			}

			id, ok := callMap["id"].(string)
			if !ok {
				return nil, fmt.Errorf("missing or invalid 'id' field in tool call")
			}

			toolType, ok := callMap["type"].(string)
			if !ok {
				return nil, fmt.Errorf("missing or invalid 'type' field in tool call")
			}

			functionMap, ok := callMap["function"].(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("missing or invalid 'function' field in tool call")
			}

			name, ok := functionMap["name"].(string)
			if !ok {
				return nil, fmt.Errorf("missing or invalid 'name' field in function")
			}

			description := ""
			if desc, ok := functionMap["description"].(string); ok {
				description = desc
			}

			toolCalls[i] = ChatMessageToolCall{
				ID:   id,
				Type: toolType,
				Function: ChatMessageToolCallDefinition{
					Name:        name,
					Description: description,
					Arguments:   functionMap["arguments"],
				},
			}
		}
	}

	return &ChatMessage{
		Role:      role,
		Content:   content,
		ToolCalls: toolCalls,
		Raw:       raw,
	}, nil
}

// ParseJSONIfNeeded attempts to parse a string as JSON if it's not already a map
func ParseJSONIfNeeded(arguments interface{}) interface{} {
	switch args := arguments.(type) {
	case map[string]interface{}:
		return args
	case string:
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(args), &result); err == nil {
			return result
		}
		return args
	default:
		return arguments
	}
}

// RemoveStopSequences removes stop sequences from the end of content
func RemoveStopSequences(content string, stopSequences []string) string {
	for _, stopSeq := range stopSequences {
		if strings.HasSuffix(content, stopSeq) {
			return content[:len(content)-len(stopSeq)]
		}
	}
	return content
}

// GetCleanMessageList converts raw message list to a clean format
func GetCleanMessageList(
	messageList []map[string]interface{},
	roleConversions map[MessageRole]MessageRole,
	convertImagesToImageURLs bool,
	flattenMessagesAsText bool,
) ([]map[string]interface{}, error) {
	var outputMessageList []map[string]interface{}

	// Deep copy the message list to avoid modifying the original
	for _, msg := range messageList {
		role, ok := msg["role"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'role' field in message")
		}

		// Check if role is valid
		foundValidRole := false
		for _, validRole := range MessageRoleSystem.Roles() {
			if role == validRole {
				foundValidRole = true
				break
			}
		}
		if !foundValidRole {
			return nil, fmt.Errorf("incorrect role %s, only %v are supported", role, MessageRoleSystem.Roles())
		}

		// Apply role conversions if needed
		if convRole, ok := roleConversions[MessageRole(role)]; ok {
			role = string(convRole)
		}

		// Process content
		var content interface{}
		if v, ok := msg["content"]; ok {
			content = v
		} else {
			content = []map[string]interface{}{{"type": "text", "text": ""}}
		}

		// Handle content conversion
		contentList, ok := content.([]map[string]interface{})
		if ok {
			for i, element := range contentList {
				if elementType, ok := element["type"].(string); ok && elementType == "image" {
					if flattenMessagesAsText {
						return nil, fmt.Errorf("cannot use images with flattenMessagesAsText=true")
					}

					if convertImagesToImageURLs {
						if img, ok := element["image"].(image.Image); ok {
							// Convert image to URL
							contentList[i] = map[string]interface{}{
								"type": "image_url",
								"image_url": map[string]interface{}{
									"url": fmt.Sprintf("data:image/png;base64,%s", encodeImageBase64(img)),
								},
							}
						}
					}
				}
			}
		}

		// Find or append to the last message with the same role
		if len(outputMessageList) > 0 && outputMessageList[len(outputMessageList)-1]["role"] == role {
			lastMsg := outputMessageList[len(outputMessageList)-1]
			lastContentList, ok := lastMsg["content"].([]map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid content format in message")
			}

			if flattenMessagesAsText {
				if contentText, ok := content.([]map[string]interface{})[0]["text"].(string); ok {
					if lastText, ok := lastContentList[0]["text"].(string); ok {
						lastContentList[0]["text"] = lastText + "\n" + contentText
						outputMessageList[len(outputMessageList)-1]["content"] = lastContentList
					}
				}
			} else {
				if contentList, ok := content.([]map[string]interface{}); ok {
					for _, el := range contentList {
						if el["type"] == "text" && len(lastContentList) > 0 && lastContentList[len(lastContentList)-1]["type"] == "text" {
							// Merge consecutive text messages
							lastContentList[len(lastContentList)-1]["text"] = fmt.Sprintf("%s\n%s",
								lastContentList[len(lastContentList)-1]["text"], el["text"])
						} else {
							lastContentList = append(lastContentList, el)
						}
					}
					outputMessageList[len(outputMessageList)-1]["content"] = lastContentList
				}
			}
		} else {
			if flattenMessagesAsText {
				if contentList, ok := content.([]map[string]interface{}); ok && len(contentList) > 0 {
					if textContent, ok := contentList[0]["text"].(string); ok {
						outputMessageList = append(outputMessageList, map[string]interface{}{
							"role":    role,
							"content": textContent,
						})
					}
				}
			} else {
				outputMessageList = append(outputMessageList, map[string]interface{}{
					"role":    role,
					"content": content,
				})
			}
		}
	}

	return outputMessageList, nil
}

// GetToolCallFromText extracts a tool call from text output
func GetToolCallFromText(text string, toolNameKey, toolArgumentsKey string) (*ChatMessageToolCall, error) {
	var toolCall map[string]interface{}
	err := json.Unmarshal([]byte(text), &toolCall)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON from text: %w", err)
	}

	toolName, ok := toolCall[toolNameKey].(string)
	if !ok {
		return nil, fmt.Errorf("key %s not found in the generated tool call", toolNameKey)
	}

	toolArguments := toolCall[toolArgumentsKey]
	toolArguments = ParseJSONIfNeeded(toolArguments)

	return &ChatMessageToolCall{
		ID:   uuid.NewString(),
		Type: "function",
		Function: ChatMessageToolCallDefinition{
			Name:      toolName,
			Arguments: toolArguments,
		},
	}, nil
}

// SupportsStopParameter checks if a model supports the stop parameter
func SupportsStopParameter(modelID string) bool {
	modelName := modelID
	if parts := strings.Split(modelID, "/"); len(parts) > 1 {
		modelName = parts[len(parts)-1]
	}

	// Models that don't support stop parameter
	pattern := `^(o3[-\d]*|o4-mini[-\d]*)$`
	matched, _ := regexp.MatchString(pattern, modelName)
	return !matched
}

// Model is the interface that all language models must implement
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

// encodeImageBase64 encodes an image to a base64 string
func encodeImageBase64(img image.Image) string {
	// This is a stub - would need a full implementation
	fmt.Println(img.Bounds())
	return "base64_encoded_string"
}

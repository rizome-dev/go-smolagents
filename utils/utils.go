// Package utils provides common utility functions used throughout the library
package utils

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// AgentError represents an error that occurred during agent execution
type AgentError struct {
	Message string
	Cause   error
}

// Error returns the error message
func (e *AgentError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// AgentGenerationError represents an error that occurred during generation
type AgentGenerationError struct {
	AgentError
}

// NewAgentGenerationError creates a new AgentGenerationError
func NewAgentGenerationError(message string, cause error) *AgentGenerationError {
	return &AgentGenerationError{
		AgentError: AgentError{
			Message: message,
			Cause:   cause,
		},
	}
}

// AgentExecutionError represents an error that occurred during execution
type AgentExecutionError struct {
	AgentError
}

// NewAgentExecutionError creates a new AgentExecutionError
func NewAgentExecutionError(message string, cause error) *AgentExecutionError {
	return &AgentExecutionError{
		AgentError: AgentError{
			Message: message,
			Cause:   cause,
		},
	}
}

// AgentParsingError represents an error that occurred during parsing
type AgentParsingError struct {
	AgentError
}

// NewAgentParsingError creates a new AgentParsingError
func NewAgentParsingError(message string, cause error) *AgentParsingError {
	return &AgentParsingError{
		AgentError: AgentError{
			Message: message,
			Cause:   cause,
		},
	}
}

// AgentToolCallError represents an error that occurred during a tool call
type AgentToolCallError struct {
	AgentError
}

// NewAgentToolCallError creates a new AgentToolCallError
func NewAgentToolCallError(message string, cause error) *AgentToolCallError {
	return &AgentToolCallError{
		AgentError: AgentError{
			Message: message,
			Cause:   cause,
		},
	}
}

// AgentToolExecutionError represents an error that occurred during tool execution
type AgentToolExecutionError struct {
	AgentError
}

// NewAgentToolExecutionError creates a new AgentToolExecutionError
func NewAgentToolExecutionError(message string, cause error) *AgentToolExecutionError {
	return &AgentToolExecutionError{
		AgentError: AgentError{
			Message: message,
			Cause:   cause,
		},
	}
}

// AgentMaxStepsError represents an error that occurred when the agent exceeds maximum steps
type AgentMaxStepsError struct {
	AgentError
}

// NewAgentMaxStepsError creates a new AgentMaxStepsError
func NewAgentMaxStepsError(message string, cause error) *AgentMaxStepsError {
	return &AgentMaxStepsError{
		AgentError: AgentError{
			Message: message,
			Cause:   cause,
		},
	}
}

// IsValidName checks if a name is a valid identifier and not a reserved keyword
func IsValidName(name string) bool {
	if name == "" {
		return false
	}

	// Check for valid identifier name
	matched, err := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	if err != nil || !matched {
		return false
	}

	// Check for reserved keywords
	for _, keyword := range []string{
		"break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough",
		"for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range",
		"return", "select", "struct", "switch", "type", "var",
	} {
		if name == keyword {
			return false
		}
	}

	return true
}

// MakeJSONSerializable ensures that a value is JSON serializable
func MakeJSONSerializable(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	// Try to marshal to check if it's serializable
	_, err := json.Marshal(v)
	if err == nil {
		return v
	}

	// If not serializable, convert to string
	return fmt.Sprintf("%v", v)
}

// ParseCodeBlobs extracts code blobs from text
func ParseCodeBlobs(text, language string) []string {
	codeBlocks := []string{}

	// Match markdown code blocks with optional language spec
	pattern := "```(?:" + language + ")?\\n([\\s\\S]*?)```"
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) > 1 {
			codeBlocks = append(codeBlocks, strings.TrimSpace(match[1]))
		}
	}

	return codeBlocks
}

// TruncateContent truncates content to a maximum length
func TruncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	halfLength := (maxLength - 5) / 2
	return content[:halfLength] + "[...]" + content[len(content)-halfLength:]
}

// ParseJSONBlob extracts a JSON blob from text
func ParseJSONBlob(text string) (map[string]interface{}, error) {
	var result map[string]interface{}

	// Try to find a JSON blob in the text
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("could not find a valid JSON blob in text")
	}

	jsonText := text[start : end+1]
	err := json.Unmarshal([]byte(jsonText), &result)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON blob: %w", err)
	}

	return result, nil
}

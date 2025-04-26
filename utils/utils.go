// Package utils provides utility functions for the smolagentsgo library.
// It includes helper functions for name validation, JSON serialization, code parsing,
// and comprehensive error types for agent operations.
package utils

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// IsValidName checks if a name is a valid identifier.
// The function validates that the name starts with a letter or underscore
// and contains only letters, numbers, and underscores.
// This is used to ensure tool and agent names meet Go identifier requirements.
func IsValidName(name string) bool {
	if name == "" {
		return false
	}

	// Must start with a letter or underscore, and contain only letters, numbers, and underscores
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	return matched
}

// MakeJSONSerializable converts a value to a JSON serializable form.
// If the value is already serializable, it is returned as is.
// Otherwise, it is converted to a string representation.
// This is essential for ensuring that all data going into and out of agents
// can be properly serialized for storage or transmission.
func MakeJSONSerializable(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	// Try to marshal and unmarshal to check if it's serializable
	_, err := json.Marshal(v)
	if err == nil {
		return v
	}

	// If not serializable, convert to string
	return fmt.Sprintf("%v", v)
}

// ParseCodeBlobs extracts code blocks from text.
// It can target a specific language if provided, or extract any code blocks if language is empty.
// This function is critical for code-based agents to parse the LLM's response.
func ParseCodeBlobs(text string, language string) []string {
	if language == "" {
		language = ".*?"
	}

	// Look for code blocks like ```language\ncode\n```
	pattern := fmt.Sprintf("```(?:%s)?\\n([\\s\\S]*?)\\n```", language)
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(text, -1)

	result := make([]string, len(matches))
	for i, match := range matches {
		if len(match) > 1 {
			result[i] = match[1]
		}
	}

	return result
}

// TruncateContent truncates a string to the specified length.
// If the string is longer than maxLength, it is truncated and "..." is appended.
// This is useful for limiting the size of logs or memory entries.
func TruncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	return content[:maxLength] + "..."
}

// ParseJSONBlob extracts and parses a JSON object from text.
// It attempts to find a JSON object between curly braces and parse it.
// This is useful for extracting structured data from LLM outputs.
func ParseJSONBlob(text string) (interface{}, error) {
	// Attempt to find JSON object in the text
	re := regexp.MustCompile(`\{.*\}`)
	match := re.FindString(text)

	if match == "" {
		return nil, fmt.Errorf("no JSON object found in text")
	}

	var result interface{}
	if err := json.Unmarshal([]byte(match), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
}

// AgentError is the base interface for all agent errors.
// All error types in the smolagentsgo library implement this interface,
// allowing for consistent error handling and type checking.
type AgentError interface {
	error
	Type() string
}

// baseAgentError provides a base implementation for agent errors.
// It includes an error type, message, and optional cause for proper error chaining.
type baseAgentError struct {
	errorType string
	message   string
	cause     error
}

// Error returns the error message including the cause if present.
// It implements the error interface.
func (e *baseAgentError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.errorType, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.errorType, e.message)
}

// Type returns the error type, allowing for easy categorization.
func (e *baseAgentError) Type() string {
	return e.errorType
}

// Unwrap returns the cause of the error, enabling error unwrapping with errors.Is and errors.As.
func (e *baseAgentError) Unwrap() error {
	return e.cause
}

// AgentGenerationError represents an error during agent output generation.
// This typically occurs when the model fails to generate a valid response.
type AgentGenerationError struct {
	*baseAgentError
}

// NewAgentGenerationError creates a new AgentGenerationError with the given message and cause.
func NewAgentGenerationError(message string, cause error) *AgentGenerationError {
	return &AgentGenerationError{
		baseAgentError: &baseAgentError{
			errorType: "AgentGenerationError",
			message:   message,
			cause:     cause,
		},
	}
}

// AgentExecutionError represents an error during agent execution.
// This is a general error for issues that occur while the agent is running.
type AgentExecutionError struct {
	*baseAgentError
}

// NewAgentExecutionError creates a new AgentExecutionError with the given message and cause.
func NewAgentExecutionError(message string, cause error) *AgentExecutionError {
	return &AgentExecutionError{
		baseAgentError: &baseAgentError{
			errorType: "AgentExecutionError",
			message:   message,
			cause:     cause,
		},
	}
}

// AgentParsingError represents an error during parsing agent output.
// This occurs when the agent's output cannot be properly parsed.
type AgentParsingError struct {
	*baseAgentError
}

// NewAgentParsingError creates a new AgentParsingError with the given message and cause.
func NewAgentParsingError(message string, cause error) *AgentParsingError {
	return &AgentParsingError{
		baseAgentError: &baseAgentError{
			errorType: "AgentParsingError",
			message:   message,
			cause:     cause,
		},
	}
}

// AgentToolCallError represents an error during a tool call.
// This occurs when there's an issue with the arguments or tool name.
type AgentToolCallError struct {
	*baseAgentError
}

// NewAgentToolCallError creates a new AgentToolCallError with the given message and cause.
func NewAgentToolCallError(message string, cause error) *AgentToolCallError {
	return &AgentToolCallError{
		baseAgentError: &baseAgentError{
			errorType: "AgentToolCallError",
			message:   message,
			cause:     cause,
		},
	}
}

// AgentToolExecutionError represents an error during tool execution.
// This occurs when a tool fails to execute properly.
type AgentToolExecutionError struct {
	*baseAgentError
}

// NewAgentToolExecutionError creates a new AgentToolExecutionError with the given message and cause.
func NewAgentToolExecutionError(message string, cause error) *AgentToolExecutionError {
	return &AgentToolExecutionError{
		baseAgentError: &baseAgentError{
			errorType: "AgentToolExecutionError",
			message:   message,
			cause:     cause,
		},
	}
}

// AgentMaxStepsError represents an error when the agent exceeds the maximum steps.
// This is usually a stopping condition rather than a true error.
type AgentMaxStepsError struct {
	*baseAgentError
}

// NewAgentMaxStepsError creates a new AgentMaxStepsError with the given message and cause.
func NewAgentMaxStepsError(message string, cause error) *AgentMaxStepsError {
	return &AgentMaxStepsError{
		baseAgentError: &baseAgentError{
			errorType: "AgentMaxStepsError",
			message:   message,
			cause:     cause,
		},
	}
}

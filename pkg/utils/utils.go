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

// Package utils provides utility functions and error types for the smolagents library.
//
// This includes error handling, text processing, code parsing, and various
// helper functions used throughout the agent system.
package utils

import (
	"fmt"
	"go/token"
	"regexp"
	"strings"
	"unicode"
)

// Error types that mirror the Python implementation

// AgentError is the base error type for all agent-related errors
type AgentError struct {
	Message string
	Cause   error
}

func (e *AgentError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AgentError) Unwrap() error {
	return e.Cause
}

// NewAgentError creates a new AgentError
func NewAgentError(message string, cause ...error) *AgentError {
	var c error
	if len(cause) > 0 {
		c = cause[0]
	}
	return &AgentError{Message: message, Cause: c}
}

// AgentParsingError represents errors during parsing operations
type AgentParsingError struct {
	*AgentError
}

// NewAgentParsingError creates a new AgentParsingError
func NewAgentParsingError(message string, cause ...error) *AgentParsingError {
	return &AgentParsingError{AgentError: NewAgentError(message, cause...)}
}

// AgentExecutionError represents errors during agent execution
type AgentExecutionError struct {
	*AgentError
}

// NewAgentExecutionError creates a new AgentExecutionError
func NewAgentExecutionError(message string, cause ...error) *AgentExecutionError {
	return &AgentExecutionError{AgentError: NewAgentError(message, cause...)}
}

// AgentMaxStepsError represents errors when max steps are exceeded
type AgentMaxStepsError struct {
	*AgentError
}

// NewAgentMaxStepsError creates a new AgentMaxStepsError
func NewAgentMaxStepsError(message string, cause ...error) *AgentMaxStepsError {
	return &AgentMaxStepsError{AgentError: NewAgentError(message, cause...)}
}

// AgentToolCallError represents errors during tool calls
type AgentToolCallError struct {
	*AgentExecutionError
}

// NewAgentToolCallError creates a new AgentToolCallError
func NewAgentToolCallError(message string, cause ...error) *AgentToolCallError {
	return &AgentToolCallError{AgentExecutionError: NewAgentExecutionError(message, cause...)}
}

// AgentToolExecutionError represents errors during tool execution
type AgentToolExecutionError struct {
	*AgentExecutionError
}

// NewAgentToolExecutionError creates a new AgentToolExecutionError
func NewAgentToolExecutionError(message string, cause ...error) *AgentToolExecutionError {
	return &AgentToolExecutionError{AgentExecutionError: NewAgentExecutionError(message, cause...)}
}

// AgentGenerationError represents errors during model generation
type AgentGenerationError struct {
	*AgentError
}

// NewAgentGenerationError creates a new AgentGenerationError
func NewAgentGenerationError(message string, cause ...error) *AgentGenerationError {
	return &AgentGenerationError{AgentError: NewAgentError(message, cause...)}
}

// Code parsing utilities

// CodeBlock represents a parsed code block
type CodeBlock struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

// ParseCodeBlobs extracts code blocks from text, supporting various formats
func ParseCodeBlobs(text string) []CodeBlock {
	var blocks []CodeBlock

	// Pattern for fenced code blocks (```language\ncode\n```)
	fencedPattern := regexp.MustCompile("```(\\w*)\\s*\\n([\\s\\S]*?)\\n```")
	matches := fencedPattern.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		language := strings.TrimSpace(match[1])
		code := strings.TrimSpace(match[2])
		if code != "" {
			blocks = append(blocks, CodeBlock{
				Language: language,
				Code:     code,
			})
		}
	}

	// Pattern for indented code blocks (4+ spaces)
	if len(blocks) == 0 {
		lines := strings.Split(text, "\n")
		var codeLines []string
		inCodeBlock := false

		for _, line := range lines {
			if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
				// This is a code line
				if !inCodeBlock {
					inCodeBlock = true
					codeLines = []string{}
				}
				codeLines = append(codeLines, strings.TrimPrefix(strings.TrimPrefix(line, "    "), "\t"))
			} else {
				// This is not a code line
				if inCodeBlock {
					// End of code block
					code := strings.TrimSpace(strings.Join(codeLines, "\n"))
					if code != "" {
						blocks = append(blocks, CodeBlock{
							Language: "",
							Code:     code,
						})
					}
					inCodeBlock = false
				}
			}
		}

		// Handle case where file ends with code block
		if inCodeBlock {
			code := strings.TrimSpace(strings.Join(codeLines, "\n"))
			if code != "" {
				blocks = append(blocks, CodeBlock{
					Language: "",
					Code:     code,
				})
			}
		}
	}

	return blocks
}

// ExtractCodeFromText extracts the first code block from text
func ExtractCodeFromText(text string) string {
	blocks := ParseCodeBlobs(text)
	if len(blocks) > 0 {
		return blocks[0].Code
	}
	return ""
}

// TruncateContent truncates content to a maximum length with ellipsis
func TruncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	if maxLength <= 3 {
		return content[:maxLength]
	}

	return content[:maxLength-3] + "..."
}

// IsValidName checks if a string is a valid Go identifier
func IsValidName(name string) bool {
	if name == "" {
		return false
	}

	// Check if it's a valid Go identifier
	if !token.IsIdentifier(name) {
		return false
	}

	// Check for Go keywords
	goKeywords := map[string]bool{
		"break": true, "case": true, "chan": true, "const": true, "continue": true,
		"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
		"func": true, "go": true, "goto": true, "if": true, "import": true,
		"interface": true, "map": true, "package": true, "range": true, "return": true,
		"select": true, "struct": true, "switch": true, "type": true, "var": true,
	}

	return !goKeywords[name]
}

// MakeJSONSerializable converts an object to a JSON-serializable format
func MakeJSONSerializable(obj interface{}) interface{} {
	switch v := obj.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = MakeJSONSerializable(val)
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = MakeJSONSerializable(val)
		}
		return result

	case string, int, int64, float64, bool, nil:
		return v

	default:
		// Convert unknown types to string representation
		return fmt.Sprintf("%v", v)
	}
}

// StringToLines splits a string into lines, handling different line endings
func StringToLines(text string) []string {
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	return strings.Split(text, "\n")
}

// JoinLines joins lines with the system's line separator
func JoinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// CleanWhitespace removes excessive whitespace from text
func CleanWhitespace(text string) string {
	// Replace multiple consecutive whitespace characters with a single space
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	// Trim leading and trailing whitespace
	return strings.TrimSpace(text)
}

// ExtractVariableName extracts a valid variable name from a string
func ExtractVariableName(text string) string {
	// Remove non-alphanumeric characters except underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	cleaned := re.ReplaceAllString(text, "_")

	// Ensure it starts with a letter or underscore
	if len(cleaned) > 0 && unicode.IsDigit(rune(cleaned[0])) {
		cleaned = "_" + cleaned
	}

	// Ensure it's not empty
	if cleaned == "" {
		cleaned = "var"
	}

	return cleaned
}

// ContainsAny checks if a string contains any of the given substrings
func ContainsAny(text string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(text, substr) {
			return true
		}
	}
	return false
}

// SanitizeForLogging sanitizes content for safe logging (removes sensitive data patterns)
func SanitizeForLogging(content string) string {
	// Remove potential API keys, tokens, etc.
	patterns := []string{
		`sk-[a-zA-Z0-9]+`,            // OpenAI-style API keys
		`Bearer [a-zA-Z0-9_-]+`,      // Bearer tokens
		`token[=:]\s*[a-zA-Z0-9_-]+`, // Generic tokens
		`key[=:]\s*[a-zA-Z0-9_-]+`,   // Generic keys
	}

	result := content
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		result = re.ReplaceAllString(result, "[REDACTED]")
	}

	return result
}

// FormatErrorContext formats an error with context information
func FormatErrorContext(err error, context map[string]interface{}) string {
	var parts []string
	parts = append(parts, err.Error())

	if context != nil && len(context) > 0 {
		parts = append(parts, "Context:")
		for k, v := range context {
			parts = append(parts, fmt.Sprintf("  %s: %v", k, v))
		}
	}

	return strings.Join(parts, "\n")
}

// SafeStringConversion safely converts an interface{} to string
func SafeStringConversion(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ValidateMapKeys checks if all required keys are present in a map
func ValidateMapKeys(data map[string]interface{}, required []string) error {
	for _, key := range required {
		if _, exists := data[key]; !exists {
			return fmt.Errorf("required key '%s' is missing", key)
		}
	}
	return nil
}

// MergeStringMaps merges multiple string maps, with later maps taking precedence
func MergeStringMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)

	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}

	return result
}

// FilterMap filters a map based on a predicate function
func FilterMap(data map[string]interface{}, predicate func(string, interface{}) bool) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range data {
		if predicate(k, v) {
			result[k] = v
		}
	}

	return result
}

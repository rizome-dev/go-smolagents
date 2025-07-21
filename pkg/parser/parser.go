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

package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ParseResult represents the result of parsing an LLM response
type ParseResult struct {
	Type        string                 // "code", "action", "structured", "final_answer", "error"
	Thought     string                 // The reasoning/thought if present
	Content     string                 // The main content (code or action)
	Action      map[string]interface{} // Parsed action for tool calling
	Error       error                  // Any parsing error
	IsStreaming bool                   // Whether this is a streaming response
}

// Parser handles parsing of LLM responses
type Parser struct {
	codeBlockTags  [2]string
	actionPattern  *regexp.Regexp
	thoughtPattern *regexp.Regexp
	jsonPattern    *regexp.Regexp
	errorPattern   *regexp.Regexp
}

// NewParser creates a new parser with default settings
func NewParser() *Parser {
	return NewParserWithTags("<code>", "</code>")
}

// NewParserWithTags creates a new parser with custom code block tags
func NewParserWithTags(openTag, closeTag string) *Parser {
	// Escape special regex characters in tags
	escapedOpen := regexp.QuoteMeta(openTag)

	return &Parser{
		codeBlockTags:  [2]string{openTag, closeTag},
		actionPattern:  regexp.MustCompile(`(?s)Action:\s*\n*\s*(?:` + "`" + `{3}json\s*\n)?(.*?)(?:\n` + "`" + `{3}|\s*$)`),
		thoughtPattern: regexp.MustCompile(`(?s)Thought:\s*(.+?)(?:\n(?:Action:|` + escapedOpen + `)|$)`),
		jsonPattern:    regexp.MustCompile(`(?s)` + "`" + `{3}json\s*\n(.*?)\n` + "`" + `{3}|(\{[^}]+\})`),
		errorPattern:   regexp.MustCompile(`(?s)Error:\s*(.+?)(?:\nNow let's retry:|$)`),
	}
}

// Parse analyzes an LLM response and extracts structured information
func (p *Parser) Parse(response string) *ParseResult {
	// Defensive programming - handle nil/empty responses
	if response == "" {
		return &ParseResult{
			Type:    "error",
			Content: "empty response",
			Error:   fmt.Errorf("received empty response from model"),
		}
	}

	// Trim whitespace for cleaner parsing
	response = strings.TrimSpace(response)

	// Check for error first
	if errorMatch := p.errorPattern.FindStringSubmatch(response); len(errorMatch) > 1 {
		return &ParseResult{
			Type:    "error",
			Content: strings.TrimSpace(errorMatch[1]),
		}
	}

	// Try to extract code blocks first
	code := p.ExtractCode(response)
	
	// Extract thought - either from explicit "Thought:" prefix or from text before code
	thought := ""
	if thoughtMatch := p.thoughtPattern.FindStringSubmatch(response); len(thoughtMatch) > 1 {
		thought = strings.TrimSpace(thoughtMatch[1])
	} else if code != "" {
		// If we have code but no explicit thought pattern, treat everything before the code as thought
		codeStartIdx := strings.Index(response, p.codeBlockTags[0])
		if codeStartIdx > 0 {
			thought = strings.TrimSpace(response[:codeStartIdx])
			// Clean up the thought - remove any trailing punctuation or incomplete sentences
			thought = cleanThought(thought)
		}
	} else {
		// No code found, try to extract implicit thought patterns
		thought = extractImplicitThought(response)
	}

	// If we found code blocks, process them
	if code != "" {
		// Validate the code is not just whitespace
		if strings.TrimSpace(code) == "" {
			return &ParseResult{
				Type:    "error",
				Thought: thought,
				Content: "empty code block",
				Error:   fmt.Errorf("code block contains only whitespace"),
			}
		}

		// Always treat code blocks as code that needs execution
		// The final_answer() function will be handled during execution
		return &ParseResult{
			Type:    "code",
			Thought: thought,
			Content: code,
		}
	}

	// Check for action format (tool calling)
	if actionMatch := p.actionPattern.FindStringSubmatch(response); len(actionMatch) > 1 {
		actionJSON := actionMatch[1]
		// Clean up JSON if it's in code block
		actionJSON = p.cleanJSON(actionJSON)

		var action map[string]interface{}
		if err := json.Unmarshal([]byte(actionJSON), &action); err != nil {
			return &ParseResult{
				Type:    "error",
				Thought: thought,
				Error:   fmt.Errorf("failed to parse action JSON: %w", err),
			}
		}

		// Validate action has required fields
		if _, hasName := action["name"]; !hasName {
			return &ParseResult{
				Type:    "error",
				Thought: thought,
				Error:   fmt.Errorf("action missing 'name' field"),
			}
		}

		// Check if it's a final answer action
		if name, ok := action["name"].(string); ok && name == "final_answer" {
			return &ParseResult{
				Type:    "final_answer",
				Thought: thought,
				Content: actionJSON,
				Action:  action,
			}
		}

		return &ParseResult{
			Type:    "action",
			Thought: thought,
			Content: actionJSON,
			Action:  action,
		}
	}

	// Check for structured JSON response
	if jsonMatch := p.jsonPattern.FindStringSubmatch(response); len(jsonMatch) > 0 {
		jsonStr := p.cleanJSON(jsonMatch[0])
		var structured map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &structured); err == nil {
			// Check if it has thought and code fields (structured code agent)
			if _, hasThought := structured["thought"]; hasThought {
				if _, hasCode := structured["code"]; hasCode {
					return &ParseResult{
						Type:    "structured",
						Thought: getString(structured, "thought"),
						Content: getString(structured, "code"),
						Action:  structured,
					}
				}
			}
		}
	}

	// If nothing matched, analyze the content more carefully
	// Check if it might be a final answer in plain text
	if looksLikeFinalAnswer(response) {
		return &ParseResult{
			Type:    "final_answer",
			Thought: thought,
			Content: extractFinalAnswerText(response),
		}
	}

	// Return as raw response with any extracted thought
	return &ParseResult{
		Type:    "raw",
		Thought: thought,
		Content: strings.TrimSpace(response),
	}
}

// ExtractCode extracts code from between code block tags
func (p *Parser) ExtractCode(text string) string {
	// First try to find complete code blocks
	pattern := fmt.Sprintf(`(?s)%s(.*?)%s`,
		regexp.QuoteMeta(p.codeBlockTags[0]),
		regexp.QuoteMeta(p.codeBlockTags[1]))


	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(text, -1)

	if len(matches) > 0 {
		// Join all code blocks with newlines
		var codes []string
		for _, match := range matches {
			if len(match) > 1 {
				codes = append(codes, strings.TrimSpace(match[1]))
			}
		}
		return strings.Join(codes, "\n\n")
	}

	// Fallback to markdown pattern if no matches found (similar to Python implementation)
	markdownPattern := `(?s)` + "```(?:go|golang)?\\s*\n(.*?)\n```"
	markdownRe := regexp.MustCompile(markdownPattern)
	markdownMatches := markdownRe.FindAllStringSubmatch(text, -1)
	
	if len(markdownMatches) > 0 {
		// Join all markdown code blocks with newlines
		var codes []string
		for _, match := range markdownMatches {
			if len(match) > 1 {
				codes = append(codes, strings.TrimSpace(match[1]))
			}
		}
		return strings.Join(codes, "\n\n")
	}

	// If no complete blocks found, check for incomplete blocks (missing closing tag)
	// This handles cases where the LLM stops generating before the closing tag
	openTag := p.codeBlockTags[0]
	if idx := strings.LastIndex(text, openTag); idx >= 0 {
		// Extract everything after the opening tag
		code := text[idx+len(openTag):]
		// Trim any trailing whitespace
		code = strings.TrimSpace(code)
		if code != "" {
			return code
		}
	}

	// Also check for incomplete markdown blocks
	if idx := strings.LastIndex(text, "```"); idx >= 0 {
		// Check if it's the start of a code block
		afterTicks := text[idx+3:]
		// Skip language identifier if present
		lines := strings.Split(afterTicks, "\n")
		if len(lines) > 0 {
			firstLine := strings.TrimSpace(lines[0])
			if firstLine == "go" || firstLine == "golang" || firstLine == "" {
				// It's a code block start
				if len(lines) > 1 {
					code := strings.Join(lines[1:], "\n")
					code = strings.TrimSpace(code)
					if code != "" {
						return code
					}
				}
			}
		}
	}

	return ""
}

// ExtractAction extracts action JSON from text
func (p *Parser) ExtractAction(text string) (map[string]interface{}, error) {
	// First try the action pattern
	if actionMatch := p.actionPattern.FindStringSubmatch(text); len(actionMatch) > 1 {
		actionJSON := p.cleanJSON(actionMatch[1])
		var action map[string]interface{}
		if err := json.Unmarshal([]byte(actionJSON), &action); err != nil {
			// Try to find JSON in code blocks as fallback
			jsonMatch := p.jsonPattern.FindStringSubmatch(text)
			if len(jsonMatch) > 1 {
				// Try first capture group (```json blocks)
				jsonStr := jsonMatch[1]
				if jsonStr == "" && len(jsonMatch) > 2 {
					// Try second capture group (inline JSON)
					jsonStr = jsonMatch[2]
				}
				jsonStr = p.cleanJSON(jsonStr)
				if err := json.Unmarshal([]byte(jsonStr), &action); err != nil {
					return nil, err
				}
				return action, nil
			}
			return nil, err
		}
		return action, nil
	}
	return nil, fmt.Errorf("no action found in text")
}

// ExtractThought extracts the thought/reasoning from text
func (p *Parser) ExtractThought(text string) string {
	if thoughtMatch := p.thoughtPattern.FindStringSubmatch(text); len(thoughtMatch) > 1 {
		return strings.TrimSpace(thoughtMatch[1])
	}
	return ""
}

// ExtractError extracts error message from text
func (p *Parser) ExtractError(text string) string {
	if errorMatch := p.errorPattern.FindStringSubmatch(text); len(errorMatch) > 1 {
		return strings.TrimSpace(errorMatch[1])
	}
	return ""
}

// ParseStructuredOutput parses a structured JSON response
func (p *Parser) ParseStructuredOutput(text string) (map[string]interface{}, error) {
	// Try to find JSON in code blocks first
	jsonMatch := p.jsonPattern.FindStringSubmatch(text)
	if len(jsonMatch) == 0 {
		// Try parsing the whole text as JSON
		jsonMatch = []string{text}
	}

	for _, match := range jsonMatch {
		jsonStr := p.cleanJSON(match)
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("no valid JSON found in text")
}

// IsFinalAnswer checks if the parsed result is a final answer
func (p *Parser) IsFinalAnswer(result *ParseResult) bool {
	if result.Type == "final_answer" {
		return true
	}

	// Check for final_answer in code
	if result.Type == "code" && strings.Contains(result.Content, "final_answer(") {
		return true
	}

	// Check for final_answer in action
	if result.Type == "action" && result.Action != nil {
		if name, ok := result.Action["name"].(string); ok && name == "final_answer" {
			return true
		}
	}

	return false
}

// GetCodeBlockTags returns the current code block tags
func (p *Parser) GetCodeBlockTags() [2]string {
	return p.codeBlockTags
}

// cleanJSON cleans up JSON strings by removing code block markers
func (p *Parser) cleanJSON(jsonStr string) string {
	// Remove code block markers
	jsonStr = strings.TrimSpace(jsonStr)
	if strings.HasPrefix(jsonStr, "```json") {
		jsonStr = strings.TrimPrefix(jsonStr, "```json")
		jsonStr = strings.TrimSuffix(jsonStr, "```")
		jsonStr = strings.TrimSpace(jsonStr)
	}
	return jsonStr
}

// getString safely gets a string value from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Helper functions for robust parsing

// cleanThought cleans up extracted thought text
func cleanThought(thought string) string {
	// Remove incomplete sentences at the end
	if idx := strings.LastIndex(thought, "."); idx > 0 && idx < len(thought)-10 {
		thought = thought[:idx+1]
	}
	// Remove any code-like patterns that leaked in
	thought = strings.TrimSuffix(thought, "<code")
	thought = strings.TrimSuffix(thought, "```")
	return strings.TrimSpace(thought)
}

// extractImplicitThought tries to extract thought patterns from unstructured text
func extractImplicitThought(text string) string {
	// Common thought indicators
	patterns := []string{
		"I need to", "I should", "I'll", "I will", "Let me", "Let's",
		"First,", "Next,", "Now,", "Then,",
		"To solve", "To answer", "To complete",
		"The task", "The problem", "The solution",
	}
	
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(trimmed), strings.ToLower(pattern)) {
				return trimmed
			}
		}
	}
	
	// Return first substantial line if no pattern found
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 20 && !strings.HasPrefix(trimmed, "<") {
			return trimmed
		}
	}
	
	return ""
}

// isFinalAnswerCode checks if code contains a final answer call
func isFinalAnswerCode(code string) bool {
	// Normalize whitespace for better detection
	normalized := strings.ReplaceAll(code, " ", "")
	return strings.Contains(normalized, "final_answer(") ||
		strings.Contains(code, "final_answer (") ||
		strings.Contains(code, "finalAnswer(") ||
		strings.Contains(code, "FinalAnswer(")
}

// looksLikeFinalAnswer checks if text looks like a final answer statement
func looksLikeFinalAnswer(text string) bool {
	lower := strings.ToLower(text)
	finalAnswerPhrases := []string{
		"the answer is",
		"the final answer is",
		"therefore, the answer",
		"in conclusion,",
		"the result is",
		"final result:",
		"answer:",
	}
	
	for _, phrase := range finalAnswerPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// extractFinalAnswerText extracts the answer portion from text
func extractFinalAnswerText(text string) string {
	lower := strings.ToLower(text)
	markers := []string{
		"the answer is",
		"the final answer is",
		"therefore, the answer",
		"the result is",
		"answer:",
	}
	
	for _, marker := range markers {
		if idx := strings.Index(lower, marker); idx >= 0 {
			result := text[idx+len(marker):]
			return strings.TrimSpace(result)
		}
	}
	
	return text
}

// StreamParser handles parsing of streaming responses
type StreamParser struct {
	parser      *Parser
	buffer      strings.Builder
	inCodeBlock bool
	thoughtSeen bool
}

// NewStreamParser creates a new streaming parser
func NewStreamParser(parser *Parser) *StreamParser {
	return &StreamParser{
		parser: parser,
	}
}

// ParseChunk parses a streaming chunk
func (sp *StreamParser) ParseChunk(chunk string) *ParseResult {
	sp.buffer.WriteString(chunk)
	current := sp.buffer.String()

	// Check if we're in a code block
	openTag := sp.parser.codeBlockTags[0]
	closeTag := sp.parser.codeBlockTags[1]

	if !sp.inCodeBlock && strings.Contains(current, openTag) {
		sp.inCodeBlock = true
	}

	if sp.inCodeBlock && strings.Contains(current, closeTag) {
		// We have a complete code block
		result := sp.parser.Parse(current)
		result.IsStreaming = true
		return result
	}

	// Check for thought pattern
	if !sp.thoughtSeen && strings.Contains(current, "Thought:") {
		sp.thoughtSeen = true
	}

	// Return partial result
	return &ParseResult{
		Type:        "streaming",
		Content:     current,
		IsStreaming: true,
	}
}

// Reset resets the stream parser
func (sp *StreamParser) Reset() {
	sp.buffer.Reset()
	sp.inCodeBlock = false
	sp.thoughtSeen = false
}

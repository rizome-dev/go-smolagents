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
	"strings"
	"testing"
)

func TestParserCodeExtraction(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected string
		hasCode  bool
	}{
		{
			name: "simple code block",
			input: `Thought: I need to calculate 2 + 2
<code>
result = 2 + 2
print(result)
</code>`,
			expected: "result = 2 + 2\nprint(result)",
			hasCode:  true,
		},
		{
			name: "multiple code blocks",
			input: `First block:
<code>
x = 1
</code>
Second block:
<code>
y = 2
</code>`,
			expected: "x = 1\n\ny = 2",
			hasCode:  true,
		},
		{
			name:     "no code blocks",
			input:    "This is just text without code",
			expected: "",
			hasCode:  false,
		},
		{
			name: "code with final_answer",
			input: `<code>
final_answer("The result is 4")
</code>`,
			expected: `final_answer("The result is 4")`,
			hasCode:  true,
		},
		{
			name: "incomplete code block (missing closing tag)",
			input: `Thought: Let me calculate this
<code>
package main

import "fmt"

func main() {
    result := 2 + 2
    fmt.Println(result)
}`,
			expected: `package main

import "fmt"

func main() {
    result := 2 + 2
    fmt.Println(result)
}`,
			hasCode:  true,
		},
		{
			name: "multiple incomplete code blocks",
			input: `First block:
<code>
x := 1`,
			expected: "x := 1",
			hasCode:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := parser.ExtractCode(tt.input)
			if tt.hasCode && code == "" {
				t.Error("Expected code but got none")
			}
			if !tt.hasCode && code != "" {
				t.Errorf("Expected no code but got: %s", code)
			}
			if tt.hasCode && code != tt.expected {
				t.Errorf("Expected code:\n%s\nGot:\n%s", tt.expected, code)
			}
		})
	}
}

func TestParserThoughtExtraction(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "thought before code",
			input: `Thought: I need to calculate something
<code>
x = 1
</code>`,
			expected: "I need to calculate something",
		},
		{
			name: "thought before action",
			input: `Thought: I should search for information
Action: {"name": "search", "arguments": "query"}`,
			expected: "I should search for information",
		},
		{
			name:     "no thought",
			input:    `<code>x = 1</code>`,
			expected: "",
		},
		{
			name: "multiline thought",
			input: `Thought: This is a complex task.
I need to break it down into steps.
First, I'll calculate the sum.
<code>
sum = 0
</code>`,
			expected: "This is a complex task.\nI need to break it down into steps.\nFirst, I'll calculate the sum.",
		},
		{
			name: "implicit thought before code without prefix",
			input: `Let me calculate 2 + 2 for you.
<code>
result = 2 + 2
print(result)
</code>`,
			expected: "",  // ExtractThought only gets explicit "Thought:" patterns
		},
		{
			name: "implicit thought in Parse result",
			input: `Sure, let's execute the Go code to calculate 2 + 2 and print the result.
<code>
package main

import "fmt"

func main() {
    result := 2 + 2
    fmt.Println(result)
}
</code>`,
			expected: "",  // ExtractThought only gets explicit "Thought:" patterns
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thought := parser.ExtractThought(tt.input)
			if thought != tt.expected {
				t.Errorf("Expected thought:\n%s\nGot:\n%s", tt.expected, thought)
			}
		})
	}
}

func TestParserActionExtraction(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		input       string
		expectError bool
		checkAction func(action map[string]interface{}) bool
	}{
		{
			name:        "simple action",
			input:       `Action: {"name": "web_search", "arguments": "weather Paris"}`,
			expectError: false,
			checkAction: func(action map[string]interface{}) bool {
				return action["name"] == "web_search" &&
					action["arguments"] == "weather Paris"
			},
		},
		{
			name: "action in code block",
			input: `Action:
` + "```json" + `
{"name": "calculator", "arguments": {"a": 2, "b": 3, "operation": "add"}}
` + "```",
			expectError: false,
			checkAction: func(action map[string]interface{}) bool {
				return action["name"] == "calculator"
			},
		},
		{
			name:        "no action",
			input:       "Just some text",
			expectError: true,
		},
		{
			name:        "final_answer action",
			input:       `Action: {"name": "final_answer", "arguments": "The result is 42"}`,
			expectError: false,
			checkAction: func(action map[string]interface{}) bool {
				return action["name"] == "final_answer"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := parser.ExtractAction(tt.input)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.expectError && tt.checkAction != nil && !tt.checkAction(action) {
				t.Errorf("Action validation failed: %+v", action)
			}
		})
	}
}

func TestParserParse(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name         string
		input        string
		expectedType string
		checkResult  func(result *ParseResult) bool
	}{
		{
			name: "code response",
			input: `Thought: Let me calculate this
<code>
result = 2 + 2
print(f"The answer is {result}")
</code>`,
			expectedType: "code",
			checkResult: func(result *ParseResult) bool {
				return result.Thought == "Let me calculate this" &&
					strings.Contains(result.Content, "result = 2 + 2")
			},
		},
		{
			name: "action response",
			input: `Thought: I need to search for information
Action: {"name": "web_search", "arguments": "Python tutorials"}`,
			expectedType: "action",
			checkResult: func(result *ParseResult) bool {
				return result.Thought == "I need to search for information" &&
					result.Action["name"] == "web_search"
			},
		},
		{
			name:         "structured response",
			input:        `{"thought": "I need to process this data", "code": "data = [1, 2, 3]\nresult = sum(data)"}`,
			expectedType: "structured",
			checkResult: func(result *ParseResult) bool {
				return result.Thought == "I need to process this data" &&
					strings.Contains(result.Content, "sum(data)")
			},
		},
		{
			name: "final answer code",
			input: `<code>
final_answer("The result is 42")
</code>`,
			expectedType: "code", // Code blocks are always parsed as code type
			checkResult: func(result *ParseResult) bool {
				return strings.Contains(result.Content, "final_answer")
			},
		},
		{
			name:         "final answer action",
			input:        `Action: {"name": "final_answer", "arguments": "The answer is 42"}`,
			expectedType: "final_answer",
			checkResult: func(result *ParseResult) bool {
				return result.Action["name"] == "final_answer"
			},
		},
		{
			name: "error response",
			input: `Error: Division by zero occurred
Now let's retry: take care not to repeat previous errors!`,
			expectedType: "error",
			checkResult: func(result *ParseResult) bool {
				return result.Content == "Division by zero occurred"
			},
		},
		{
			name:         "raw response",
			input:        "Just some plain text without any structure",
			expectedType: "raw",
			checkResult: func(result *ParseResult) bool {
				return result.Content == "Just some plain text without any structure"
			},
		},
		{
			name: "code with implicit thought (no Thought: prefix)",
			input: `Sure, let's execute the Go code to calculate 2 + 2 and print the result.
<code>
package main

import "fmt"

func main() {
    result := 2 + 2
    fmt.Println(result)
}
</code>`,
			expectedType: "code",
			checkResult: func(result *ParseResult) bool {
				return result.Thought == "Sure, let's execute the Go code to calculate 2 + 2 and print the result." &&
					strings.Contains(result.Content, "result := 2 + 2")
			},
		},
		{
			name: "code with implicit multiline thought",
			input: `I'll help you solve this problem.
First, I need to set up the variables.
Then I'll calculate the result.
<code>
x = 10
y = 20
result = x + y
</code>`,
			expectedType: "code",
			checkResult: func(result *ParseResult) bool {
				expectedThought := "I'll help you solve this problem.\nFirst, I need to set up the variables.\nThen I'll calculate the result."
				return result.Thought == expectedThought &&
					strings.Contains(result.Content, "result = x + y")
			},
		},
		{
			name: "code immediately without text",
			input: `<code>
print("Hello")
</code>`,
			expectedType: "code",
			checkResult: func(result *ParseResult) bool {
				return result.Thought == "" &&
					strings.Contains(result.Content, "print(\"Hello\")")
			},
		},
		{
			name: "raw response with no code returns full text as thought",
			input: `I understand you want to calculate 2 + 2. Let me think about this.`,
			expectedType: "raw",
			checkResult: func(result *ParseResult) bool {
				return result.Thought == "I understand you want to calculate 2 + 2. Let me think about this." &&
					result.Content == "I understand you want to calculate 2 + 2. Let me think about this."
			},
		},
		{
			name: "response with 'the answer is' detected as final answer",
			input: `I understand you want to calculate 2 + 2. The answer is 4.`,
			expectedType: "final_answer",
			checkResult: func(result *ParseResult) bool {
				return result.Content == "4."
			},
		},
		{
			name: "incomplete code block (missing closing tag)",
			input: `Thought: Let's execute the code we've written to calculate 2 + 2 and print the result.
<code>
package main

import "fmt"

func main() {
    result := 2 + 2
    fmt.Println(result)
}`,
			expectedType: "code",
			checkResult: func(result *ParseResult) bool {
				return result.Thought == "Let's execute the code we've written to calculate 2 + 2 and print the result." &&
					strings.Contains(result.Content, "result := 2 + 2")
			},
		},
		{
			name: "incomplete code without explicit thought",
			input: `Great! Let's execute the code to calculate 2 + 2.
<code>
result := 2 + 2
fmt.Println(result)`,
			expectedType: "code",
			checkResult: func(result *ParseResult) bool {
				return result.Thought == "Great! Let's execute the code to calculate 2 + 2." &&
					strings.Contains(result.Content, "result := 2 + 2")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.input)
			if result.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, result.Type)
			}
			if tt.checkResult != nil && !tt.checkResult(result) {
				t.Errorf("Result validation failed: %+v", result)
			}
		})
	}
}

func TestParserCustomTags(t *testing.T) {
	parser := NewParserWithTags("```go", "```")

	input := `Thought: Custom tags test
` + "```go" + `
x := 42
fmt.Println(x)
` + "```"

	result := parser.Parse(input)
	if result.Type != "code" {
		t.Errorf("Expected type 'code', got %s", result.Type)
	}
	if !strings.Contains(result.Content, "x := 42") {
		t.Errorf("Code not extracted properly: %s", result.Content)
	}
}

func TestParserIsFinalAnswer(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		result   *ParseResult
		expected bool
	}{
		{
			name:     "final_answer type",
			result:   &ParseResult{Type: "final_answer"},
			expected: true,
		},
		{
			name:     "code with final_answer",
			result:   &ParseResult{Type: "code", Content: `final_answer("result")`},
			expected: true,
		},
		{
			name: "action with final_answer",
			result: &ParseResult{
				Type:   "action",
				Action: map[string]interface{}{"name": "final_answer"},
			},
			expected: true,
		},
		{
			name:     "regular code",
			result:   &ParseResult{Type: "code", Content: "x = 1"},
			expected: false,
		},
		{
			name:     "regular action",
			result:   &ParseResult{Type: "action", Action: map[string]interface{}{"name": "search"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isFinal := parser.IsFinalAnswer(tt.result)
			if isFinal != tt.expected {
				t.Errorf("Expected IsFinalAnswer=%v, got %v", tt.expected, isFinal)
			}
		})
	}
}

func TestStreamParser(t *testing.T) {
	parser := NewParser()
	streamParser := NewStreamParser(parser)

	// Simulate streaming response
	chunks := []string{
		"Thought: I need to ",
		"calculate something\n",
		"<code>\n",
		"result = 2 + 2\n",
		"print(result)\n",
		"</code>",
	}

	var lastResult *ParseResult
	for _, chunk := range chunks {
		result := streamParser.ParseChunk(chunk)
		lastResult = result
		if !result.IsStreaming {
			t.Error("Expected streaming result")
		}
	}

	// The last result should have the complete code
	if lastResult.Type != "code" {
		t.Errorf("Expected final type 'code', got %s", lastResult.Type)
	}
	if !strings.Contains(lastResult.Content, "result = 2 + 2") {
		t.Error("Code not properly extracted from stream")
	}

	// Test reset
	streamParser.Reset()
	result := streamParser.ParseChunk("New content")
	if strings.Contains(result.Content, "result = 2 + 2") {
		t.Error("Stream parser not properly reset")
	}
}

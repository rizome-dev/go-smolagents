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

// Package display provides beautiful CLI output formatting for smolagents
package display

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// captureOutput captures stdout during test execution
type testWriter struct {
	buffer bytes.Buffer
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	return tw.buffer.Write(p)
}

func (tw *testWriter) String() string {
	return tw.buffer.String()
}

// TestDisplay tests all display methods
func TestDisplay(t *testing.T) {
	// Create display with verbose mode
	d := New(true)
	// We'll test that methods run without errors and don't panic

	t.Run("Rule", func(t *testing.T) {
		// Test rule without title
		d.Rule("")

		// Test rule with title
		d.Rule("Test Section")
	})

	t.Run("Task", func(t *testing.T) {
		d.Task("Calculate 2 + 2", "A simple arithmetic task")
		d.Task("Complex calculation", "")
	})

	t.Run("Step", func(t *testing.T) {
		d.Step(1, "react_code_step")
		d.Step(2, "planning_step")
	})

	t.Run("Thought", func(t *testing.T) {
		d.Thought("I need to calculate the sum of 2 + 2")
		d.Thought("") // Should not print anything
		d.Thought("This is a multiline thought.\nIt spans multiple lines.\nAnd continues here.")
	})

	t.Run("Code", func(t *testing.T) {
		d.Code("Executing Go code:", `package main

import "fmt"

func main() {
    result := 2 + 2
    fmt.Println(result)
}`)

		d.Code("", "x := 42")
	})

	t.Run("Observation", func(t *testing.T) {
		d.Observation("Output: 4")
		d.Observation("") // Should not print anything
		d.Observation("Multiple line output:\nLine 1\nLine 2")
	})

	t.Run("Error", func(t *testing.T) {
		d.Error(errors.New("compilation error: undefined variable"))
		d.Error(errors.New("runtime error: index out of bounds"))
	})

	t.Run("Success", func(t *testing.T) {
		d.Success("Task completed successfully!")
		d.Success("All tests passed")
	})

	t.Run("Info", func(t *testing.T) {
		d.Info("Starting execution...")
		d.Info("Parse result type: code")

		// Test with verbose off
		dQuiet := New(false)
		dQuiet.Info("This should not be displayed")
	})

	t.Run("Progress", func(t *testing.T) {
		d.Progress("Loading model...")
		d.Progress("Processing request...")
	})

	t.Run("FinalAnswer", func(t *testing.T) {
		d.FinalAnswer("The result is 42")
		d.FinalAnswer(map[string]interface{}{
			"answer":      42,
			"explanation": "The meaning of life",
		})
	})

	t.Run("Metrics", func(t *testing.T) {
		d.Metrics(1, 1500*time.Millisecond, 250, 180)

		// Test with verbose off
		dQuiet := New(false)
		dQuiet.Metrics(2, 2*time.Second, 300, 200) // Should not display
	})

	t.Run("ModelOutput", func(t *testing.T) {
		d.ModelOutput("Raw LLM response: Thinking about the problem...")

		// Test with verbose off
		dQuiet := New(false)
		dQuiet.ModelOutput("This should not be displayed")
	})

	t.Run("Planning", func(t *testing.T) {
		d.Planning("Step 1: Analyze the problem\nStep 2: Break it down\nStep 3: Solve each part")
	})

	t.Run("Table", func(t *testing.T) {
		headers := []string{"Name", "Type", "Value"}
		rows := [][]string{
			{"x", "int", "42"},
			{"y", "string", "hello"},
			{"result", "bool", "true"},
		}
		d.Table(headers, rows)
	})

	t.Run("Clear", func(t *testing.T) {
		// Just test it doesn't panic
		d.Clear()
	})

	t.Run("Spinner", func(t *testing.T) {
		spinner := d.StartSpinner("Processing...")
		time.Sleep(10 * time.Millisecond)
		spinner.Stop()

		// Test multiple spinners
		s1 := d.StartSpinner("Loading...")
		s2 := d.StartSpinner("Computing...")
		time.Sleep(10 * time.Millisecond)
		s1.Stop()
		s2.Stop()
	})
}

// TestHelperFunctions tests the internal helper functions
func TestHelperFunctions(t *testing.T) {
	d := New(true)

	t.Run("indent", func(t *testing.T) {
		tests := []struct {
			input    string
			spaces   int
			expected string
		}{
			{
				input:    "Hello\nWorld",
				spaces:   2,
				expected: "  Hello\n  World",
			},
			{
				input:    "Single line",
				spaces:   4,
				expected: "    Single line",
			},
			{
				input:    "Line 1\n\nLine 3",
				spaces:   3,
				expected: "   Line 1\n\n   Line 3",
			},
		}

		for _, tt := range tests {
			result := d.indent(tt.input, tt.spaces)
			if result != tt.expected {
				t.Errorf("indent(%q, %d) = %q, want %q", tt.input, tt.spaces, result, tt.expected)
			}
		}
	})

	t.Run("codeBox", func(t *testing.T) {
		// Test short code
		box := d.codeBox("x := 42")
		if !strings.Contains(box, "x := 42") {
			t.Error("Code box should contain the code")
		}
		if !strings.Contains(box, "┌") || !strings.Contains(box, "└") {
			t.Error("Code box should have borders")
		}

		// Test multiline code
		code := "func main() {\n    fmt.Println(\"Hello\")\n}"
		box = d.codeBox(code)
		if !strings.Contains(box, "func main()") {
			t.Error("Code box should contain multiline code")
		}
	})

	t.Run("truncate", func(t *testing.T) {
		tests := []struct {
			input    string
			maxLen   int
			expected string
		}{
			{
				input:    "Short text",
				maxLen:   20,
				expected: "Short text",
			},
			{
				input:    "This is a very long text that needs truncation",
				maxLen:   20,
				expected: "This is a very lo...",
			},
			{
				input:    "Exact",
				maxLen:   5,
				expected: "Exact",
			},
			{
				input:    "TooLong",
				maxLen:   5,
				expected: "To...",
			},
		}

		for _, tt := range tests {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		}
	})
}

// TestColorOutput tests that colors are properly applied
func TestColorOutput(t *testing.T) {
	d := New(true)

	// Since we can't easily capture colored output, we'll just ensure methods don't panic
	// In a real test, you might want to check for ANSI color codes

	t.Run("ColoredMethods", func(t *testing.T) {
		// These should all use colors without panicking
		d.Rule("Colored Title")
		d.Task("Colored Task", "With subtitle")
		d.Thought("Colored thought")
		d.Code("Colored code", "print('hello')")
		d.Observation("Colored observation")
		d.Error(errors.New("colored error"))
		d.Success("Colored success")
		d.Info("Colored info")
		d.Progress("Colored progress")
		d.FinalAnswer("Colored final answer")
		d.Planning("Colored plan")
	})
}

// TestEdgeCases tests edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	d := New(true)

	t.Run("EmptyInputs", func(t *testing.T) {
		// All of these should handle empty inputs gracefully
		d.Rule("")
		d.Task("", "")
		d.Thought("")
		d.Code("", "")
		d.Observation("")
		d.Success("")
		d.Info("")
		d.Progress("")
		d.Planning("")
		d.ModelOutput("")
	})

	t.Run("VeryLongInputs", func(t *testing.T) {
		longText := strings.Repeat("A very long line of text. ", 100)

		// These should handle long inputs without issues
		d.Rule(longText[:50]) // Rule titles should be reasonable
		d.Task(longText, longText)
		d.Thought(longText)
		d.Code("Long code", longText)
		d.Observation(longText)
		d.Error(errors.New(longText))
		d.Success(longText)
		d.Info(longText)
		d.Progress(longText)
		d.Planning(longText)
		d.ModelOutput(longText)
	})

	t.Run("SpecialCharacters", func(t *testing.T) {
		specialText := "Text with special chars: \n\t\r\b and unicode: 中文 😊 🚀"

		d.Thought(specialText)
		d.Code("Special chars", specialText)
		d.Observation(specialText)
		d.Success(specialText)
	})

	t.Run("NilError", func(t *testing.T) {
		// Should handle nil error gracefully
		var nilErr error
		d.Error(nilErr) // Should show "<nil>" or similar
	})
}

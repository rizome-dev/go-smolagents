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
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Colors for consistent theming
var (
	yellowColor  = color.New(color.FgYellow, color.Bold)
	blueColor    = color.New(color.FgBlue)
	greenColor   = color.New(color.FgGreen)
	redColor     = color.New(color.FgRed, color.Bold)
	cyanColor    = color.New(color.FgCyan)
	magentaColor = color.New(color.FgMagenta)
	whiteColor   = color.New(color.FgWhite)
	dimColor     = color.New(color.Faint)
	boldColor    = color.New(color.Bold)
)

// Display handles all CLI output formatting
type Display struct {
	verbose bool
	width   int
}

// New creates a new Display instance (deprecated - use NewCharmDisplay)
func New(verbose bool) *Display {
	return &Display{
		verbose: verbose,
		width:   80, // Default terminal width
	}
}

// Rule prints a horizontal rule with optional title
func (d *Display) Rule(title string) {
	ruleChar := "━"
	if title == "" {
		fmt.Println(yellowColor.Sprint(strings.Repeat(ruleChar, d.width)))
		return
	}

	// Calculate padding for centered title
	titleWithSpaces := fmt.Sprintf(" %s ", title)
	titleLen := len(titleWithSpaces)
	leftPadding := (d.width - titleLen) / 2
	rightPadding := d.width - titleLen - leftPadding

	// Ensure we have at least some rule characters
	if leftPadding < 3 {
		leftPadding = 3
		rightPadding = 3
	}

	fmt.Println(yellowColor.Sprint(
		strings.Repeat(ruleChar, leftPadding) +
			boldColor.Sprint(titleWithSpaces) +
			strings.Repeat(ruleChar, rightPadding),
	))
}

// Task prints a task header with title and subtitle
func (d *Display) Task(title, subtitle string) {
	d.Rule("")
	fmt.Println()
	fmt.Println(boldColor.Sprint("Task: ") + whiteColor.Sprint(title))
	if subtitle != "" {
		fmt.Println(dimColor.Sprint(subtitle))
	}
	fmt.Println()
}

// Step prints a step header
func (d *Display) Step(stepNumber int, stepType string) {
	timestamp := time.Now().Format("15:04:05")
	stepText := fmt.Sprintf("Step %d: %s", stepNumber, stepType)
	fmt.Println()
	d.Rule(stepText)
	fmt.Println(dimColor.Sprintf("[%s]", timestamp))
}

// Thought prints the agent's thinking process
func (d *Display) Thought(content string) {
	if content == "" {
		return
	}
	fmt.Println()
	fmt.Println(magentaColor.Sprint("💭 Thought:"))
	fmt.Println(d.indent(content, 3))
}

// Code prints a code block with syntax highlighting
func (d *Display) Code(title, code string) {
	fmt.Println()
	if title != "" {
		fmt.Println(cyanColor.Sprint("📝 " + title))
	}

	// Create a box around the code
	fmt.Println(d.codeBox(code))
}

// Observation prints execution results
func (d *Display) Observation(content string) {
	if content == "" {
		return
	}
	fmt.Println()
	fmt.Println(greenColor.Sprint("👁  Observation:"))
	fmt.Println(d.indent(content, 3))
}

// Error prints an error message
func (d *Display) Error(err error) {
	if err == nil {
		return
	}
	fmt.Println()
	fmt.Println(redColor.Sprint("❌ Error:"))

	// In verbose mode, try to extract and format detailed error info
	if d.verbose {
		errStr := err.Error()
		// Check if it's a model error with structured info
		if strings.Contains(errStr, "[Model:") && strings.Contains(errStr, "]") {
			// Extract and format model info
			fmt.Println(d.indent(errStr, 3))

			// Add debugging suggestions
			fmt.Println()
			fmt.Println(dimColor.Sprint("   💡 Debug tips:"))
			if strings.Contains(errStr, "status 401") {
				fmt.Println(dimColor.Sprint("      - Check your API token (HF_TOKEN environment variable)"))
			} else if strings.Contains(errStr, "status 404") {
				fmt.Println(dimColor.Sprint("      - Verify the model ID exists and you have access"))
			} else if strings.Contains(errStr, "status 429") {
				fmt.Println(dimColor.Sprint("      - Rate limit exceeded, wait a moment before retrying"))
			} else if strings.Contains(errStr, "empty content") || strings.Contains(errStr, "nil response") {
				fmt.Println(dimColor.Sprint("      - Model returned empty response, try a different model"))
				fmt.Println(dimColor.Sprint("      - Set SMOLAGENTS_DEBUG=true for detailed API logs"))
			}
		} else {
			fmt.Println(d.indent(errStr, 3))
		}
	} else {
		fmt.Println(d.indent(err.Error(), 3))
	}
}

// Success prints a success message
func (d *Display) Success(message string) {
	fmt.Println()
	fmt.Println(greenColor.Sprint("✅ " + message))
}

// Info prints an info message
func (d *Display) Info(message string) {
	if d.verbose {
		fmt.Println(blueColor.Sprint("ℹ  " + message))
	}
}

// Progress prints a progress indicator
func (d *Display) Progress(message string) {
	fmt.Println(dimColor.Sprint("⏳ " + message))
}

// FinalAnswer prints the final answer
func (d *Display) FinalAnswer(answer interface{}) {
	fmt.Println()
	d.Rule("Final Answer")
	fmt.Println(greenColor.Sprint("🎯 Result:"))
	fmt.Println(d.indent(fmt.Sprintf("%v", answer), 3))
	d.Rule("")
}

// Metrics prints token usage and timing information
func (d *Display) Metrics(stepNumber int, duration time.Duration, inputTokens, outputTokens int) {
	if !d.verbose {
		return
	}
	fmt.Println()
	metricsText := fmt.Sprintf(
		"[Step %d: Duration %.2fs | Input tokens: %d | Output tokens: %d]",
		stepNumber, duration.Seconds(), inputTokens, outputTokens,
	)
	fmt.Println(dimColor.Sprint(metricsText))
}

// ModelOutput prints the raw model output (for debugging)
func (d *Display) ModelOutput(content string) {
	if !d.verbose {
		return
	}
	fmt.Println()
	fmt.Println(yellowColor.Sprint("🤖 Model Output:"))
	fmt.Println(d.indent(content, 3))
}

// Planning prints a planning step
func (d *Display) Planning(plan string) {
	fmt.Println()
	fmt.Println(yellowColor.Sprint("📋 Planning:"))
	fmt.Println(d.indent(plan, 3))
}

// Helper functions

// indent adds spaces to the beginning of each line
func (d *Display) indent(content string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// codeBox creates a simple box around code
func (d *Display) codeBox(code string) string {
	lines := strings.Split(code, "\n")
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// Ensure minimum width
	if maxLen < 40 {
		maxLen = 40
	}

	var result strings.Builder

	// Top border
	result.WriteString(dimColor.Sprint("┌" + strings.Repeat("─", maxLen+2) + "┐\n"))

	// Code lines
	for _, line := range lines {
		padding := maxLen - len(line)
		result.WriteString(dimColor.Sprint("│ "))
		result.WriteString(cyanColor.Sprint(line))
		result.WriteString(strings.Repeat(" ", padding))
		result.WriteString(dimColor.Sprint(" │\n"))
	}

	// Bottom border
	result.WriteString(dimColor.Sprint("└" + strings.Repeat("─", maxLen+2) + "┘"))

	return result.String()
}

// Clear clears the screen (optional)
func (d *Display) Clear() {
	fmt.Print("\033[H\033[2J")
}

// Spinner represents a loading spinner
type Spinner struct {
	message string
	stopped bool
}

// StartSpinner starts a loading spinner
func (d *Display) StartSpinner(message string) *Spinner {
	spinner := &Spinner{
		message: message,
		stopped: false,
	}

	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for !spinner.stopped {
			fmt.Printf("\r%s %s", cyanColor.Sprint(frames[i]), message)
			i = (i + 1) % len(frames)
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Print("\r" + strings.Repeat(" ", len(message)+3) + "\r")
	}()

	return spinner
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	s.stopped = true
	time.Sleep(150 * time.Millisecond) // Give time for cleanup
}

// Table creates a simple table
func (d *Display) Table(headers []string, rows [][]string) {
	// Calculate column widths
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Print headers
	for i, header := range headers {
		fmt.Print(boldColor.Sprintf("%-*s", colWidths[i]+2, header))
	}
	fmt.Println()

	// Print separator
	for _, width := range colWidths {
		fmt.Print(strings.Repeat("─", width+2))
	}
	fmt.Println()

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) {
				fmt.Printf("%-*s", colWidths[i]+2, cell)
			}
		}
		fmt.Println()
	}
}

// truncate limits string length with ellipsis
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

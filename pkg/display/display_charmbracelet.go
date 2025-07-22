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

// Package display provides beautiful CLI output formatting for smolagents using Charmbracelet libraries
package display

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// CharmDisplay handles all CLI output formatting with Charmbracelet libraries
type CharmDisplay struct {
	verbose     bool
	width       int
	styles      *DisplayStyles
	renderer    *glamour.TermRenderer
	termProfile termenv.Profile
}

// DisplayStyles holds all the lipgloss styles
type DisplayStyles struct {
	Rule         lipgloss.Style
	Task         lipgloss.Style
	TaskSubtitle lipgloss.Style
	Step         lipgloss.Style
	Thought      lipgloss.Style
	Code         lipgloss.Style
	CodeBox      lipgloss.Style
	Observation  lipgloss.Style
	Error        lipgloss.Style
	ErrorTip     lipgloss.Style
	Success      lipgloss.Style
	Info         lipgloss.Style
	Progress     lipgloss.Style
	FinalAnswer  lipgloss.Style
	Planning     lipgloss.Style
	Timestamp    lipgloss.Style
}

// NewCharmDisplay creates a new display instance with Charmbracelet styling
func NewCharmDisplay(verbose bool) *CharmDisplay {
	width := getTerminalWidth()

	// Create glamour renderer for markdown
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)

	// Detect color profile
	profile := termenv.ColorProfile()

	// Create styles
	styles := createStyles(profile)

	return &CharmDisplay{
		verbose:     verbose,
		width:       width,
		styles:      styles,
		renderer:    renderer,
		termProfile: profile,
	}
}

// createStyles creates all the lipgloss styles
func createStyles(profile termenv.Profile) *DisplayStyles {
	// Define colors based on terminal capabilities
	yellow := lipgloss.AdaptiveColor{Light: "#FFD700", Dark: "#FFD700"}
	blue := lipgloss.AdaptiveColor{Light: "#0066CC", Dark: "#5B9BD5"}
	green := lipgloss.AdaptiveColor{Light: "#008000", Dark: "#50C878"}
	red := lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF6B6B"}
	cyan := lipgloss.AdaptiveColor{Light: "#006B8E", Dark: "#4DD0E1"}
	magenta := lipgloss.AdaptiveColor{Light: "#8B008B", Dark: "#BA68C8"}
	gray := lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"}

	return &DisplayStyles{
		Rule: lipgloss.NewStyle().
			Foreground(yellow).
			Bold(true).
			Align(lipgloss.Center),

		Task: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")),

		TaskSubtitle: lipgloss.NewStyle().
			Foreground(gray).
			Italic(true),

		Step: lipgloss.NewStyle().
			Foreground(yellow).
			Bold(true),

		Thought: lipgloss.NewStyle().
			Foreground(magenta).
			PaddingLeft(3),

		Code: lipgloss.NewStyle().
			Foreground(cyan),

		CodeBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cyan).
			Padding(0, 1),

		Observation: lipgloss.NewStyle().
			Foreground(green).
			PaddingLeft(3),

		Error: lipgloss.NewStyle().
			Foreground(red).
			Bold(true),

		ErrorTip: lipgloss.NewStyle().
			Foreground(gray).
			PaddingLeft(6),

		Success: lipgloss.NewStyle().
			Foreground(green).
			Bold(true),

		Info: lipgloss.NewStyle().
			Foreground(blue),

		Progress: lipgloss.NewStyle().
			Foreground(gray).
			Italic(true),

		FinalAnswer: lipgloss.NewStyle().
			Foreground(green).
			Bold(true).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(green).
			Padding(1, 2),

		Planning: lipgloss.NewStyle().
			Foreground(yellow).
			PaddingLeft(3),

		Timestamp: lipgloss.NewStyle().
			Foreground(gray).
			Faint(true),
	}
}

// UpdateWidth updates the terminal width
func (d *CharmDisplay) UpdateWidth() {
	d.width = getTerminalWidth()
	// Update glamour renderer
	if renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(d.width-4),
	); err == nil {
		d.renderer = renderer
	}
}

// Rule prints a horizontal rule with optional title
func (d *CharmDisplay) Rule(title string) {
	d.UpdateWidth() // Check for terminal resize

	ruleChar := "━"
	ruleWidth := d.width

	if title == "" {
		rule := strings.Repeat(ruleChar, ruleWidth)
		fmt.Println(d.styles.Rule.Render(rule))
		return
	}

	// Calculate padding for centered title
	titleWithSpaces := fmt.Sprintf(" %s ", title)
	titleLen := lipgloss.Width(titleWithSpaces)
	remainingWidth := ruleWidth - titleLen
	leftPadding := remainingWidth / 2
	rightPadding := remainingWidth - leftPadding

	// Ensure we have at least some rule characters
	if leftPadding < 3 {
		leftPadding = 3
		rightPadding = 3
	}

	rule := strings.Repeat(ruleChar, leftPadding) +
		d.styles.Step.Render(titleWithSpaces) +
		strings.Repeat(ruleChar, rightPadding)

	fmt.Println(d.styles.Rule.Render(rule))
}

// Task prints a task header with title and subtitle
func (d *CharmDisplay) Task(title, subtitle string) {
	d.Rule("")
	fmt.Println()
	fmt.Println(d.styles.Task.Render("Task: ") + d.styles.Task.Copy().Bold(false).Render(title))
	if subtitle != "" {
		fmt.Println(d.styles.TaskSubtitle.Render(subtitle))
	}
	fmt.Println()
}

// Step prints a step header
func (d *CharmDisplay) Step(stepNumber int, stepType string) {
	timestamp := time.Now().Format("15:04:05")
	stepText := fmt.Sprintf("Step %d", stepNumber)
	fmt.Println()
	d.Rule(stepText)
	fmt.Println(d.styles.Timestamp.Render(fmt.Sprintf("[%s]", timestamp)))
}

// Thought prints the agent's thinking process
func (d *CharmDisplay) Thought(content string) {
	if content == "" {
		return
	}
	fmt.Println()
	fmt.Println(d.styles.Thought.Copy().PaddingLeft(0).Render("💭 Thought:"))
	fmt.Println(d.styles.Thought.Render(content))
}

// Code prints a code block with syntax highlighting
func (d *CharmDisplay) Code(title, code string) {
	fmt.Println()
	if title != "" {
		// Don't add emoji if title already has one
		if !strings.Contains(title, "📝") {
			fmt.Println(d.styles.Code.Render("📝 " + title))
		} else {
			fmt.Println(d.styles.Code.Render(title))
		}
	}

	// Render code in a box
	codeBox := d.styles.CodeBox.Render(code)
	fmt.Println(codeBox)
}

// Observation prints execution results
func (d *CharmDisplay) Observation(content string) {
	if content == "" {
		return
	}
	fmt.Println()
	fmt.Println(d.styles.Observation.Copy().PaddingLeft(0).Render("👁  Observation:"))
	fmt.Println(d.styles.Observation.Render(content))
}

// Error prints an error message
func (d *CharmDisplay) Error(err error) {
	if err == nil {
		return
	}
	fmt.Println()
	fmt.Println(d.styles.Error.Render("❌ Error:"))
	errStr := err.Error()
	fmt.Println(d.styles.Error.Copy().Bold(false).PaddingLeft(3).Render(errStr))

	// In verbose mode, add debugging tips
	if d.verbose {
		d.addErrorTips(errStr)
	}
}

// addErrorTips adds context-specific debugging tips
func (d *CharmDisplay) addErrorTips(errStr string) {
	fmt.Println()
	fmt.Println(d.styles.ErrorTip.Copy().PaddingLeft(3).Render("💡 Debug tips:"))

	if strings.Contains(errStr, "status 401") {
		fmt.Println(d.styles.ErrorTip.Render("- Check your API token (HF_TOKEN environment variable)"))
	} else if strings.Contains(errStr, "status 404") {
		fmt.Println(d.styles.ErrorTip.Render("- Verify the model ID exists and you have access"))
	} else if strings.Contains(errStr, "status 429") {
		fmt.Println(d.styles.ErrorTip.Render("- Rate limit exceeded, wait a moment before retrying"))
	} else if strings.Contains(errStr, "empty content") || strings.Contains(errStr, "nil response") {
		fmt.Println(d.styles.ErrorTip.Render("- Model returned empty response, try a different model"))
		fmt.Println(d.styles.ErrorTip.Render("- Set SMOLAGENTS_DEBUG=true for detailed API logs"))
	}
}

// Success prints a success message
func (d *CharmDisplay) Success(message string) {
	fmt.Println()
	fmt.Println(d.styles.Success.Render("✅ " + message))
}

// Info prints an info message
func (d *CharmDisplay) Info(message string) {
	if d.verbose {
		fmt.Println(d.styles.Info.Render("ℹ  " + message))
	}
}

// Progress prints a progress indicator
func (d *CharmDisplay) Progress(message string) {
	fmt.Println(d.styles.Progress.Render("⏳ " + message))
}

// FinalAnswer prints the final answer
func (d *CharmDisplay) FinalAnswer(answer interface{}) {
	fmt.Println()
	d.Rule("Final Answer")
	answerStr := fmt.Sprintf("%v", answer)
	finalBox := d.styles.FinalAnswer.Render("🎯 Result:\n" + answerStr)
	fmt.Println(finalBox)
	d.Rule("")
}

// Metrics prints token usage and timing information
func (d *CharmDisplay) Metrics(stepNumber int, duration time.Duration, inputTokens, outputTokens int) {
	if !d.verbose {
		return
	}
	fmt.Println()
	metricsText := fmt.Sprintf(
		"[Step %d: Duration %.2fs | Input tokens: %d | Output tokens: %d]",
		stepNumber, duration.Seconds(), inputTokens, outputTokens,
	)
	fmt.Println(d.styles.Timestamp.Render(metricsText))
}

// ModelOutput prints the raw model output (for debugging)
func (d *CharmDisplay) ModelOutput(content string) {
	if !d.verbose {
		return
	}
	fmt.Println()
	fmt.Println(d.styles.Info.Render("🤖 Model Output:"))

	// Try to render as markdown
	if rendered, err := d.renderer.Render(content); err == nil {
		fmt.Print(rendered)
	} else {
		// Fallback to plain text
		fmt.Println(d.styles.Info.Copy().PaddingLeft(3).Render(content))
	}
}

// Planning prints a planning step
func (d *CharmDisplay) Planning(plan string) {
	fmt.Println()
	fmt.Println(d.styles.Planning.Copy().PaddingLeft(0).Render("📋 Planning:"))
	fmt.Println(d.styles.Planning.Render(plan))
}

// getTerminalWidth returns the current terminal width
func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 40 {
		return 80 // Default width
	}
	return width
}

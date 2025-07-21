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

// Package monitoring provides logging and monitoring capabilities for agent execution.
//
// This includes token usage tracking, timing information, structured logging,
// and execution monitoring for debugging and observability.
package monitoring

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// LogLevel represents different log levels
type LogLevel int

const (
	LogLevelOFF LogLevel = iota - 1
	LogLevelERROR
	LogLevelINFO
	LogLevelDEBUG
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelOFF:
		return "OFF"
	case LogLevelERROR:
		return "ERROR"
	case LogLevelINFO:
		return "INFO"
	case LogLevelDEBUG:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// Color constants for rich output
const (
	ResetColor  = "\033[0m"
	RedColor    = "\033[31m"
	GreenColor  = "\033[32m"
	YellowColor = "\033[33m"
	BlueColor   = "\033[34m"
	PurpleColor = "\033[35m"
	CyanColor   = "\033[36m"
	WhiteColor  = "\033[37m"

	// HEX color constants
	YELLOW_HEX = "#FFD700"
)

// TokenUsage represents token usage statistics
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// NewTokenUsage creates a new TokenUsage instance
func NewTokenUsage(inputTokens, outputTokens int) *TokenUsage {
	return &TokenUsage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	}
}

// Add combines this token usage with another
func (tu *TokenUsage) Add(other *TokenUsage) {
	if other != nil {
		tu.InputTokens += other.InputTokens
		tu.OutputTokens += other.OutputTokens
		tu.TotalTokens += other.TotalTokens
	}
}

// String returns a string representation of token usage
func (tu *TokenUsage) String() string {
	return fmt.Sprintf("TokenUsage(input=%d, output=%d, total=%d)",
		tu.InputTokens, tu.OutputTokens, tu.TotalTokens)
}

// Timing represents timing information for operations
type Timing struct {
	StartTime time.Time      `json:"start_time"`
	EndTime   *time.Time     `json:"end_time,omitempty"`
	Duration  *time.Duration `json:"duration,omitempty"`
}

// NewTiming creates a new timing instance with start time set to now
func NewTiming() *Timing {
	return &Timing{
		StartTime: time.Now(),
	}
}

// End marks the end time and calculates duration
func (t *Timing) End() {
	now := time.Now()
	t.EndTime = &now
	duration := now.Sub(t.StartTime)
	t.Duration = &duration
}

// GetDuration returns the duration, calculating it if not already done
func (t *Timing) GetDuration() time.Duration {
	if t.Duration != nil {
		return *t.Duration
	}

	if t.EndTime != nil {
		return t.EndTime.Sub(t.StartTime)
	}

	return time.Since(t.StartTime)
}

// String returns a string representation of timing
func (t *Timing) String() string {
	duration := t.GetDuration()
	return fmt.Sprintf("Timing(duration=%v)", duration.Truncate(time.Millisecond))
}

// AgentLogger provides structured logging for agent operations
type AgentLogger struct {
	level      LogLevel
	output     io.Writer
	useColors  bool
	prefix     string
	timestamps bool
}

// NewAgentLogger creates a new agent logger
func NewAgentLogger(level LogLevel) *AgentLogger {
	return &AgentLogger{
		level:      level,
		output:     os.Stdout,
		useColors:  true,
		timestamps: true,
	}
}

// SetOutput sets the output writer for the logger
func (al *AgentLogger) SetOutput(w io.Writer) {
	al.output = w
}

// SetUseColors enables or disables colored output
func (al *AgentLogger) SetUseColors(useColors bool) {
	al.useColors = useColors
}

// SetPrefix sets a prefix for all log messages
func (al *AgentLogger) SetPrefix(prefix string) {
	al.prefix = prefix
}

// SetTimestamps enables or disables timestamps in log messages
func (al *AgentLogger) SetTimestamps(timestamps bool) {
	al.timestamps = timestamps
}

// SetLevel sets the logging level
func (al *AgentLogger) SetLevel(level LogLevel) {
	al.level = level
}

// GetLevel returns the current logging level
func (al *AgentLogger) GetLevel() LogLevel {
	return al.level
}

// isEnabled checks if a log level is enabled
func (al *AgentLogger) isEnabled(level LogLevel) bool {
	return level <= al.level
}

// formatMessage formats a log message with color, timestamp, and prefix
func (al *AgentLogger) formatMessage(level LogLevel, message string) string {
	var parts []string

	// Add timestamp
	if al.timestamps {
		timestamp := time.Now().Format("15:04:05")
		parts = append(parts, timestamp)
	}

	// Add level with color
	levelStr := level.String()
	if al.useColors {
		switch level {
		case LogLevelERROR:
			levelStr = RedColor + levelStr + ResetColor
		case LogLevelINFO:
			levelStr = BlueColor + levelStr + ResetColor
		case LogLevelDEBUG:
			levelStr = CyanColor + levelStr + ResetColor
		}
	}
	parts = append(parts, fmt.Sprintf("[%s]", levelStr))

	// Add prefix
	if al.prefix != "" {
		parts = append(parts, al.prefix)
	}

	// Add message
	parts = append(parts, message)

	return strings.Join(parts, " ")
}

// log writes a log message if the level is enabled
func (al *AgentLogger) log(level LogLevel, message string) {
	if !al.isEnabled(level) {
		return
	}

	formatted := al.formatMessage(level, message)
	fmt.Fprintln(al.output, formatted)
}

// Error logs an error message
func (al *AgentLogger) Error(message string, args ...interface{}) {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	al.log(LogLevelERROR, message)
}

// Info logs an info message
func (al *AgentLogger) Info(message string, args ...interface{}) {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	al.log(LogLevelINFO, message)
}

// Debug logs a debug message
func (al *AgentLogger) Debug(message string, args ...interface{}) {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	al.log(LogLevelDEBUG, message)
}

// LogStep logs a step execution with formatted output
func (al *AgentLogger) LogStep(stepNumber int, stepType string, content string) {
	if !al.isEnabled(LogLevelINFO) {
		return
	}

	header := fmt.Sprintf("=== Step %d: %s ===", stepNumber, stepType)
	if al.useColors {
		header = YellowColor + header + ResetColor
	}

	al.Info(header)
	if content != "" {
		// Indent the content
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			al.Info("  " + line)
		}
	}
}

// LogToolCall logs a tool call
func (al *AgentLogger) LogToolCall(toolName string, args map[string]interface{}) {
	if !al.isEnabled(LogLevelINFO) {
		return
	}

	header := fmt.Sprintf("🔧 Tool Call: %s", toolName)
	if al.useColors {
		header = GreenColor + header + ResetColor
	}

	al.Info(header)
	for k, v := range args {
		al.Info("  %s: %v", k, v)
	}
}

// LogToolResult logs a tool result
func (al *AgentLogger) LogToolResult(toolName string, result interface{}, err error) {
	if !al.isEnabled(LogLevelINFO) {
		return
	}

	if err != nil {
		header := fmt.Sprintf("❌ Tool Error: %s", toolName)
		if al.useColors {
			header = RedColor + header + ResetColor
		}
		al.Info(header)
		al.Error("  %v", err)
	} else {
		header := fmt.Sprintf("✅ Tool Result: %s", toolName)
		if al.useColors {
			header = GreenColor + header + ResetColor
		}
		al.Info(header)
		al.Info("  %v", result)
	}
}

// LogTokenUsage logs token usage information
func (al *AgentLogger) LogTokenUsage(usage *TokenUsage) {
	if !al.isEnabled(LogLevelDEBUG) || usage == nil {
		return
	}

	al.Debug("Token Usage: input=%d, output=%d, total=%d",
		usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
}

// LogTiming logs timing information
func (al *AgentLogger) LogTiming(operation string, timing *Timing) {
	if !al.isEnabled(LogLevelDEBUG) || timing == nil {
		return
	}

	duration := timing.GetDuration()
	al.Debug("Timing [%s]: %v", operation, duration.Truncate(time.Millisecond))
}

// Monitor provides comprehensive monitoring for agent execution
type Monitor struct {
	logger            *AgentLogger
	startTime         time.Time
	stepDurations     []time.Duration
	totalInputTokens  int
	totalOutputTokens int
	totalTokens       int
	currentStepStart  time.Time
	enabled           bool
}

// NewMonitor creates a new monitor instance
func NewMonitor(logger *AgentLogger) *Monitor {
	if logger == nil {
		logger = NewAgentLogger(LogLevelINFO)
	}

	return &Monitor{
		logger:        logger,
		startTime:     time.Now(),
		stepDurations: make([]time.Duration, 0),
		enabled:       true,
	}
}

// SetEnabled enables or disables monitoring
func (m *Monitor) SetEnabled(enabled bool) {
	m.enabled = enabled
}

// IsEnabled returns whether monitoring is enabled
func (m *Monitor) IsEnabled() bool {
	return m.enabled
}

// GetLogger returns the logger instance
func (m *Monitor) GetLogger() *AgentLogger {
	return m.logger
}

// StartStep starts monitoring a new step
func (m *Monitor) StartStep(stepNumber int, stepType string) {
	if !m.enabled {
		return
	}

	m.currentStepStart = time.Now()
	m.logger.LogStep(stepNumber, stepType, "")
}

// EndStep ends monitoring of the current step
func (m *Monitor) EndStep() {
	if !m.enabled {
		return
	}

	if !m.currentStepStart.IsZero() {
		duration := time.Since(m.currentStepStart)
		m.stepDurations = append(m.stepDurations, duration)
		m.logger.LogTiming("Step", &Timing{
			StartTime: m.currentStepStart,
			Duration:  &duration,
		})
		m.currentStepStart = time.Time{}
	}
}

// LogToolCall logs a tool call
func (m *Monitor) LogToolCall(toolName string, args map[string]interface{}) {
	if !m.enabled {
		return
	}

	m.logger.LogToolCall(toolName, args)
}

// LogToolResult logs a tool result
func (m *Monitor) LogToolResult(toolName string, result interface{}, err error) {
	if !m.enabled {
		return
	}

	m.logger.LogToolResult(toolName, result, err)
}

// AddTokenUsage adds token usage to the total count
func (m *Monitor) AddTokenUsage(usage *TokenUsage) {
	if !m.enabled || usage == nil {
		return
	}

	m.totalInputTokens += usage.InputTokens
	m.totalOutputTokens += usage.OutputTokens
	m.totalTokens += usage.TotalTokens

	m.logger.LogTokenUsage(usage)
}

// GetTotalTokenUsage returns the total token usage
func (m *Monitor) GetTotalTokenUsage() *TokenUsage {
	return &TokenUsage{
		InputTokens:  m.totalInputTokens,
		OutputTokens: m.totalOutputTokens,
		TotalTokens:  m.totalTokens,
	}
}

// GetStepDurations returns all step durations
func (m *Monitor) GetStepDurations() []time.Duration {
	durations := make([]time.Duration, len(m.stepDurations))
	copy(durations, m.stepDurations)
	return durations
}

// GetTotalDuration returns the total execution duration
func (m *Monitor) GetTotalDuration() time.Duration {
	return time.Since(m.startTime)
}

// GetAverageStepDuration returns the average step duration
func (m *Monitor) GetAverageStepDuration() time.Duration {
	if len(m.stepDurations) == 0 {
		return 0
	}

	var total time.Duration
	for _, duration := range m.stepDurations {
		total += duration
	}

	return total / time.Duration(len(m.stepDurations))
}

// LogSummary logs a summary of the monitoring session
func (m *Monitor) LogSummary() {
	if !m.enabled {
		return
	}

	m.logger.Info("=== Execution Summary ===")
	m.logger.Info("Total Duration: %v", m.GetTotalDuration().Truncate(time.Millisecond))
	m.logger.Info("Total Steps: %d", len(m.stepDurations))

	if len(m.stepDurations) > 0 {
		m.logger.Info("Average Step Duration: %v", m.GetAverageStepDuration().Truncate(time.Millisecond))
	}

	if m.totalTokens > 0 {
		m.logger.Info("Total Tokens: %d (input: %d, output: %d)",
			m.totalTokens, m.totalInputTokens, m.totalOutputTokens)
	}
}

// Reset resets all monitoring data
func (m *Monitor) Reset() {
	m.startTime = time.Now()
	m.stepDurations = make([]time.Duration, 0)
	m.totalInputTokens = 0
	m.totalOutputTokens = 0
	m.totalTokens = 0
	m.currentStepStart = time.Time{}
}

// CreateChildLogger creates a child logger with a prefix
func (m *Monitor) CreateChildLogger(prefix string) *AgentLogger {
	childLogger := NewAgentLogger(m.logger.GetLevel())
	childLogger.SetOutput(m.logger.output)
	childLogger.SetUseColors(m.logger.useColors)
	childLogger.SetTimestamps(m.logger.timestamps)

	if m.logger.prefix != "" {
		childLogger.SetPrefix(m.logger.prefix + ":" + prefix)
	} else {
		childLogger.SetPrefix(prefix)
	}

	return childLogger
}

// Default logger instance
var defaultLogger = NewAgentLogger(LogLevelINFO)

// SetDefaultLogLevel sets the default log level for the package
func SetDefaultLogLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

// GetDefaultLogger returns the default logger instance
func GetDefaultLogger() *AgentLogger {
	return defaultLogger
}

package smolagentsgo

import "log/slog"

const (
	YELLOW_HEX = "#d4b702"
)

type (
	ToolCall struct {
		name      string
		arguments any
		id        string
	}

	AgentStep struct{}

	ActionStep struct {
		*AgentStep
		agentMemory  []map[string]string
		toolCalls    []*ToolCall
		startTime    float64
		endTime      float64
		step         int
		error        AgentError
		duration     float64
		llmOutput    string
		observations string
		actionOutput any
	}

	PlanningStep struct {
		*AgentStep
		plan  string
		facts string
	}

	TaskStep struct {
		*AgentStep
		task string
	}

	SystemPromptStep struct {
		*AgentStep
		systemPrompt string
	}

	LogLevel    int
	AgentLogger *slog.Logger
)

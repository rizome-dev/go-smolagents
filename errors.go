package smolagentsgo

import (
	"errors"
	"fmt"
)

var (
	AgentParsingError    = errors.New("Exception raised for errors in parsing in the agent")
	AgentExecutionError  = errors.New("Exception raised for errors in execution in the agent")
	AgentMaxStepsError   = errors.New("Exception raised for errors in agent max steps")
	AgentGenerationError = errors.New("Exception raised for errors in generation in the agent")
)

type AgentError struct {
	Err error
}

// Needs Error() to satisfy error interface
func (e *AgentError) Error() string {
	return fmt.Sprintf("%v: %v", e.Err, e.Err.Error())
}

// Unwrap is needed to support working with errors.Is & errors.As.
func (e *AgentError) Unwrap() error {
	// Return the inner error.
	return e.Err
}

// WrapTraderError to easily create a new error which wraps the given error.
func WrapTraderError(err error) error {
	return &AgentError{
		Err: err,
	}
}

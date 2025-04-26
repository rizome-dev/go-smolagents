// Package agents defines the interfaces for various agent components.
package agents

import (
	"context"
)

// CodeExecutor defines the interface for executing code in various languages.
type CodeExecutor interface {
	// Execute runs the provided code in the specified language and returns the output.
	Execute(ctx context.Context, code string, language string) (string, error)

	// SendVariables allows passing variables to the execution environment.
	SendVariables(variables map[string]interface{}) error
}

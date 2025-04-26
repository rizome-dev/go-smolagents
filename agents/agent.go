// Package agents defines the interfaces for various agent components.
package agents

// Agent defines the core interface that all agent implementations must satisfy.
type Agent interface {
	// Run executes the agent with the given input and returns the output.
	// The context can be used to cancel the execution.
	Run(input string) (interface{}, error)
}

// AgentFactory defines an interface for creating new agent instances.
type AgentFactory interface {
	// Create returns a new instance of the agent.
	Create() (Agent, error)
}

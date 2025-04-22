// Package agents provides agent implementations for various types of AI agents
package agents

import (
	"fmt"
	"image"
)

// ManagedAgent is an interface for agents that can be managed by a parent agent
type ManagedAgent interface {
	// Run executes the agent for a given task
	Run(task string, stream bool, reset bool, images []image.Image, additionalArgs map[string]interface{}, maxSteps int) (interface{}, error)

	// GetName returns the name of the agent
	GetName() string

	// GetDescription returns the description of the agent
	GetDescription() string
}

// ManagedAgentImpl implements the ManagedAgent interface
type ManagedAgentImpl struct {
	Agent       MultiStepAgent
	Name        string
	Description string
}

// NewManagedAgent creates a new ManagedAgent wrapping an agent
func NewManagedAgent(agent MultiStepAgent, name string, description string) (*ManagedAgentImpl, error) {
	if name == "" {
		return nil, fmt.Errorf("managed agent name cannot be empty")
	}

	if description == "" {
		return nil, fmt.Errorf("managed agent description cannot be empty")
	}

	return &ManagedAgentImpl{
		Agent:       agent,
		Name:        name,
		Description: description,
	}, nil
}

// Run executes the agent for a given task
func (m *ManagedAgentImpl) Run(task string, stream bool, reset bool, images []image.Image, additionalArgs map[string]interface{}, maxSteps int) (interface{}, error) {
	return m.Agent.Run(task, stream, reset, images, additionalArgs, maxSteps)
}

// GetName returns the name of the agent
func (m *ManagedAgentImpl) GetName() string {
	return m.Name
}

// GetDescription returns the description of the agent
func (m *ManagedAgentImpl) GetDescription() string {
	return m.Description
}

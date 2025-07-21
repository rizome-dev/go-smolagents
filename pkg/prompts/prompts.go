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

package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var promptFiles embed.FS

// PromptTemplate represents a loaded prompt template
type PromptTemplate struct {
	SystemPrompt      string                 `yaml:"system_prompt"`
	TaskPrompt        string                 `yaml:"task_prompt"`
	PlanningPrompt    string                 `yaml:"planning_prompt"`
	StepFormat        string                 `yaml:"step_format"`
	ErrorFormat       string                 `yaml:"error_format"`
	FinalAnswerFormat string                 `yaml:"final_answer_format"`
	ActionFormat      string                 `yaml:"action_format,omitempty"`
	DefaultVariables  map[string]interface{} `yaml:"default_variables"`
	StopSequences     []string               `yaml:"stop_sequences"`
	ResponseSchema    map[string]interface{} `yaml:"response_schema,omitempty"`
}

// PromptManager manages prompt templates
type PromptManager struct {
	templates map[string]*PromptTemplate
}

// NewPromptManager creates a new prompt manager
func NewPromptManager() (*PromptManager, error) {
	pm := &PromptManager{
		templates: make(map[string]*PromptTemplate),
	}

	// Load all embedded prompt files
	entries, err := promptFiles.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt files: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if len(name) > 5 && name[len(name)-5:] == ".yaml" {
			templateName := name[:len(name)-5]

			data, err := promptFiles.ReadFile(name)
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", name, err)
			}

			var tmpl PromptTemplate
			if err := yaml.Unmarshal(data, &tmpl); err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", name, err)
			}

			pm.templates[templateName] = &tmpl
		}
	}

	return pm, nil
}

// GetTemplate returns a prompt template by name
func (pm *PromptManager) GetTemplate(name string) (*PromptTemplate, error) {
	tmpl, ok := pm.templates[name]
	if !ok {
		return nil, fmt.Errorf("template %s not found", name)
	}
	return tmpl, nil
}

// RenderPrompt renders a prompt template with the given variables
func (pt *PromptTemplate) RenderPrompt(promptType string, variables map[string]interface{}) (string, error) {
	// Merge default variables with provided variables
	vars := make(map[string]interface{})
	for k, v := range pt.DefaultVariables {
		vars[k] = v
	}
	for k, v := range variables {
		vars[k] = v
	}

	var promptTemplate string
	switch promptType {
	case "system":
		promptTemplate = pt.SystemPrompt
	case "task":
		promptTemplate = pt.TaskPrompt
	case "planning":
		promptTemplate = pt.PlanningPrompt
	case "step":
		promptTemplate = pt.StepFormat
	case "error":
		promptTemplate = pt.ErrorFormat
	case "final_answer":
		promptTemplate = pt.FinalAnswerFormat
	case "action":
		promptTemplate = pt.ActionFormat
	default:
		return "", fmt.Errorf("unknown prompt type: %s", promptType)
	}

	if promptTemplate == "" {
		return "", fmt.Errorf("prompt type %s not defined in template", promptType)
	}

	// Render the template
	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}

// GetStopSequences returns the stop sequences with variables replaced
func (pt *PromptTemplate) GetStopSequences(variables map[string]interface{}) ([]string, error) {
	// Merge default variables with provided variables
	vars := make(map[string]interface{})
	for k, v := range pt.DefaultVariables {
		vars[k] = v
	}
	for k, v := range variables {
		vars[k] = v
	}

	stopSequences := make([]string, len(pt.StopSequences))
	for i, seq := range pt.StopSequences {
		tmpl, err := template.New("stop").Parse(seq)
		if err != nil {
			return nil, fmt.Errorf("failed to parse stop sequence template: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, vars); err != nil {
			return nil, fmt.Errorf("failed to render stop sequence: %w", err)
		}
		stopSequences[i] = buf.String()
	}

	return stopSequences, nil
}

// PromptBuilder helps build prompts for agents
type PromptBuilder struct {
	template  *PromptTemplate
	variables map[string]interface{}
}

// NewPromptBuilder creates a new prompt builder
func NewPromptBuilder(template *PromptTemplate) *PromptBuilder {
	return &PromptBuilder{
		template:  template,
		variables: make(map[string]interface{}),
	}
}

// WithVariable adds a variable to the prompt builder
func (pb *PromptBuilder) WithVariable(key string, value interface{}) *PromptBuilder {
	pb.variables[key] = value
	return pb
}

// WithVariables adds multiple variables to the prompt builder
func (pb *PromptBuilder) WithVariables(vars map[string]interface{}) *PromptBuilder {
	for k, v := range vars {
		pb.variables[k] = v
	}
	return pb
}

// BuildSystemPrompt builds the system prompt
func (pb *PromptBuilder) BuildSystemPrompt() (string, error) {
	return pb.template.RenderPrompt("system", pb.variables)
}

// BuildTaskPrompt builds the task prompt
func (pb *PromptBuilder) BuildTaskPrompt() (string, error) {
	return pb.template.RenderPrompt("task", pb.variables)
}

// BuildPlanningPrompt builds the planning prompt
func (pb *PromptBuilder) BuildPlanningPrompt() (string, error) {
	return pb.template.RenderPrompt("planning", pb.variables)
}

// BuildStepPrompt builds a step prompt
func (pb *PromptBuilder) BuildStepPrompt(thought, code, observation string) (string, error) {
	vars := make(map[string]interface{})
	for k, v := range pb.variables {
		vars[k] = v
	}
	vars["thought"] = thought
	vars["code"] = code
	vars["observation"] = observation
	return pb.template.RenderPrompt("step", vars)
}

// BuildErrorPrompt builds an error prompt
func (pb *PromptBuilder) BuildErrorPrompt(errorMsg string) (string, error) {
	vars := make(map[string]interface{})
	for k, v := range pb.variables {
		vars[k] = v
	}
	vars["error"] = errorMsg
	return pb.template.RenderPrompt("error", vars)
}

// BuildFinalAnswerPrompt builds a final answer prompt
func (pb *PromptBuilder) BuildFinalAnswerPrompt(answer string) (string, error) {
	vars := make(map[string]interface{})
	for k, v := range pb.variables {
		vars[k] = v
	}
	vars["answer"] = answer
	return pb.template.RenderPrompt("final_answer", vars)
}

// GetStopSequences returns the stop sequences for this prompt
func (pb *PromptBuilder) GetStopSequences() ([]string, error) {
	return pb.template.GetStopSequences(pb.variables)
}

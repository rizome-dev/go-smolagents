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
	"strings"
	"testing"
)

func TestPromptManager(t *testing.T) {
	pm, err := NewPromptManager()
	if err != nil {
		t.Fatalf("Failed to create prompt manager: %v", err)
	}

	// Test loading templates
	templates := []string{"code_agent", "toolcalling_agent", "structured_code_agent"}
	for _, name := range templates {
		tmpl, err := pm.GetTemplate(name)
		if err != nil {
			t.Errorf("Failed to get template %s: %v", name, err)
		}
		if tmpl == nil {
			t.Errorf("Template %s is nil", name)
		}
	}

	// Test non-existent template
	_, err = pm.GetTemplate("non_existent")
	if err == nil {
		t.Error("Expected error for non-existent template")
	}
}

func TestPromptTemplateRendering(t *testing.T) {
	pm, err := NewPromptManager()
	if err != nil {
		t.Fatalf("Failed to create prompt manager: %v", err)
	}

	tmpl, err := pm.GetTemplate("code_agent")
	if err != nil {
		t.Fatalf("Failed to get code_agent template: %v", err)
	}

	variables := map[string]interface{}{
		"task":              "Calculate 2 + 2",
		"tool_descriptions": "- calculator: Performs basic arithmetic",
		"memory":            "No previous steps.",
	}

	// Test system prompt
	systemPrompt, err := tmpl.RenderPrompt("system", variables)
	if err != nil {
		t.Errorf("Failed to render system prompt: %v", err)
	}
	if !strings.Contains(systemPrompt, "expert assistant") {
		t.Error("System prompt missing expected content")
	}

	// Test task prompt
	taskPrompt, err := tmpl.RenderPrompt("task", variables)
	if err != nil {
		t.Errorf("Failed to render task prompt: %v", err)
	}
	if !strings.Contains(taskPrompt, "Calculate 2 + 2") {
		t.Error("Task prompt missing task content")
	}

	// Test step format
	stepPrompt, err := tmpl.RenderPrompt("step", map[string]interface{}{
		"thought":                "I need to add 2 and 2",
		"code":                   "result = 2 + 2",
		"observation":            "4",
		"code_block_opening_tag": "<code>",
		"code_block_closing_tag": "</code>",
	})
	if err != nil {
		t.Errorf("Failed to render step prompt: %v", err)
	}
	if !strings.Contains(stepPrompt, "Thought: I need to add 2 and 2") {
		t.Error("Step prompt missing thought")
	}
	if !strings.Contains(stepPrompt, "<code>") {
		t.Error("Step prompt missing code tags")
	}
}

func TestPromptBuilder(t *testing.T) {
	pm, err := NewPromptManager()
	if err != nil {
		t.Fatalf("Failed to create prompt manager: %v", err)
	}

	tmpl, err := pm.GetTemplate("code_agent")
	if err != nil {
		t.Fatalf("Failed to get code_agent template: %v", err)
	}

	builder := NewPromptBuilder(tmpl).
		WithVariable("task", "Write a hello world program").
		WithVariable("tool_descriptions", "- go_interpreter: Execute Go code").
		WithVariable("memory", "")

	// Test building system prompt
	systemPrompt, err := builder.BuildSystemPrompt()
	if err != nil {
		t.Errorf("Failed to build system prompt: %v", err)
	}
	if systemPrompt == "" {
		t.Error("System prompt is empty")
	}

	// Test building task prompt
	taskPrompt, err := builder.BuildTaskPrompt()
	if err != nil {
		t.Errorf("Failed to build task prompt: %v", err)
	}
	if !strings.Contains(taskPrompt, "Write a hello world program") {
		t.Error("Task prompt missing task")
	}

	// Test stop sequences
	stopSeqs, err := builder.GetStopSequences()
	if err != nil {
		t.Errorf("Failed to get stop sequences: %v", err)
	}
	if len(stopSeqs) == 0 {
		t.Error("No stop sequences returned")
	}
	// Should contain "Observation:" and "</code>"
	hasObservation := false
	hasCodeTag := false
	for _, seq := range stopSeqs {
		if seq == "Observation:" {
			hasObservation = true
		}
		if seq == "</code>" {
			hasCodeTag = true
		}
	}
	if !hasObservation {
		t.Error("Stop sequences missing 'Observation:'")
	}
	if !hasCodeTag {
		t.Error("Stop sequences missing code closing tag")
	}
}

func TestToolCallingAgentPrompt(t *testing.T) {
	pm, err := NewPromptManager()
	if err != nil {
		t.Fatalf("Failed to create prompt manager: %v", err)
	}

	tmpl, err := pm.GetTemplate("toolcalling_agent")
	if err != nil {
		t.Fatalf("Failed to get toolcalling_agent template: %v", err)
	}

	// Test action format exists
	if tmpl.ActionFormat == "" {
		t.Error("ActionFormat should be defined for toolcalling_agent")
	}

	// Test rendering action format
	actionPrompt, err := tmpl.RenderPrompt("action", map[string]interface{}{
		"thought":     "I need to search for weather information",
		"action":      `{"name": "web_search", "arguments": "weather Paris"}`,
		"observation": "Current temperature in Paris is 15°C",
	})
	if err != nil {
		t.Errorf("Failed to render action prompt: %v", err)
	}
	if !strings.Contains(actionPrompt, "Thought:") {
		t.Error("Action prompt missing thought")
	}
	if !strings.Contains(actionPrompt, "Action:") {
		t.Error("Action prompt missing action")
	}
}

func TestStructuredCodeAgentPrompt(t *testing.T) {
	pm, err := NewPromptManager()
	if err != nil {
		t.Fatalf("Failed to create prompt manager: %v", err)
	}

	tmpl, err := pm.GetTemplate("structured_code_agent")
	if err != nil {
		t.Fatalf("Failed to get structured_code_agent template: %v", err)
	}

	// Test response schema exists
	if tmpl.ResponseSchema == nil {
		t.Error("ResponseSchema should be defined for structured_code_agent")
	}

	// Verify schema structure
	if schemaType, ok := tmpl.ResponseSchema["type"].(string); !ok || schemaType != "object" {
		t.Error("ResponseSchema should have type 'object'")
	}

	if props, ok := tmpl.ResponseSchema["properties"].(map[string]interface{}); ok {
		if _, hasThought := props["thought"]; !hasThought {
			t.Error("ResponseSchema should have 'thought' property")
		}
		if _, hasCode := props["code"]; !hasCode {
			t.Error("ResponseSchema should have 'code' property")
		}
	} else {
		t.Error("ResponseSchema should have properties")
	}
}

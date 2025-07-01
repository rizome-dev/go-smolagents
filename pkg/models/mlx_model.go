// Package models - MLXModel implementation for Apple Silicon
package models

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/monitoring"
)

// MLXModel represents a model using MLX for Apple Silicon
type MLXModel struct {
	*BaseModel
	ModelPath       string                 `json:"model_path"`
	TokenizerPath   string                 `json:"tokenizer_path"`
	TrustRemoteCode bool                   `json:"trust_remote_code"`
	MaxTokens       int                    `json:"max_tokens"`
	Temperature     float64                `json:"temperature"`
	TorchDtype      string                 `json:"torch_dtype"`
	DeviceMap       string                 `json:"device_map"`
	ModelKwargs     map[string]interface{} `json:"model_kwargs"`
	TokenizerKwargs map[string]interface{} `json:"tokenizer_kwargs"`
	mlxLMPath       string                 // Path to mlx-lm executable
	pythonPath      string                 // Path to Python executable
}

// NewMLXModel creates a new MLX model for Apple Silicon
func NewMLXModel(modelID string, options map[string]interface{}) *MLXModel {
	base := NewBaseModel(modelID, options)
	base.FlattenMessagesAsText = true // MLX doesn't support vision models

	model := &MLXModel{
		BaseModel:       base,
		ModelPath:       modelID, // Default to model ID as path
		TrustRemoteCode: false,
		MaxTokens:       2048,
		Temperature:     0.7,
		TorchDtype:      "auto",
		DeviceMap:       "auto",
		ModelKwargs:     make(map[string]interface{}),
		TokenizerKwargs: make(map[string]interface{}),
	}

	if options != nil {
		if modelPath, ok := options["model_path"].(string); ok {
			model.ModelPath = modelPath
		}
		if tokenizerPath, ok := options["tokenizer_path"].(string); ok {
			model.TokenizerPath = tokenizerPath
		}
		if trustRemoteCode, ok := options["trust_remote_code"].(bool); ok {
			model.TrustRemoteCode = trustRemoteCode
		}
		if maxTokens, ok := options["max_tokens"].(int); ok {
			model.MaxTokens = maxTokens
		}
		if temperature, ok := options["temperature"].(float64); ok {
			model.Temperature = temperature
		}
		if torchDtype, ok := options["torch_dtype"].(string); ok {
			model.TorchDtype = torchDtype
		}
		if deviceMap, ok := options["device_map"].(string); ok {
			model.DeviceMap = deviceMap
		}
		if modelKwargs, ok := options["model_kwargs"].(map[string]interface{}); ok {
			model.ModelKwargs = modelKwargs
		}
		if tokenizerKwargs, ok := options["tokenizer_kwargs"].(map[string]interface{}); ok {
			model.TokenizerKwargs = tokenizerKwargs
		}
	}

	// Initialize MLX-LM paths
	model.initializeMLXPaths()

	return model
}

// initializeMLXPaths sets up paths for MLX-LM and Python
func (mm *MLXModel) initializeMLXPaths() {
	// Check if we're on Apple Silicon
	if runtime.GOOS != "darwin" || (runtime.GOARCH != "arm64" && runtime.GOARCH != "amd64") {
		return
	}

	// Try to find mlx-lm in common locations
	mlxPaths := []string{
		"/opt/homebrew/bin/mlx_lm",
		"/usr/local/bin/mlx_lm",
		"mlx_lm", // In PATH
	}

	for _, path := range mlxPaths {
		if _, err := exec.LookPath(path); err == nil {
			mm.mlxLMPath = path
			break
		}
	}

	// Try to find Python with mlx-lm installed
	pythonPaths := []string{
		"/opt/homebrew/bin/python3",
		"/usr/local/bin/python3",
		"/usr/bin/python3",
		"python3", // In PATH
		"python",  // Fallback
	}

	for _, path := range pythonPaths {
		if _, err := exec.LookPath(path); err == nil {
			// Test if mlx-lm is available
			cmd := exec.Command(path, "-c", "import mlx_lm; print('OK')")
			if output, err := cmd.Output(); err == nil && strings.TrimSpace(string(output)) == "OK" {
				mm.pythonPath = path
				break
			}
		}
	}
}

// Generate implements Model interface
func (mm *MLXModel) Generate(messages []interface{}, options *GenerateOptions) (*ChatMessage, error) {
	// Check if MLX is available
	if !mm.isMLXAvailable() {
		return nil, fmt.Errorf("MLX-LM is not available. Please install it with: pip install mlx-lm")
	}

	// Check if we're on Apple Silicon
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("MLX is only supported on macOS (Apple Silicon)")
	}

	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, mm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
	}

	// Prepare the prompt using a simple chat template
	prompt, err := mm.formatMessagesAsPrompt(cleanMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Prepare generation parameters
	maxTokens := mm.MaxTokens
	temperature := mm.Temperature

	if options != nil {
		if options.MaxTokens != nil {
			maxTokens = *options.MaxTokens
		}
		if options.Temperature != nil {
			temperature = *options.Temperature
		}
	}

	// Generate response using MLX
	response, tokenCounts, err := mm.generateWithMLX(prompt, maxTokens, temperature, options)
	if err != nil {
		return nil, fmt.Errorf("MLX generation failed: %w", err)
	}

	// Create ChatMessage
	message := &ChatMessage{
		Role:       "assistant",
		Content:    &response,
		TokenUsage: monitoring.NewTokenUsage(tokenCounts["input"], tokenCounts["output"]),
		Raw: map[string]interface{}{
			"prompt":      prompt,
			"model_id":    mm.ModelID,
			"max_tokens":  maxTokens,
			"temperature": temperature,
		},
	}

	return message, nil
}

// GenerateStream implements Model interface
func (mm *MLXModel) GenerateStream(messages []interface{}, options *GenerateOptions) (<-chan *ChatMessageStreamDelta, error) {
	// Check if MLX is available
	if !mm.isMLXAvailable() {
		return nil, fmt.Errorf("MLX-LM is not available. Please install it with: pip install mlx-lm")
	}

	// Check if we're on Apple Silicon
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("MLX is only supported on macOS (Apple Silicon)")
	}

	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, mm.FlattenMessagesAsText)
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
	}

	// Prepare the prompt
	prompt, err := mm.formatMessagesAsPrompt(cleanMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Create the stream channel
	streamChan := make(chan *ChatMessageStreamDelta, 100)

	// Start streaming in a goroutine
	go func() {
		defer close(streamChan)

		maxTokens := mm.MaxTokens
		temperature := mm.Temperature

		if options != nil {
			if options.MaxTokens != nil {
				maxTokens = *options.MaxTokens
			}
			if options.Temperature != nil {
				temperature = *options.Temperature
			}
		}

		err := mm.generateStreamWithMLX(prompt, maxTokens, temperature, streamChan)
		if err != nil {
			// In a real implementation, you'd send an error delta
			return
		}
	}()

	return streamChan, nil
}

// SupportsStreaming implements Model interface
func (mm *MLXModel) SupportsStreaming() bool {
	return mm.isMLXAvailable()
}

// ToDict implements Model interface
func (mm *MLXModel) ToDict() map[string]interface{} {
	result := mm.BaseModel.ToDict()
	result["model_path"] = mm.ModelPath
	result["tokenizer_path"] = mm.TokenizerPath
	result["trust_remote_code"] = mm.TrustRemoteCode
	result["max_tokens"] = mm.MaxTokens
	result["temperature"] = mm.Temperature
	result["torch_dtype"] = mm.TorchDtype
	result["device_map"] = mm.DeviceMap
	result["model_kwargs"] = mm.ModelKwargs
	result["tokenizer_kwargs"] = mm.TokenizerKwargs
	return result
}

// isMLXAvailable checks if MLX-LM is available on the system
func (mm *MLXModel) isMLXAvailable() bool {
	return mm.pythonPath != "" || mm.mlxLMPath != ""
}

// formatMessagesAsPrompt converts messages to a prompt string
func (mm *MLXModel) formatMessagesAsPrompt(messages []map[string]interface{}) (string, error) {
	var promptParts []string

	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		switch role {
		case "system":
			promptParts = append(promptParts, fmt.Sprintf("System: %s", content))
		case "user":
			promptParts = append(promptParts, fmt.Sprintf("User: %s", content))
		case "assistant":
			promptParts = append(promptParts, fmt.Sprintf("Assistant: %s", content))
		default:
			promptParts = append(promptParts, fmt.Sprintf("%s: %s", role, content))
		}
	}

	// Add final assistant prompt
	promptParts = append(promptParts, "Assistant:")

	return strings.Join(promptParts, "\n"), nil
}

// generateWithMLX generates text using MLX-LM
func (mm *MLXModel) generateWithMLX(prompt string, maxTokens int, temperature float64, options *GenerateOptions) (string, map[string]int, error) {
	// Create a temporary Python script for generation
	scriptContent := mm.createMLXScript(prompt, maxTokens, temperature, false)

	// Write script to temporary file
	tmpDir := os.TempDir()
	scriptPath := filepath.Join(tmpDir, fmt.Sprintf("mlx_generate_%d.py", time.Now().UnixNano()))

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return "", nil, fmt.Errorf("failed to write MLX script: %w", err)
	}
	defer os.Remove(scriptPath)

	// Execute the script
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, mm.pythonPath, scriptPath)
	output, err := cmd.Output()
	if err != nil {
		return "", nil, fmt.Errorf("MLX execution failed: %w", err)
	}

	// Parse the output (assuming JSON format)
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		// If JSON parsing fails, return raw output
		return strings.TrimSpace(string(output)), map[string]int{"input": 100, "output": 50}, nil
	}

	text, _ := result["text"].(string)
	inputTokens, _ := result["input_tokens"].(float64)
	outputTokens, _ := result["output_tokens"].(float64)

	tokenCounts := map[string]int{
		"input":  int(inputTokens),
		"output": int(outputTokens),
	}

	return text, tokenCounts, nil
}

// generateStreamWithMLX generates text using MLX-LM with streaming
func (mm *MLXModel) generateStreamWithMLX(prompt string, maxTokens int, temperature float64, streamChan chan<- *ChatMessageStreamDelta) error {
	// Create a temporary Python script for streaming generation
	scriptContent := mm.createMLXScript(prompt, maxTokens, temperature, true)

	// Write script to temporary file
	tmpDir := os.TempDir()
	scriptPath := filepath.Join(tmpDir, fmt.Sprintf("mlx_stream_%d.py", time.Now().UnixNano()))

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return fmt.Errorf("failed to write MLX script: %w", err)
	}
	defer os.Remove(scriptPath)

	// Execute the script
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, mm.pythonPath, scriptPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MLX process: %w", err)
	}

	// Read streaming output
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			streamChan <- &ChatMessageStreamDelta{
				Content: &line,
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("MLX process failed: %w", err)
	}

	return nil
}

// createMLXScript creates a Python script for MLX-LM generation
func (mm *MLXModel) createMLXScript(prompt string, maxTokens int, temperature float64, streaming bool) string {
	if streaming {
		return fmt.Sprintf(`
import mlx_lm
import json
import sys

try:
    model, tokenizer = mlx_lm.load("%s", tokenizer_config={"trust_remote_code": %t})
    
    prompt = %s
    
    for response in mlx_lm.stream_generate(
        model, 
        tokenizer, 
        prompt=prompt,
        max_tokens=%d,
        temperature=%f
    ):
        print(response.text, flush=True)
        
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
    sys.exit(1)
`, mm.ModelPath, mm.TrustRemoteCode, jsonEscape(prompt), maxTokens, temperature)
	}

	return fmt.Sprintf(`
import mlx_lm
import json
import sys

try:
    model, tokenizer = mlx_lm.load("%s", tokenizer_config={"trust_remote_code": %t})
    
    prompt = %s
    
    # Simple token counting (approximate)
    input_tokens = len(tokenizer.encode(prompt))
    
    response = mlx_lm.generate(
        model, 
        tokenizer, 
        prompt=prompt,
        max_tokens=%d,
        temperature=%f
    )
    
    output_tokens = len(tokenizer.encode(response)) - input_tokens
    
    result = {
        "text": response,
        "input_tokens": input_tokens,
        "output_tokens": max(0, output_tokens)
    }
    
    print(json.dumps(result))
    
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
    sys.exit(1)
`, mm.ModelPath, mm.TrustRemoteCode, jsonEscape(prompt), maxTokens, temperature)
}

// jsonEscape properly escapes a string for JSON
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// SupportsModel checks if a model is supported by MLX
func SupportsMLXModel(modelID string) bool {
	// MLX supports most Hugging Face transformers models on Apple Silicon
	// Common model families that work well with MLX
	supportedPrefixes := []string{
		"mlx-community/",
		"microsoft/DialoGPT",
		"microsoft/CodeGPT",
		"codeparrot/",
		"Salesforce/codegen",
		"bigcode/",
		"Qwen/",
		"microsoft/",
		"meta-llama/",
		"mistralai/",
		"01-ai/",
	}

	for _, prefix := range supportedPrefixes {
		if strings.HasPrefix(modelID, prefix) {
			return true
		}
	}

	// Also support local paths
	if _, err := os.Stat(modelID); err == nil {
		return true
	}

	return false
}

// GetMLXModelDefaults returns default configuration for MLX models
func GetMLXModelDefaults() map[string]interface{} {
	return map[string]interface{}{
		"max_tokens":        2048,
		"temperature":       0.7,
		"trust_remote_code": false,
		"torch_dtype":       "auto",
		"device_map":        "auto",
	}
}

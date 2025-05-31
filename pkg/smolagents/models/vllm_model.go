// Package models - VLLMModel implementation
package models

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rizome-dev/smolagentsgo/pkg/smolagents/monitoring"
)

// VLLMModel represents a model using vLLM for fast inference
type VLLMModel struct {
	*BaseModel
	ModelPath            string                 `json:"model_path"`
	TensorParallelSize   int                    `json:"tensor_parallel_size"`
	GPUMemoryUtilization float64                `json:"gpu_memory_utilization"`
	MaxModelLen          int                    `json:"max_model_len"`
	Quantization         string                 `json:"quantization"`
	DType                string                 `json:"dtype"`
	Seed                 int                    `json:"seed"`
	TrustRemoteCode      bool                   `json:"trust_remote_code"`
	RevisionID           string                 `json:"revision_id"`
	ModelKwargs          map[string]interface{} `json:"model_kwargs"`
	SamplingParams       map[string]interface{} `json:"sampling_params"`
	pythonPath           string                 // Path to Python executable with vLLM
	tempDir              string                 // Temporary directory for scripts
}

// NewVLLMModel creates a new vLLM model
func NewVLLMModel(modelID string, options map[string]interface{}) *VLLMModel {
	base := NewBaseModel(modelID, options)

	model := &VLLMModel{
		BaseModel:            base,
		ModelPath:            modelID, // Default to model ID as path
		TensorParallelSize:   1,
		GPUMemoryUtilization: 0.9,
		MaxModelLen:          0, // Auto-detect
		Quantization:         "",
		DType:                "auto",
		Seed:                 0,
		TrustRemoteCode:      false,
		ModelKwargs:          make(map[string]interface{}),
		SamplingParams:       make(map[string]interface{}),
	}

	if options != nil {
		if modelPath, ok := options["model_path"].(string); ok {
			model.ModelPath = modelPath
		}
		if tensorParallelSize, ok := options["tensor_parallel_size"].(int); ok {
			model.TensorParallelSize = tensorParallelSize
		}
		if gpuMemoryUtilization, ok := options["gpu_memory_utilization"].(float64); ok {
			model.GPUMemoryUtilization = gpuMemoryUtilization
		}
		if maxModelLen, ok := options["max_model_len"].(int); ok {
			model.MaxModelLen = maxModelLen
		}
		if quantization, ok := options["quantization"].(string); ok {
			model.Quantization = quantization
		}
		if dtype, ok := options["dtype"].(string); ok {
			model.DType = dtype
		}
		if seed, ok := options["seed"].(int); ok {
			model.Seed = seed
		}
		if trustRemoteCode, ok := options["trust_remote_code"].(bool); ok {
			model.TrustRemoteCode = trustRemoteCode
		}
		if revisionID, ok := options["revision_id"].(string); ok {
			model.RevisionID = revisionID
		}
		if modelKwargs, ok := options["model_kwargs"].(map[string]interface{}); ok {
			model.ModelKwargs = modelKwargs
		}
		if samplingParams, ok := options["sampling_params"].(map[string]interface{}); ok {
			model.SamplingParams = samplingParams
		}
	}

	// Initialize vLLM paths
	model.initializeVLLMPaths()

	// Create temporary directory
	tempDir, _ := os.MkdirTemp("", "vllm-model-*")
	model.tempDir = tempDir

	return model
}

// initializeVLLMPaths sets up paths for vLLM and Python
func (vm *VLLMModel) initializeVLLMPaths() {
	// Try to find Python with vLLM installed
	pythonPaths := []string{
		"/opt/conda/bin/python",
		"/usr/local/bin/python3",
		"/usr/bin/python3",
		"python3", // In PATH
		"python",  // Fallback
	}

	for _, path := range pythonPaths {
		if _, err := exec.LookPath(path); err == nil {
			// Test if vLLM is available
			cmd := exec.Command(path, "-c", "import vllm; print('OK')")
			if output, err := cmd.Output(); err == nil && strings.TrimSpace(string(output)) == "OK" {
				vm.pythonPath = path
				break
			}
		}
	}
}

// Generate implements Model interface
func (vm *VLLMModel) Generate(messages []interface{}, options *GenerateOptions) (*ChatMessage, error) {
	// Check if vLLM is available
	if !vm.isVLLMAvailable() {
		return nil, fmt.Errorf("vLLM is not available. Please install it with: pip install vllm")
	}

	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, true) // Flatten for vLLM
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
	}

	// Prepare the prompt
	prompt, err := vm.formatMessagesAsPrompt(cleanMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Prepare sampling parameters
	samplingParams := vm.prepareSamplingParams(options)

	// Handle tools if provided
	var tools []map[string]interface{}
	if options != nil && len(options.ToolsToCallFrom) > 0 {
		tools = make([]map[string]interface{}, len(options.ToolsToCallFrom))
		for i, tool := range options.ToolsToCallFrom {
			tools[i] = GetToolJSONSchema(tool)
		}
	}

	// Generate response using vLLM
	response, tokenCounts, err := vm.generateWithVLLM(prompt, samplingParams, tools, options)
	if err != nil {
		return nil, fmt.Errorf("vLLM generation failed: %w", err)
	}

	// Create ChatMessage
	message := &ChatMessage{
		Role:       "assistant",
		Content:    &response,
		TokenUsage: monitoring.NewTokenUsage(tokenCounts["input"], tokenCounts["output"]),
		Raw: map[string]interface{}{
			"prompt":          prompt,
			"model_id":        vm.ModelID,
			"sampling_params": samplingParams,
		},
	}

	return message, nil
}

// GenerateStream implements Model interface
func (vm *VLLMModel) GenerateStream(messages []interface{}, options *GenerateOptions) (<-chan *ChatMessageStreamDelta, error) {
	// Check if vLLM is available
	if !vm.isVLLMAvailable() {
		return nil, fmt.Errorf("vLLM is not available. Please install it with: pip install vllm")
	}

	// Convert messages to the required format
	cleanMessages, err := GetCleanMessageList(messages, ToolRoleConversions, false, true) // Flatten for vLLM
	if err != nil {
		return nil, fmt.Errorf("failed to clean messages: %w", err)
	}

	// Prepare the prompt
	prompt, err := vm.formatMessagesAsPrompt(cleanMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to format messages: %w", err)
	}

	// Create the stream channel
	streamChan := make(chan *ChatMessageStreamDelta, 100)

	// Start streaming in a goroutine
	go func() {
		defer close(streamChan)

		samplingParams := vm.prepareSamplingParams(options)

		err := vm.generateStreamWithVLLM(prompt, samplingParams, streamChan)
		if err != nil {
			// In a real implementation, you'd send an error delta
			return
		}
	}()

	return streamChan, nil
}

// SupportsStreaming implements Model interface
func (vm *VLLMModel) SupportsStreaming() bool {
	return vm.isVLLMAvailable()
}

// ToDict implements Model interface
func (vm *VLLMModel) ToDict() map[string]interface{} {
	result := vm.BaseModel.ToDict()
	result["model_path"] = vm.ModelPath
	result["tensor_parallel_size"] = vm.TensorParallelSize
	result["gpu_memory_utilization"] = vm.GPUMemoryUtilization
	result["max_model_len"] = vm.MaxModelLen
	result["quantization"] = vm.Quantization
	result["dtype"] = vm.DType
	result["seed"] = vm.Seed
	result["trust_remote_code"] = vm.TrustRemoteCode
	result["revision_id"] = vm.RevisionID
	result["model_kwargs"] = vm.ModelKwargs
	result["sampling_params"] = vm.SamplingParams
	return result
}

// Close implements Model interface
func (vm *VLLMModel) Close() error {
	if vm.tempDir != "" {
		return os.RemoveAll(vm.tempDir)
	}
	return nil
}

// isVLLMAvailable checks if vLLM is available on the system
func (vm *VLLMModel) isVLLMAvailable() bool {
	return vm.pythonPath != ""
}

// formatMessagesAsPrompt converts messages to a prompt string for vLLM
func (vm *VLLMModel) formatMessagesAsPrompt(messages []map[string]interface{}) (string, error) {
	var promptParts []string

	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		switch role {
		case "system":
			promptParts = append(promptParts, fmt.Sprintf("<|system|>\n%s<|end|>", content))
		case "user":
			promptParts = append(promptParts, fmt.Sprintf("<|user|>\n%s<|end|>", content))
		case "assistant":
			promptParts = append(promptParts, fmt.Sprintf("<|assistant|>\n%s<|end|>", content))
		default:
			promptParts = append(promptParts, fmt.Sprintf("<%s>\n%s<|end|>", role, content))
		}
	}

	// Add final assistant prompt
	promptParts = append(promptParts, "<|assistant|>")

	return strings.Join(promptParts, "\n"), nil
}

// prepareSamplingParams prepares sampling parameters for vLLM
func (vm *VLLMModel) prepareSamplingParams(options *GenerateOptions) map[string]interface{} {
	params := make(map[string]interface{})

	// Copy base sampling params
	for k, v := range vm.SamplingParams {
		params[k] = v
	}

	// Set defaults
	if _, exists := params["temperature"]; !exists {
		params["temperature"] = 0.7
	}
	if _, exists := params["max_tokens"]; !exists {
		params["max_tokens"] = 2048
	}

	// Override with options
	if options != nil {
		if options.Temperature != nil {
			params["temperature"] = *options.Temperature
		}
		if options.MaxTokens != nil {
			params["max_tokens"] = *options.MaxTokens
		}
		if options.TopP != nil {
			params["top_p"] = *options.TopP
		}
		if len(options.StopSequences) > 0 {
			params["stop"] = options.StopSequences
		}
	}

	return params
}

// generateWithVLLM generates text using vLLM
func (vm *VLLMModel) generateWithVLLM(prompt string, samplingParams map[string]interface{}, tools []map[string]interface{}, options *GenerateOptions) (string, map[string]int, error) {
	// Create a temporary Python script for generation
	scriptContent := vm.createVLLMScript(prompt, samplingParams, tools, false)

	// Write script to temporary file
	scriptPath := filepath.Join(vm.tempDir, fmt.Sprintf("vllm_generate_%d.py", time.Now().UnixNano()))

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return "", nil, fmt.Errorf("failed to write vLLM script: %w", err)
	}
	defer os.Remove(scriptPath)

	// Execute the script
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, vm.pythonPath, scriptPath)
	output, err := cmd.Output()
	if err != nil {
		return "", nil, fmt.Errorf("vLLM execution failed: %w", err)
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

// generateStreamWithVLLM generates text using vLLM with streaming
func (vm *VLLMModel) generateStreamWithVLLM(prompt string, samplingParams map[string]interface{}, streamChan chan<- *ChatMessageStreamDelta) error {
	// Create a temporary Python script for streaming generation
	scriptContent := vm.createVLLMScript(prompt, samplingParams, nil, true)

	// Write script to temporary file
	scriptPath := filepath.Join(vm.tempDir, fmt.Sprintf("vllm_stream_%d.py", time.Now().UnixNano()))

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return fmt.Errorf("failed to write vLLM script: %w", err)
	}
	defer os.Remove(scriptPath)

	// Execute the script
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, vm.pythonPath, scriptPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start vLLM process: %w", err)
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
		return fmt.Errorf("vLLM process failed: %w", err)
	}

	return nil
}

// createVLLMScript creates a Python script for vLLM generation
func (vm *VLLMModel) createVLLMScript(prompt string, samplingParams map[string]interface{}, tools []map[string]interface{}, streaming bool) string {
	// Convert sampling params to JSON
	samplingParamsJSON, _ := json.Marshal(samplingParams)

	baseScript := fmt.Sprintf(`
import vllm
import json
import sys
from vllm import LLM, SamplingParams
from vllm.transformers_utils.tokenizer import get_tokenizer

try:
    # Initialize the model
    llm = LLM(
        model="%s",
        tensor_parallel_size=%d,
        gpu_memory_utilization=%f,
        trust_remote_code=%t,
        dtype="%s",
        seed=%d
    )
    
    tokenizer = get_tokenizer("%s")
    
    prompt = %s
    sampling_params_dict = %s
    
    # Create SamplingParams object
    sampling_params = SamplingParams(**sampling_params_dict)
    
    `, vm.ModelPath, vm.TensorParallelSize, vm.GPUMemoryUtilization,
		vm.TrustRemoteCode, vm.DType, vm.Seed, vm.ModelPath,
		jsonEscape(prompt), string(samplingParamsJSON))

	if streaming {
		return baseScript + `
    # Generate with streaming
    for output in llm.generate([prompt], sampling_params=sampling_params):
        for completion_output in output.outputs:
            print(completion_output.text, flush=True)
            
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
    sys.exit(1)
`
	}

	return baseScript + `
    # Generate response
    outputs = llm.generate([prompt], sampling_params=sampling_params)
    
    # Get the generated text
    generated_text = outputs[0].outputs[0].text
    
    # Calculate token counts
    input_tokens = len(outputs[0].prompt_token_ids)
    output_tokens = len(outputs[0].outputs[0].token_ids)
    
    result = {
        "text": generated_text,
        "input_tokens": input_tokens,
        "output_tokens": output_tokens
    }
    
    print(json.dumps(result))
    
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
    sys.exit(1)
`
}

// SupportsVLLMModel checks if a model is supported by vLLM
func SupportsVLLMModel(modelID string) bool {
	// vLLM supports most popular transformer models
	supportedPrefixes := []string{
		"meta-llama/",
		"microsoft/",
		"mistralai/",
		"codellama/",
		"WizardLM/",
		"lmsys/",
		"mosaicml/",
		"tiiuae/",
		"bigcode/",
		"Salesforce/",
		"Qwen/",
		"01-ai/",
		"deepseek-ai/",
		"NousResearch/",
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

// GetVLLMModelDefaults returns default configuration for vLLM models
func GetVLLMModelDefaults() map[string]interface{} {
	return map[string]interface{}{
		"tensor_parallel_size":   1,
		"gpu_memory_utilization": 0.9,
		"max_model_len":          0, // Auto-detect
		"dtype":                  "auto",
		"trust_remote_code":      false,
		"seed":                   0,
		"sampling_params": map[string]interface{}{
			"temperature": 0.7,
			"max_tokens":  2048,
			"top_p":       0.95,
		},
	}
}

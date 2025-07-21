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

// Package executors - Remote execution framework for distributed agent execution
package executors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// RemoteExecutionRequest represents a request to execute code remotely
type RemoteExecutionRequest struct {
	ID       string            `json:"id"`
	Language string            `json:"language"`
	Code     string            `json:"code"`
	Timeout  int               `json:"timeout"`
	Env      map[string]string `json:"env,omitempty"`
	Files    map[string]string `json:"files,omitempty"`
}

// RemoteExecutionResponse represents the response from remote execution
type RemoteExecutionResponse struct {
	ID       string `json:"id"`
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code"`
	Duration int64  `json:"duration_ms"`
}

// RemoteExecutor executes code on remote execution servers
type RemoteExecutor struct {
	servers      []string               // List of remote execution servers
	client       *http.Client           // HTTP client for requests
	roundRobin   int                    // Round-robin counter for load balancing
	mu           sync.Mutex             // Mutex for thread safety
	maxRetries   int                    // Maximum number of retries
	timeout      time.Duration          // Request timeout
	capabilities map[string]interface{} // Server capabilities
}

// NewRemoteExecutor creates a new remote executor
func NewRemoteExecutor(servers []string, options map[string]interface{}) *RemoteExecutor {
	executor := &RemoteExecutor{
		servers:      servers,
		roundRobin:   0,
		maxRetries:   3,
		timeout:      30 * time.Second,
		capabilities: make(map[string]interface{}),
		client: &http.Client{
			Timeout: 35 * time.Second,
		},
	}

	if options != nil {
		if timeout, ok := options["timeout"].(time.Duration); ok {
			executor.timeout = timeout
			executor.client.Timeout = timeout + 5*time.Second
		}
		if maxRetries, ok := options["max_retries"].(int); ok {
			executor.maxRetries = maxRetries
		}
		if capabilities, ok := options["capabilities"].(map[string]interface{}); ok {
			executor.capabilities = capabilities
		}
	}

	return executor
}

// Execute executes code on a remote server
func (re *RemoteExecutor) Execute(language, code string, options map[string]interface{}) (*ExecutionResult, error) {
	if len(re.servers) == 0 {
		return nil, fmt.Errorf("no remote execution servers configured")
	}

	// Prepare execution request
	request := &RemoteExecutionRequest{
		ID:       fmt.Sprintf("exec_%d", time.Now().UnixNano()),
		Language: language,
		Code:     code,
		Timeout:  int(re.timeout.Seconds()),
		Env:      make(map[string]string),
		Files:    make(map[string]string),
	}

	// Add environment variables and files from options
	if options != nil {
		if env, ok := options["env"].(map[string]string); ok {
			request.Env = env
		}
		if files, ok := options["files"].(map[string]string); ok {
			request.Files = files
		}
		if timeout, ok := options["timeout"].(int); ok {
			request.Timeout = timeout
		}
	}

	// Try executing on available servers with retries
	var lastError error
	for attempt := 0; attempt < re.maxRetries; attempt++ {
		server := re.getNextServer()

		response, err := re.executeOnServer(server, request)
		if err != nil {
			lastError = err
			continue
		}

		// Convert response to ExecutionResult
		result := &ExecutionResult{
			Output:   response.Output,
			ExitCode: response.ExitCode,
			Duration: time.Duration(response.Duration) * time.Millisecond,
			Stdout:   response.Output,
		}

		if !response.Success && response.Error != "" {
			result.Stderr = response.Error
		}

		return result, nil
	}

	return nil, fmt.Errorf("failed to execute on any server after %d attempts, last error: %w", re.maxRetries, lastError)
}

// executeOnServer executes a request on a specific server
func (re *RemoteExecutor) executeOnServer(server string, request *RemoteExecutionRequest) (*RemoteExecutionResponse, error) {
	// Serialize request
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/execute", server)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "smolagents-remote-executor/1.0")

	// Add timeout context
	ctx, cancel := context.WithTimeout(context.Background(), re.timeout)
	defer cancel()
	httpReq = httpReq.WithContext(ctx)

	// Make the request
	resp, err := re.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response RemoteExecutionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// getNextServer returns the next server in round-robin fashion
func (re *RemoteExecutor) getNextServer() string {
	re.mu.Lock()
	defer re.mu.Unlock()

	server := re.servers[re.roundRobin%len(re.servers)]
	re.roundRobin++
	return server
}

// GetCapabilities returns the capabilities of the remote executor
func (re *RemoteExecutor) GetCapabilities() map[string]interface{} {
	return map[string]interface{}{
		"type":           "remote",
		"languages":      []string{"go", "javascript", "rust", "cpp", "java"},
		"distributed":    true,
		"load_balancing": true,
		"servers":        len(re.servers),
		"timeout":        re.timeout.Seconds(),
		"max_retries":    re.maxRetries,
	}
}

// HealthCheck checks the health of all remote servers
func (re *RemoteExecutor) HealthCheck() map[string]bool {
	results := make(map[string]bool)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, server := range re.servers {
		wg.Add(1)
		go func(srv string) {
			defer wg.Done()

			healthy := re.checkServerHealth(srv)

			mu.Lock()
			results[srv] = healthy
			mu.Unlock()
		}(server)
	}

	wg.Wait()
	return results
}

// checkServerHealth checks if a specific server is healthy
func (re *RemoteExecutor) checkServerHealth(server string) bool {
	url := fmt.Sprintf("%s/health", server)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := re.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetHealthyServers returns a list of healthy servers
func (re *RemoteExecutor) GetHealthyServers() []string {
	healthCheck := re.HealthCheck()
	var healthy []string

	for server, isHealthy := range healthCheck {
		if isHealthy {
			healthy = append(healthy, server)
		}
	}

	return healthy
}

// RemoteExecutorPool manages multiple remote executors for different languages
type RemoteExecutorPool struct {
	executors     map[string]*RemoteExecutor // Language -> RemoteExecutor
	defaultConfig map[string]interface{}     // Default configuration
}

// NewRemoteExecutorPool creates a new remote executor pool
func NewRemoteExecutorPool(config map[string]interface{}) *RemoteExecutorPool {
	pool := &RemoteExecutorPool{
		executors:     make(map[string]*RemoteExecutor),
		defaultConfig: config,
	}

	// Initialize default executors if servers are provided
	if servers, ok := config["default_servers"].([]string); ok {
		for _, lang := range []string{"go", "javascript", "rust"} {
			pool.executors[lang] = NewRemoteExecutor(servers, config)
		}
	}

	return pool
}

// AddExecutor adds a remote executor for a specific language
func (rep *RemoteExecutorPool) AddExecutor(language string, servers []string, options map[string]interface{}) {
	if options == nil {
		options = rep.defaultConfig
	}
	rep.executors[language] = NewRemoteExecutor(servers, options)
}

// Execute executes code using the appropriate remote executor
func (rep *RemoteExecutorPool) Execute(language, code string, options map[string]interface{}) (*ExecutionResult, error) {
	executor, exists := rep.executors[language]
	if !exists {
		return nil, fmt.Errorf("no remote executor configured for language: %s", language)
	}

	return executor.Execute(language, code, options)
}

// GetAvailableLanguages returns languages with configured remote executors
func (rep *RemoteExecutorPool) GetAvailableLanguages() []string {
	var languages []string
	for lang := range rep.executors {
		languages = append(languages, lang)
	}
	return languages
}

// GetPoolStatus returns the status of all executors in the pool
func (rep *RemoteExecutorPool) GetPoolStatus() map[string]map[string]interface{} {
	status := make(map[string]map[string]interface{})

	for lang, executor := range rep.executors {
		healthCheck := executor.HealthCheck()
		healthyCount := 0
		for _, healthy := range healthCheck {
			if healthy {
				healthyCount++
			}
		}

		status[lang] = map[string]interface{}{
			"total_servers":   len(executor.servers),
			"healthy_servers": healthyCount,
			"health_check":    healthCheck,
			"capabilities":    executor.GetCapabilities(),
		}
	}

	return status
}

// ExecutorInterface defines the interface for code executors
type ExecutorInterface interface {
	Execute(language, code string, options map[string]interface{}) (*ExecutionResult, error)
}

// GoExecutorWrapper wraps GoExecutor to match the ExecutorInterface
type GoExecutorWrapper struct {
	executor *GoExecutor
}

// Execute implements ExecutorInterface for GoExecutor
func (gew *GoExecutorWrapper) Execute(language, code string, options map[string]interface{}) (*ExecutionResult, error) {
	// For now, just execute the code with default authorized imports
	result, err := gew.executor.Execute(code, DefaultAuthorizedPackages())
	if err != nil {
		return &ExecutionResult{
			Output:   "",
			Stderr:   err.Error(),
			ExitCode: 1,
			Duration: 0,
		}, err
	}

	// Convert the result to ExecutionResult format
	return &ExecutionResult{
		Output:    result,
		Variables: gew.executor.GetState(),
		Stdout:    fmt.Sprintf("%v", result),
		ExitCode:  0,
		Duration:  0, // We don't track duration in the original Execute method
	}, nil
}

// MockRemoteExecutionServer provides a simple mock server for testing
type MockRemoteExecutionServer struct {
	port      int
	executors map[string]ExecutorInterface
	server    *http.Server
}

// NewMockRemoteExecutionServer creates a new mock server
func NewMockRemoteExecutionServer(port int) *MockRemoteExecutionServer {
	mock := &MockRemoteExecutionServer{
		port:      port,
		executors: make(map[string]ExecutorInterface),
	}

	// Add default executors
	goExecutor, _ := NewGoExecutor() // Use Go executor as fallback

	// Create wrapper for Go executor to match interface
	goWrapper := &GoExecutorWrapper{executor: goExecutor}
	mock.executors["go"] = goWrapper

	return mock
}

// Start starts the mock server
func (mrs *MockRemoteExecutionServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/execute", mrs.handleExecute)
	mux.HandleFunc("/health", mrs.handleHealth)
	mux.HandleFunc("/capabilities", mrs.handleCapabilities)

	mrs.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", mrs.port),
		Handler: mux,
	}

	return mrs.server.ListenAndServe()
}

// Stop stops the mock server
func (mrs *MockRemoteExecutionServer) Stop() error {
	if mrs.server != nil {
		return mrs.server.Shutdown(context.Background())
	}
	return nil
}

// handleExecute handles execution requests
func (mrs *MockRemoteExecutionServer) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var request RemoteExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get executor for language
	executor, exists := mrs.executors[request.Language]
	if !exists {
		response := RemoteExecutionResponse{
			ID:      request.ID,
			Success: false,
			Error:   fmt.Sprintf("Unsupported language: %s", request.Language),
		}
		mrs.sendJSONResponse(w, response)
		return
	}

	// Execute code
	start := time.Now()
	result, err := executor.Execute(request.Language, request.Code, map[string]interface{}{
		"timeout": time.Duration(request.Timeout) * time.Second,
	})
	duration := time.Since(start)

	// Prepare response
	response := RemoteExecutionResponse{
		ID:       request.ID,
		Duration: duration.Milliseconds(),
	}

	if err != nil {
		response.Success = false
		response.Error = err.Error()
		response.ExitCode = 1
	} else {
		response.Success = result.ExitCode == 0
		if outputStr, ok := result.Output.(string); ok {
			response.Output = outputStr
		} else {
			response.Output = fmt.Sprintf("%v", result.Output)
		}
		response.ExitCode = result.ExitCode
		if result.Stderr != "" {
			response.Error = result.Stderr
		}
	}

	mrs.sendJSONResponse(w, response)
}

// handleHealth handles health check requests
func (mrs *MockRemoteExecutionServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

// handleCapabilities handles capabilities requests
func (mrs *MockRemoteExecutionServer) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	capabilities := map[string]interface{}{
		"languages": []string{"go", "javascript"},
		"features":  []string{"timeout", "environment", "files"},
		"version":   "1.0.0",
	}

	mrs.sendJSONResponse(w, capabilities)
}

// sendJSONResponse sends a JSON response
func (mrs *MockRemoteExecutionServer) sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

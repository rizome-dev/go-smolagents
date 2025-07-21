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

// Package tools - Model Context Protocol (MCP) integration
package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// MCPClient represents a client for communicating with MCP servers
type MCPClient struct {
	ServerPath   string            `json:"server_path"`
	ServerArgs   []string          `json:"server_args"`
	Environment  map[string]string `json:"environment"`
	Timeout      time.Duration     `json:"timeout"`
	MaxRetries   int               `json:"max_retries"`
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	requestID    int
	mu           sync.Mutex
	initialized  bool
	capabilities *MCPCapabilities
	tools        map[string]*MCPToolDefinition
}

// MCPCapabilities represents the capabilities of an MCP server
type MCPCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Logging      map[string]interface{} `json:"logging,omitempty"`
	Prompts      map[string]interface{} `json:"prompts,omitempty"`
	Resources    map[string]interface{} `json:"resources,omitempty"`
	Tools        map[string]interface{} `json:"tools,omitempty"`
}

// MCPMessage represents a JSON-RPC 2.0 message for MCP
type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an error in MCP communication
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPToolDefinition represents a tool definition from an MCP server
type MCPToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPTool represents a tool that communicates via MCP
type MCPTool struct {
	*BaseTool
	client     *MCPClient
	definition *MCPToolDefinition
}

// NewMCPClient creates a new MCP client
func NewMCPClient(serverPath string, serverArgs []string, options map[string]interface{}) *MCPClient {
	client := &MCPClient{
		ServerPath:  serverPath,
		ServerArgs:  serverArgs,
		Environment: make(map[string]string),
		Timeout:     30 * time.Second,
		MaxRetries:  3,
		tools:       make(map[string]*MCPToolDefinition),
	}

	if options != nil {
		if env, ok := options["environment"].(map[string]string); ok {
			client.Environment = env
		}
		if timeout, ok := options["timeout"].(time.Duration); ok {
			client.Timeout = timeout
		}
		if maxRetries, ok := options["max_retries"].(int); ok {
			client.MaxRetries = maxRetries
		}
	}

	return client
}

// Connect establishes a connection to the MCP server
func (mc *MCPClient) Connect() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.initialized {
		return nil
	}

	// Create command
	mc.cmd = exec.Command(mc.ServerPath, mc.ServerArgs...)

	// Set environment
	mc.cmd.Env = os.Environ()
	for key, value := range mc.Environment {
		mc.cmd.Env = append(mc.cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Set up pipes
	var err error
	mc.stdin, err = mc.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	mc.stdout, err = mc.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	mc.stderr, err = mc.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the server
	if err := mc.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Initialize the connection
	if err := mc.initialize(); err != nil {
		mc.Close()
		return fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	mc.initialized = true
	return nil
}

// initialize performs the MCP initialization handshake
func (mc *MCPClient) initialize() error {
	// Send initialize request
	initRequest := MCPMessage{
		JSONRPC: "2.0",
		ID:      mc.nextRequestID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"roots": map[string]interface{}{
					"listChanged": true,
				},
				"sampling": map[string]interface{}{},
			},
			"clientInfo": map[string]interface{}{
				"name":    "smolagents-go",
				"version": "1.18.0",
			},
		},
	}

	// Send request and wait for response
	response, err := mc.sendRequest(initRequest)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Parse capabilities
	if result, ok := response.Result.(map[string]interface{}); ok {
		if caps, ok := result["capabilities"].(map[string]interface{}); ok {
			capData, _ := json.Marshal(caps)
			mc.capabilities = &MCPCapabilities{}
			json.Unmarshal(capData, mc.capabilities)
		}
	}

	// Send initialized notification
	initializedNotif := MCPMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]interface{}{},
	}

	return mc.sendNotification(initializedNotif)
}

// ListTools retrieves available tools from the MCP server
func (mc *MCPClient) ListTools() ([]*MCPToolDefinition, error) {
	if !mc.initialized {
		if err := mc.Connect(); err != nil {
			return nil, err
		}
	}

	// Send tools/list request
	request := MCPMessage{
		JSONRPC: "2.0",
		ID:      mc.nextRequestID(),
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	response, err := mc.sendRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Parse response
	var tools []*MCPToolDefinition
	if result, ok := response.Result.(map[string]interface{}); ok {
		if toolsList, ok := result["tools"].([]interface{}); ok {
			for _, toolData := range toolsList {
				if toolMap, ok := toolData.(map[string]interface{}); ok {
					tool := &MCPToolDefinition{}
					if name, ok := toolMap["name"].(string); ok {
						tool.Name = name
					}
					if desc, ok := toolMap["description"].(string); ok {
						tool.Description = desc
					}
					if schema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
						tool.InputSchema = schema
					}
					tools = append(tools, tool)
					mc.tools[tool.Name] = tool
				}
			}
		}
	}

	return tools, nil
}

// CallTool executes a tool on the MCP server
func (mc *MCPClient) CallTool(name string, arguments map[string]interface{}) (interface{}, error) {
	if !mc.initialized {
		if err := mc.Connect(); err != nil {
			return nil, err
		}
	}

	// Send tools/call request
	request := MCPMessage{
		JSONRPC: "2.0",
		ID:      mc.nextRequestID(),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
	}

	response, err := mc.sendRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool %s: %w", name, err)
	}

	// Parse response
	if result, ok := response.Result.(map[string]interface{}); ok {
		if content, ok := result["content"]; ok {
			return content, nil
		}
	}

	return response.Result, nil
}

// sendRequest sends a request and waits for a response
func (mc *MCPClient) sendRequest(request MCPMessage) (*MCPMessage, error) {
	// Serialize request
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	if _, err := mc.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	ctx, cancel := context.WithTimeout(context.Background(), mc.Timeout)
	defer cancel()

	responseChan := make(chan *MCPMessage, 1)
	errorChan := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(mc.stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var response MCPMessage
			if err := json.Unmarshal([]byte(line), &response); err != nil {
				errorChan <- fmt.Errorf("failed to parse response: %w", err)
				return
			}

			// Check if this is the response to our request
			if response.ID == request.ID {
				responseChan <- &response
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errorChan <- fmt.Errorf("error reading response: %w", err)
		}
	}()

	select {
	case response := <-responseChan:
		if response.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", response.Error.Code, response.Error.Message)
		}
		return response, nil
	case err := <-errorChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("request timeout")
	}
}

// sendNotification sends a notification (no response expected)
func (mc *MCPClient) sendNotification(notification MCPMessage) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	if _, err := mc.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// nextRequestID generates the next request ID
func (mc *MCPClient) nextRequestID() int {
	mc.requestID++
	return mc.requestID
}

// Close closes the connection to the MCP server
func (mc *MCPClient) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if !mc.initialized {
		return nil
	}

	// Close pipes
	if mc.stdin != nil {
		mc.stdin.Close()
	}
	if mc.stdout != nil {
		mc.stdout.Close()
	}
	if mc.stderr != nil {
		mc.stderr.Close()
	}

	// Terminate the process
	if mc.cmd != nil && mc.cmd.Process != nil {
		mc.cmd.Process.Kill()
		mc.cmd.Wait()
	}

	mc.initialized = false
	return nil
}

// LoadMCPTool creates a tool wrapper for an MCP tool
func LoadMCPTool(client *MCPClient, toolName string) (*MCPTool, error) {
	// Ensure tools are loaded
	tools, err := client.ListTools()
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP tools: %w", err)
	}

	// Find the tool
	var toolDef *MCPToolDefinition
	for _, tool := range tools {
		if tool.Name == toolName {
			toolDef = tool
			break
		}
	}

	if toolDef == nil {
		return nil, fmt.Errorf("tool %s not found on MCP server", toolName)
	}

	// Convert MCP input schema to ToolInput format
	inputs := make(map[string]*ToolInput)
	if toolDef.InputSchema != nil {
		if properties, ok := toolDef.InputSchema["properties"].(map[string]interface{}); ok {
			for name, propData := range properties {
				if prop, ok := propData.(map[string]interface{}); ok {
					input := &ToolInput{
						Type:        "string", // Default
						Description: "",
						Nullable:    true,
					}

					if propType, ok := prop["type"].(string); ok {
						input.Type = propType
					}
					if desc, ok := prop["description"].(string); ok {
						input.Description = desc
					}

					// Check if required
					if required, ok := toolDef.InputSchema["required"].([]interface{}); ok {
						for _, req := range required {
							if reqName, ok := req.(string); ok && reqName == name {
								input.Nullable = false
								break
							}
						}
					}

					inputs[name] = input
				}
			}
		}
	}

	// Create base tool
	baseTool := NewBaseTool(
		toolDef.Name,
		toolDef.Description,
		inputs,
		"any", // MCP tools can return various types
	)

	// Create MCP tool
	mcpTool := &MCPTool{
		BaseTool:   baseTool,
		client:     client,
		definition: toolDef,
	}

	// Set the forward function
	mcpTool.ForwardFunc = mcpTool.forward

	return mcpTool, nil
}

// forward implements the tool execution logic for MCP tools
func (mt *MCPTool) forward(args ...interface{}) (interface{}, error) {
	// Convert args to input map
	inputs := make(map[string]interface{})

	// If we have a single map argument, use it directly
	if len(args) == 1 {
		if inputMap, ok := args[0].(map[string]interface{}); ok {
			inputs = inputMap
		} else {
			// Single argument - map to first input parameter
			if len(mt.Inputs) > 0 {
				for name := range mt.Inputs {
					inputs[name] = args[0]
					break
				}
			}
		}
	} else {
		// Multiple arguments - map positionally
		i := 0
		for name := range mt.Inputs {
			if i < len(args) {
				inputs[name] = args[i]
				i++
			}
		}
	}

	// Call the MCP tool
	return mt.client.CallTool(mt.definition.Name, inputs)
}

// MCPToolRegistry manages a collection of MCP tools
type MCPToolRegistry struct {
	clients map[string]*MCPClient
	tools   map[string]*MCPTool
	mu      sync.RWMutex
}

// NewMCPToolRegistry creates a new MCP tool registry
func NewMCPToolRegistry() *MCPToolRegistry {
	return &MCPToolRegistry{
		clients: make(map[string]*MCPClient),
		tools:   make(map[string]*MCPTool),
	}
}

// AddMCPServer adds an MCP server to the registry
func (mtr *MCPToolRegistry) AddMCPServer(name, serverPath string, serverArgs []string, options map[string]interface{}) error {
	mtr.mu.Lock()
	defer mtr.mu.Unlock()

	client := NewMCPClient(serverPath, serverArgs, options)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to MCP server %s: %w", name, err)
	}

	mtr.clients[name] = client

	// Load all tools from this server
	tools, err := client.ListTools()
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to list tools from MCP server %s: %w", name, err)
	}

	for _, toolDef := range tools {
		mcpTool, err := LoadMCPTool(client, toolDef.Name)
		if err != nil {
			continue // Skip tools that fail to load
		}
		mtr.tools[toolDef.Name] = mcpTool
	}

	return nil
}

// GetTool retrieves an MCP tool by name
func (mtr *MCPToolRegistry) GetTool(name string) (*MCPTool, bool) {
	mtr.mu.RLock()
	defer mtr.mu.RUnlock()

	tool, exists := mtr.tools[name]
	return tool, exists
}

// ListTools returns all available MCP tools
func (mtr *MCPToolRegistry) ListTools() map[string]*MCPTool {
	mtr.mu.RLock()
	defer mtr.mu.RUnlock()

	result := make(map[string]*MCPTool)
	for k, v := range mtr.tools {
		result[k] = v
	}
	return result
}

// RemoveServer removes an MCP server and its tools
func (mtr *MCPToolRegistry) RemoveServer(name string) error {
	mtr.mu.Lock()
	defer mtr.mu.Unlock()

	client, exists := mtr.clients[name]
	if !exists {
		return fmt.Errorf("MCP server %s not found", name)
	}

	// Remove all tools from this server
	for toolName, tool := range mtr.tools {
		if tool.client == client {
			delete(mtr.tools, toolName)
		}
	}

	// Close and remove the client
	client.Close()
	delete(mtr.clients, name)

	return nil
}

// Close closes all MCP connections
func (mtr *MCPToolRegistry) Close() error {
	mtr.mu.Lock()
	defer mtr.mu.Unlock()

	for _, client := range mtr.clients {
		client.Close()
	}

	mtr.clients = make(map[string]*MCPClient)
	mtr.tools = make(map[string]*MCPTool)

	return nil
}

// DefaultMCPRegistry is the default MCP tool registry
var DefaultMCPRegistry = NewMCPToolRegistry()

// AddMCPServer is a convenience function to add an MCP server to the default registry
func AddMCPServer(name, serverPath string, serverArgs []string, options map[string]interface{}) error {
	return DefaultMCPRegistry.AddMCPServer(name, serverPath, serverArgs, options)
}

// GetMCPTool is a convenience function to get an MCP tool from the default registry
func GetMCPTool(name string) (*MCPTool, bool) {
	return DefaultMCPRegistry.GetTool(name)
}

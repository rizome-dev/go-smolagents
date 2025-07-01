// Package executors provides code execution capabilities for smolagents.
//
// This implements a Go code executor that can safely execute Go code snippets
// while maintaining variable state between executions and providing proper
// error handling and security features.
package executors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rizome-dev/go-smolagents/pkg/tools"
)

// GoExecutor implements code execution for Go code snippets
type GoExecutor struct {
	// State management
	variables          map[string]interface{}
	availableTools     map[string]tools.Tool
	authorizedPackages []string
	workingDir         string

	// Configuration
	timeout          time.Duration
	maxMemory        int64
	maxOutputLength  int
	enableNetworking bool
	enableFileSystem bool

	// Execution state
	executionCount int
	mu             sync.RWMutex

	// Code template for execution
	codeTemplate string
}

// NewGoExecutor creates a new Go code executor
func NewGoExecutor(options ...map[string]interface{}) (*GoExecutor, error) {
	executor := &GoExecutor{
		variables:          make(map[string]interface{}),
		availableTools:     make(map[string]tools.Tool),
		authorizedPackages: DefaultAuthorizedPackages(),
		timeout:            30 * time.Second,
		maxMemory:          100 * 1024 * 1024, // 100MB
		maxOutputLength:    10000,
		enableNetworking:   false,
		enableFileSystem:   false,
		executionCount:     0,
		codeTemplate:       defaultGoTemplate,
	}

	// Create working directory
	workDir, err := os.MkdirTemp("", "smolagents-go-executor-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}
	executor.workingDir = workDir

	// Apply options
	if len(options) > 0 {
		opts := options[0]
		if timeout, ok := opts["timeout"].(time.Duration); ok {
			executor.timeout = timeout
		}
		if maxMemory, ok := opts["max_memory"].(int64); ok {
			executor.maxMemory = maxMemory
		}
		if maxOutput, ok := opts["max_output_length"].(int); ok {
			executor.maxOutputLength = maxOutput
		}
		if networking, ok := opts["enable_networking"].(bool); ok {
			executor.enableNetworking = networking
		}
		if filesystem, ok := opts["enable_filesystem"].(bool); ok {
			executor.enableFileSystem = filesystem
		}
		if packages, ok := opts["authorized_packages"].([]string); ok {
			executor.authorizedPackages = packages
		}
	}

	return executor, nil
}

// DefaultAuthorizedPackages returns the default list of authorized Go packages
func DefaultAuthorizedPackages() []string {
	return []string{
		"fmt", "math", "math/rand", "strings", "strconv", "time",
		"encoding/json", "encoding/csv", "encoding/base64",
		"regexp", "sort", "unicode", "unicode/utf8",
		"crypto/md5", "crypto/sha1", "crypto/sha256",
		"path", "path/filepath", "net/url",
		"compress/gzip", "archive/zip",
		"bytes", "bufio", "io", "container/list", "container/heap",
	}
}

// Execute executes Go code and returns the result
func (ge *GoExecutor) Execute(code string, authorizedImports []string) (interface{}, error) {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	ge.executionCount++

	// Validate the code
	if err := ge.validateCode(code, authorizedImports); err != nil {
		return nil, fmt.Errorf("code validation failed: %w", err)
	}

	// Prepare the complete Go program
	program, err := ge.buildProgram(code, authorizedImports)
	if err != nil {
		return nil, fmt.Errorf("failed to build program: %w", err)
	}

	// Execute the program
	result, err := ge.executeProgram(program)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	// Parse and store any variable updates
	if err := ge.updateVariables(result); err != nil {
		// Don't fail on variable update errors, just log them
		fmt.Printf("Warning: failed to update variables: %v\n", err)
	}

	return result.Output, nil
}

// ExecutionResult represents the result of code execution
type ExecutionResult struct {
	Output    interface{}            `json:"output"`
	Variables map[string]interface{} `json:"variables"`
	Stdout    string                 `json:"stdout"`
	Stderr    string                 `json:"stderr"`
	ExitCode  int                    `json:"exit_code"`
	Duration  time.Duration          `json:"duration"`
}

// validateCode validates the Go code before execution
func (ge *GoExecutor) validateCode(code string, authorizedImports []string) error {
	// Parse the code to check syntax by wrapping it in a function
	fset := token.NewFileSet()
	wrappedCode := fmt.Sprintf(`package main
func main() {
	var result interface{}
	%s
}`, code)
	_, err := parser.ParseFile(fset, "", wrappedCode, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("syntax error: %w", err)
	}

	// Check for unauthorized imports
	if err := ge.checkImports(code, authorizedImports); err != nil {
		return fmt.Errorf("unauthorized import: %w", err)
	}

	// Check for unsafe operations
	if err := ge.checkUnsafeOperations(code); err != nil {
		return fmt.Errorf("unsafe operation detected: %w", err)
	}

	return nil
}

// checkImports validates that only authorized imports are used
func (ge *GoExecutor) checkImports(code string, authorizedImports []string) error {
	// Combine authorized packages with explicitly allowed imports
	allowed := make(map[string]bool)
	for _, pkg := range ge.authorizedPackages {
		allowed[pkg] = true
	}
	for _, pkg := range authorizedImports {
		allowed[pkg] = true
	}

	// Parse imports from code
	importRegex := regexp.MustCompile(`import\s+(?:\(\s*([^)]+)\s*\)|"([^"]+)"|([^\s]+))`)
	matches := importRegex.FindAllStringSubmatch(code, -1)

	for _, match := range matches {
		var importPath string
		if match[1] != "" {
			// Multiple imports in parentheses
			lines := strings.Split(match[1], "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					importPath = strings.Trim(line, `"`)
					if !allowed[importPath] {
						return fmt.Errorf("unauthorized import: %s", importPath)
					}
				}
			}
		} else if match[2] != "" {
			// Single import with quotes
			importPath = match[2]
		} else if match[3] != "" {
			// Single import without quotes
			importPath = match[3]
		}

		if importPath != "" && !allowed[importPath] {
			return fmt.Errorf("unauthorized import: %s", importPath)
		}
	}

	return nil
}

// checkUnsafeOperations checks for potentially unsafe operations
func (ge *GoExecutor) checkUnsafeOperations(code string) error {
	unsafePatterns := []string{
		`unsafe\.`,     // unsafe package usage
		`syscall\.`,    // direct syscalls
		`os\.Exit`,     // program termination
		`panic\s*\(`,   // panic calls
		`recover\s*\(`, // recover calls
	}

	if !ge.enableFileSystem {
		unsafePatterns = append(unsafePatterns,
			`os\.Create`, `os\.Open`, `os\.Remove`, `os\.Mkdir`,
			`ioutil\.WriteFile`, `ioutil\.ReadFile`,
			`filepath\.Walk`,
		)
	}

	if !ge.enableNetworking {
		unsafePatterns = append(unsafePatterns,
			`http\.`, `net\.`, `url\.`,
		)
	}

	for _, pattern := range unsafePatterns {
		matched, _ := regexp.MatchString(pattern, code)
		if matched {
			return fmt.Errorf("unsafe operation detected: %s", pattern)
		}
	}

	return nil
}

// buildProgram creates a complete Go program from the code snippet
func (ge *GoExecutor) buildProgram(code string, authorizedPackages []string) (string, error) {
	// Prepare imports with deduplication
	defaultImports := []string{"encoding/json", "fmt", "os"}
	importSet := make(map[string]bool)
	var imports []string

	// Add default imports first
	for _, pkg := range defaultImports {
		importSet[pkg] = true
		imports = append(imports, fmt.Sprintf(`"%s"`, pkg))
	}

	// Add authorized packages (avoiding duplicates)
	for _, pkg := range authorizedPackages {
		if !importSet[pkg] {
			importSet[pkg] = true
			imports = append(imports, fmt.Sprintf(`"%s"`, pkg))
		}
	}

	// Prepare variables as global variables
	var variableDeclarations []string
	for name, value := range ge.variables {
		varType := reflect.TypeOf(value).String()
		varValue := ge.formatValue(value)
		variableDeclarations = append(variableDeclarations,
			fmt.Sprintf("var %s %s = %s", name, varType, varValue))
	}

	// Build the complete program
	program := fmt.Sprintf(`package main

import (
	%s
)

%s

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Panic: %%v\n", r)
			os.Exit(1)
		}
	}()
	
	var result interface{}
	var err error
	_ = err // Suppress unused variable error
	
	// User code starts here
	%s
	// User code ends here
	
	// Capture variables state
	variables := map[string]interface{}{
		%s
	}
	
	// Output result
	output := map[string]interface{}{
		"result": result,
		"variables": variables,
	}
	
	jsonOutput, err := json.Marshal(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal output: %%v\n", err)
		os.Exit(1)
	}
	
	fmt.Print(string(jsonOutput))
}`,
		strings.Join(imports, "\n\t"),
		strings.Join(variableDeclarations, "\n"),
		code,
		ge.buildVariableCapture(),
	)

	return program, nil
}

// formatValue formats a Go value for code generation
func (ge *GoExecutor) formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf(`"%s"`, strings.ReplaceAll(v, `"`, `\"`))
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return "nil"
	default:
		// For complex types, use JSON marshaling
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "nil"
		}
		return fmt.Sprintf(`json.RawMessage(%q)`, string(jsonBytes))
	}
}

// buildVariableCapture creates code to capture current variable values
func (ge *GoExecutor) buildVariableCapture() string {
	var captures []string
	for name := range ge.variables {
		captures = append(captures, fmt.Sprintf(`"%s": %s`, name, name))
	}
	return strings.Join(captures, ",\n\t\t")
}

// executeProgram executes the Go program and returns the result
func (ge *GoExecutor) executeProgram(program string) (*ExecutionResult, error) {
	// Write program to file
	filename := fmt.Sprintf("program_%d.go", ge.executionCount)
	filePath := filepath.Join(ge.workingDir, filename)

	if err := os.WriteFile(filePath, []byte(program), 0644); err != nil {
		return nil, fmt.Errorf("failed to write program file: %w", err)
	}

	// Set up execution context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), ge.timeout)
	defer cancel()

	// Prepare command
	cmd := exec.CommandContext(ctx, "go", "run", filePath)
	cmd.Dir = ge.workingDir

	// Set up environment restrictions
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"GOPATH=" + ge.workingDir,
		"GOCACHE=" + filepath.Join(ge.workingDir, ".cache"),
		fmt.Sprintf("GOMAXPROCS=%d", 1), // Limit CPU usage
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	result := &ExecutionResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("execution timeout after %v", ge.timeout)
		}
		result.ExitCode = 1
		errorMsg := fmt.Sprintf("execution failed: %v", err)
		if stderr.Len() > 0 {
			errorMsg += fmt.Sprintf(", stderr: %s", stderr.String())
		}
		return result, fmt.Errorf("%s", errorMsg)
	}

	// Parse output
	if stdout.Len() > 0 {
		var output map[string]interface{}
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			// If JSON parsing fails, return raw output
			result.Output = stdout.String()
		} else {
			result.Output = output["result"]
			if vars, ok := output["variables"].(map[string]interface{}); ok {
				result.Variables = vars
			}
		}
	}

	return result, nil
}

// updateVariables updates the executor's variable state from execution result
func (ge *GoExecutor) updateVariables(result *ExecutionResult) error {
	if result.Variables == nil {
		return nil
	}

	for name, value := range result.Variables {
		ge.variables[name] = value
	}

	return nil
}

// SendVariables updates the executor's variable state
func (ge *GoExecutor) SendVariables(variables map[string]interface{}) error {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	for name, value := range variables {
		ge.variables[name] = value
	}

	return nil
}

// SendTools makes tools available to the executor
func (ge *GoExecutor) SendTools(tools map[string]tools.Tool) error {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	for name, tool := range tools {
		ge.availableTools[name] = tool
	}

	return nil
}

// GetState returns the current variable state
func (ge *GoExecutor) GetState() map[string]interface{} {
	ge.mu.RLock()
	defer ge.mu.RUnlock()

	state := make(map[string]interface{})
	for name, value := range ge.variables {
		state[name] = value
	}

	return state
}

// Reset clears all variables and state
func (ge *GoExecutor) Reset() error {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	ge.variables = make(map[string]interface{})
	ge.executionCount = 0

	// Clean up working directory
	if err := os.RemoveAll(ge.workingDir); err != nil {
		return fmt.Errorf("failed to clean working directory: %w", err)
	}

	// Create new working directory
	workDir, err := os.MkdirTemp("", "smolagents-go-executor-*")
	if err != nil {
		return fmt.Errorf("failed to create new working directory: %w", err)
	}
	ge.workingDir = workDir

	return nil
}

// SetTimeout sets the execution timeout
func (ge *GoExecutor) SetTimeout(timeout time.Duration) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	ge.timeout = timeout
}

// SetMaxMemory sets the maximum memory usage
func (ge *GoExecutor) SetMaxMemory(maxMemory int64) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	ge.maxMemory = maxMemory
}

// SetAuthorizedPackages sets the list of authorized Go packages
func (ge *GoExecutor) SetAuthorizedPackages(packages []string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	ge.authorizedPackages = packages
}

// Close cleans up the executor
func (ge *GoExecutor) Close() error {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	return os.RemoveAll(ge.workingDir)
}

// defaultGoTemplate is the default template for Go code execution
const defaultGoTemplate = `package main

import (
	{{IMPORTS}}
)

{{VARIABLES}}

func main() {
	{{CODE}}
}`
